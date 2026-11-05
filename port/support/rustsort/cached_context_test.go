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
)

type cachedWireCase struct {
	Kind      int    `json:"kind"`
	Chain     bool   `json:"chain"`
	Length    int    `json:"length"`
	Seed      uint64 `json:"seed"`
	KeyFail   int    `json:"key_fail"`
	CmpFail   int    `json:"cmp_fail"`
	DropFail  int    `json:"drop_fail"`
	FnFail    bool   `json:"fn_fail"`
	TokenFail bool   `json:"token_fail"`
	Moved     bool   `json:"moved"`
}
type cachedChild struct {
	Context    string         `json:"context"`
	Request    cachedWireCase `json:"request"`
	Outcome    string         `json:"outcome"`
	NativeCode int            `json:"native_exit_code"`
	Stdout     string         `json:"stdout"`
}

func TestCachedContextChild(t *testing.T) {
	encoded := os.Getenv("FERRUGO_CACHED_CONTEXT_CHILD")
	if encoded == "" {
		t.Skip("isolated cached-key context child")
	}
	var child cachedChild
	if err := json.Unmarshal([]byte(encoded), &child); err != nil {
		t.Fatal(err)
	}
	r := child.Request
	spec := cachedCase{kind: r.Kind, chain: r.Chain, length: r.Length, seed: r.Seed, keyFail: r.KeyFail, cmpFail: r.CmpFail, dropFail: r.DropFail, fnFail: r.FnFail, tokenFail: r.TokenFail, moved: r.Moved}
	context := Context{Unwinding: child.Context == "unwind", ProcessingPanic: child.Context == "processing"}
	if child.Context != "normal" && child.Context != "unwind" && child.Context != "processing" {
		t.Fatal("invalid cached-key context")
	}
	func() {
		fmt.Printf("scope:%t\n", context.Panicking())
		defer func() { fmt.Printf("scope_drop:%t\n", context.Panicking()) }()
		message, state, _, compares, drops := replayCachedCaseContext(spec, context, func(value string) { fmt.Println(value) })
		fmt.Printf("RESULT:%s:%d:%d:%s\n", hex.EncodeToString([]byte(message)), compares, drops, hex.EncodeToString([]byte(state)))
	}()
	if child.Context != "normal" {
		fmt.Println("outer_end:false")
	}
	os.Exit(0)
}
func TestOriginalCachedContext(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/cached/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Children struct {
			Cases, Aborts int
			Fixture       string `json:"fixture_sha256"`
			Raw           string `json:"raw_sha256"`
		}
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { hash := sha256.Sum256(b); return hex.EncodeToString(hash[:]) }
	compressed, err := os.ReadFile("testdata/cached_context.jsonl.gz")
	if err != nil || digest(compressed) != manifest.Children.Fixture || manifest.Children.Cases != 99 || manifest.Children.Aborts != 36 {
		t.Fatalf("original cached-key child fixture differs:%v", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 16<<20))
	if err != nil || digest(raw) != manifest.Children.Raw {
		t.Fatalf("original cached-key child body differs:%v", err)
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
	count, aborts := 0, 0
	for scanner.Scan() {
		var child cachedChild
		if err := json.Unmarshal(scanner.Bytes(), &child); err != nil {
			t.Fatal(err)
		}
		if (child.Outcome == "abort" && child.NativeCode != -6) || (child.Outcome == "completed" && child.NativeCode != 0) {
			t.Fatal("original cached-key process class differs")
		}
		command := exec.Command(executable, "-test.run=^TestCachedContextChild$")
		command.Env = append(os.Environ(), "FERRUGO_CACHED_CONTEXT_CHILD="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		outcome := "completed"
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 134 {
				outcome = "abort"
			} else {
				t.Fatalf("cached-key child failed unexpectedly:%v:%s", err, stderr.String())
			}
		}
		if outcome != child.Outcome || stdout.String() != child.Stdout {
			t.Fatalf("original cached-key context%d differs request%+v/context%s\nclass:%s/want%s\n got:%s\nwant:%s\nstderr:%s", count, child.Request, child.Context, outcome, child.Outcome, stdout.String(), child.Stdout, stderr.String())
		}
		count++
		if outcome == "abort" {
			aborts++
		}
	}
	if err := scanner.Err(); err != nil || count != manifest.Children.Cases || aborts != manifest.Children.Aborts {
		t.Fatalf("cached-key child coverage differs:%d/%d:%v", count, aborts, err)
	}
}
