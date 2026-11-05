// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type cachedHookCase struct {
	Context    string         `json:"context"`
	Request    cachedWireCase `json:"request"`
	Resume     bool           `json:"resume"`
	Outcome    string         `json:"outcome"`
	NativeCode int            `json:"native_exit_code"`
	Stdout     string         `json:"stdout"`
}
type cachedHookEnvelope struct {
	Case      cachedHookCase
	Locations map[string]rustfmt.SourceLocation
}
type cachedResumeControl bool

func (c cachedResumeControl) ResumeCachedPanic() bool { return bool(c) }
func TestCachedHookChild(t *testing.T) {
	encoded := os.Getenv("FERRUGO_CACHED_HOOK_CHILD")
	if encoded == "" {
		t.Skip("isolated cached hook child")
	}
	var envelope cachedHookEnvelope
	if err := json.Unmarshal([]byte(encoded), &envelope); err != nil {
		t.Fatal(err)
	}
	child := envelope.Case
	r := child.Request
	spec := cachedCase{kind: r.Kind, chain: r.Chain, length: r.Length, seed: r.Seed, keyFail: r.KeyFail, cmpFail: r.CmpFail, dropFail: r.DropFail, fnFail: r.FnFail, tokenFail: r.TokenFail, moved: r.Moved}
	locations := map[string]rustfmt.SourceLocation{}
	for key, value := range envelope.Locations {
		locations["cached.client."+key] = value
	}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	ops := &rustfmt.PanicOperations{CleanupAbort: &cleanup, Locations: locations}
	var body func(Context)
	body = func(c Context) {
		fmt.Printf("scope:%t\n", c.Panicking())
		defer func() { fmt.Printf("scope_drop:%t\n", c.Panicking()) }()
		message, state, _, compares, drops := replayCachedCaseContext(spec, c, func(event string) { fmt.Println(event) })
		fmt.Printf("RESULT:%s:%d:%d:%s\n", hex.EncodeToString([]byte(message)), compares, drops, hex.EncodeToString([]byte(state)))
	}
	ops.Hook = func(info rustfmt.SourcePanicInfo, c Context) {
		func() {
			file := ""
			var line, column uint32
			if info.Location != nil {
				file, line, column = info.Location.File, info.Location.Line, info.Location.Column
			}
			fmt.Printf("HOOK:%s:%s:%t:%t:%t:%s:%d:%d\n", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(file)), line, column)
			defer func() { fmt.Printf("hook_local:%t\n", c.Panicking()) }()
		}()
		if child.Context == "processing" {
			body(c)
		}
	}
	c := rustfmt.WithPanicOperations(Context{Runtime: &rustfmt.RuntimeMetadata{Value: cachedResumeControl(child.Resume)}}, ops)
	outerInfo := func(mode string) rustfmt.SourcePanicInfo {
		return rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: c.SourcePanicLocation("cached.client.outer." + mode), CanUnwind: true}
	}
	switch child.Context {
	case "normal":
		body(c)
	case "unwind":
		func() {
			defer func() { recover() }()
			defer func() { p := recover(); unwind := c; unwind.Unwinding = true; body(unwind); c.ResumeUnwind(p) }()
			c.PanicAt(outerInfo("unwind"))
		}()
		fmt.Println("outer_end:false")
	case "processing":
		func() { defer func() { recover() }(); c.PanicAt(outerInfo("processing")) }()
		fmt.Println("outer_end:false")
	default:
		t.Fatal("invalid cached hook context")
	}
	os.Exit(0)
}
func TestOriginalCachedHooks(t *testing.T) {
	const root = "../../../tools/port-rust-unord/cached/"
	metadata, err := os.ReadFile(root + "hooks_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases, Aborts int
		Locations     map[string]rustfmt.SourceLocation
		Fixture       string            `json:"fixture_sha256"`
		Raw           string            `json:"raw_sha256"`
		Sources       map[string]string `json:"reference_sources"`
		SDK           string            `json:"sdk_selector_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { hash := sha256.Sum256(b); return hex.EncodeToString(hash[:]) }
	for name, want := range manifest.Sources {
		b, err := os.ReadFile(root + name)
		if err != nil || digest(b) != want {
			t.Fatalf("cached hook source binding differs:%s:%v", name, err)
		}
	}
	shared, err := os.ReadFile(root + "../sort/generate.py")
	if err != nil || digest(shared) != manifest.SDK {
		t.Fatalf("cached hook SDK selector differs:%v", err)
	}
	compressed, err := os.ReadFile("testdata/cached_hooks.jsonl.gz")
	if err != nil || digest(compressed) != manifest.Fixture || manifest.Cases != 198 || manifest.Aborts != 72 {
		t.Fatalf("cached hook fixture differs:%v", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 32<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("cached hook raw fixture differs:%v", err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases, aborts := 0, 0
	for scanner.Scan() {
		var child cachedHookCase
		if err := json.Unmarshal(scanner.Bytes(), &child); err != nil {
			t.Fatal(err)
		}
		if (child.Outcome == "abort" && child.NativeCode != -6) || (child.Outcome == "completed" && child.NativeCode != 0) {
			t.Fatal("cached hook native process class differs")
		}
		envelope, _ := json.Marshal(cachedHookEnvelope{child, manifest.Locations})
		command := exec.Command(executable, "-test.run=^TestCachedHookChild$")
		command.Env = append(os.Environ(), "FERRUGO_CACHED_HOOK_CHILD="+string(envelope), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		outcome := "completed"
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 134 {
				outcome = "abort"
			} else {
				t.Fatalf("cached hook child failed:%v:%s", err, stderr.String())
			}
		}
		if outcome != child.Outcome || stdout.String() != child.Stdout {
			t.Fatalf("cached hook%d differs context%s/resume%t/request%+v\nclass%s/want%s\n got:%s\nwant:%s", cases, child.Context, child.Resume, child.Request, outcome, child.Outcome, stdout.String(), child.Stdout)
		}
		cases++
		if outcome == "abort" {
			aborts++
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases || aborts != manifest.Aborts {
		t.Fatalf("cached hook coverage differs:%d/%d:%v", cases, aborts, err)
	}
}
