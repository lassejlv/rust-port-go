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
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func replayCachedZeroContext(spec cachedCase, context Context, stream func(string)) (message, state, events string, compares, drops int) {
	var trace []string
	event := func(value string) {
		trace = append(trace, value)
		if stream != nil {
			stream(value)
		}
	}
	raise := func(c Context, label string) {
		if c.Runtime != nil {
			if control, ok := c.Runtime.Value.(interface{ ResumeCachedPanic() bool }); ok && control.ResumeCachedPanic() {
				c.ResumeUnwind(label)
			}
		}
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: c.SourcePanicLocation("cached.client." + label), CanUnwind: true})
	}
	typ := cachedSourceType(spec.kind)
	narrow, wide := tupleIndexType(typ, 4, context), tupleIndexType(typ, 8, context)
	event(fmt.Sprintf("TYPE:%d:%d:%d:%d", typ.Size, typ.Align, narrow.Size, wide.Size))
	movedToken := false
	tokenDrop := func(c Context) {
		event(fmt.Sprintf("TD:%t", c.Panicking()))
		if spec.tokenFail {
			raise(c, "token_drop")
		}
	}
	keyDrop := func(key *cachedKey, c Context) {
		index := drops
		drops++
		event(fmt.Sprintf("KD:%d:%d:%t", key.id, key.touches, c.Panicking()))
		if index == spec.dropFail {
			raise(c, "key_drop")
		}
	}
	keyOps := CachedKeyOps[cachedKey]{}
	if spec.kind != 0 && spec.kind != 4 {
		keyOps.Drop = keyDrop
	}
	comparison := func(label string, a, b *cachedKey, c Context) Ordering {
		if spec.kind == 2 {
			a.touches++
			b.touches++
		}
		index := compares
		compares++
		event(fmt.Sprintf("%s:%d:%d:%d:%d:%t", label, a.id, b.id, a.touches, b.touches, c.Panicking()))
		if index == spec.cmpFail {
			raise(c, "key_compare")
		}
		if a.key < b.key {
			return LessThan
		}
		if a.key > b.key {
			return GreaterThan
		}
		return Equal
	}
	keyOps.PartialCompare = func(a, b *cachedKey, c Context) (Ordering, bool) { return comparison("P", a, b, c), true }
	if spec.chain {
		keyOps.ChainingLess = func(a, b *cachedKey, c Context) ChainingLess {
			order := comparison("C", a, b, c)
			return ChainingLess{Continue: order == Equal, Less: order == LessThan}
		}
	}
	environment := rustvec.NewOwned(cachedEnvironment{token: true}, rustvec.ValueOps[cachedEnvironment]{Drop: func(env *cachedEnvironment, c Context) {
		// This witness includes the complete Rust field-drop glue. Token is
		// destroyed even when the Function destructor body starts unwinding.
		defer func() {
			p := recover()
			if env.token {
				env.token = false
				if p != nil {
					unwind := c
					unwind.Unwinding = true
					func() {
						defer func() {
							if recover() != nil {
								unwind.AbortCleanup()
							}
						}()
						tokenDrop(unwind)
					}()
				} else {
					tokenDrop(c)
				}
			}
			if p != nil {
				c.ResumeUnwind(p)
			}
		}()
		event(fmt.Sprintf("FD:%d:%t:%t:%t", env.calls, env.token, env.calls == 0 || env.first == env, c.Panicking()))
		if spec.fnFail {
			raise(c, "function_drop")
		}
	}})
	function := NewKeyFunction(environment, func(env *cachedEnvironment, _ *struct{}, c Context) *rustvec.Owned[cachedKey] {
		if env.calls == 0 {
			env.first = env
		}
		index := env.calls
		value := &cachedItem{key: int64((index*7 + 3) % 5), id: uint64(index)}
		env.calls++
		event(fmt.Sprintf("F:%d:%d:%t:%t", value.id, env.calls, env.first == env, c.Panicking()))
		if spec.moved && index == 0 {
			env.token = false
			movedToken = true
			event(fmt.Sprintf("move:%t", c.Panicking()))
		}
		if index == spec.keyFail {
			raise(c, "key_function")
		}
		key := cachedKey{key: value.key, id: value.id}
		if spec.kind == 0 {
			key.key = int64(uint8(value.key))
			key.id = uint64(key.key)
		}
		if spec.kind == 3 {
			key.key, key.id = 0, 0
		}
		return rustvec.NewOwned(key, rustvec.ValueOps[cachedKey]{Drop: keyOps.Drop})
	})
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		zero := struct{}{}
		CachedZeroSized(uint(spec.length), &zero, function, CachedConfig{KeyType: typ, PointerBits: 64}, keyOps, context)
	}()
	event(fmt.Sprintf("end:%t:%t", message != "", context.Panicking()))
	if movedToken {
		tokenDrop(context)
	}
	return message, fmt.Sprint(uint(spec.length)), strings.Join(trace, "\n"), compares, drops
}

type cachedZeroWire struct {
	Context    string
	Request    cachedWireCase
	Resume     bool
	Outcome    string
	Stdout     string
	NativeCode int `json:"native_exit_code"`
}
type cachedZeroManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type cachedZeroControl bool

func (c cachedZeroControl) ResumeCachedPanic() bool { return bool(c) }

type cachedZeroRunner struct {
	spec      cachedZeroWire
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *cachedZeroRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *cachedZeroRunner) body(c Context) {
	r.event("scope:%t", c.Panicking())
	defer func() { r.event("scope_drop:%t", c.Panicking()) }()
	request := r.spec.Request
	spec := cachedCase{kind: request.Kind, chain: request.Chain, length: request.Length, seed: request.Seed, keyFail: request.KeyFail, cmpFail: request.CmpFail, dropFail: request.DropFail, fnFail: request.FnFail, tokenFail: request.TokenFail, moved: request.Moved}
	message, state, _, compares, drops := replayCachedZeroContext(spec, c, func(value string) { r.event("%s", value) })
	r.event("RESULT:%s:%d:%d:%s", hex.EncodeToString([]byte(message)), compares, drops, hex.EncodeToString([]byte(state)))
}
func (r *cachedZeroRunner) run() {
	c := Context{Runtime: &rustfmt.RuntimeMetadata{Value: cachedZeroControl(r.spec.Resume)}}
	locations := map[string]rustfmt.SourceLocation{}
	for _, name := range []string{"key_function", "key_compare", "key_drop", "function_drop", "token_drop"} {
		locations["cached.client."+name] = r.locations[name]
	}
	locations[rustvec.PanicCapacityOverflowSite] = r.locations["capacity overflow"]
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c = rustfmt.WithPanicOperations(c, &rustfmt.PanicOperations{Locations: locations, CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, c Context) {
		location := rustfmt.SourceLocation{}
		if info.Location != nil {
			location = *info.Location
		}
		r.event("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(location.File)), location.Line, location.Column)
		if r.spec.Context == "processing" {
			r.body(c)
		}
	}})
	if r.spec.Context == "normal" {
		r.body(c)
		return
	}
	func() {
		defer func() {
			if recover() != nil && r.spec.Context == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		location := r.locations["outer."+r.spec.Context]
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func cachedZeroDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadCachedZeroManifest(t *testing.T) cachedZeroManifest {
	t.Helper()
	data, err := os.ReadFile("../../../tools/port-rust-unord/cached-zero-sized/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata cachedZeroManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for path, want := range metadata.Files {
		data, err := os.ReadFile("../../../" + path)
		if err != nil || cachedZeroDigest(data) != want {
			t.Fatalf("ZST binding %s: %v", path, err)
		}
	}
	return metadata
}
func TestOriginalCachedZeroSized(t *testing.T) {
	metadata := loadCachedZeroManifest(t)
	data, err := os.ReadFile("testdata/cached_zero_sized.jsonl.gz")
	if err != nil || cachedZeroDigest(data) != metadata.Fixture {
		t.Fatal("ZST fixture", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 8<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if cachedZeroDigest(raw) != metadata.Raw {
		t.Fatal("ZST raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec cachedZeroWire
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		var reference strings.Builder
		for _, line := range strings.SplitAfter(spec.Stdout, "\n") {
			if strings.HasPrefix(line, "ALLOC:") {
				fields := strings.Split(strings.TrimSpace(line), ":")
				if len(fields) != 3 {
					t.Fatal("native allocation record")
				}
				size, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				align, err := strconv.ParseUint(fields[2], 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if size != 0 {
					typ := cachedZeroIndexType(uint(spec.Request.Length), CachedConfig{KeyType: cachedSourceType(spec.Request.Kind), PointerBits: 64}, Context{})
					if uint64(typ.Size)*uint64(spec.Request.Length) != size || uint64(typ.Align) != align {
						t.Fatalf("actual native index layout %d/%d != %d/%d at %+v", typ.Size*uint(spec.Request.Length), typ.Align, size, align, spec.Request)
					}
				}
				continue
			}
			reference.WriteString(line)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestCachedZeroSizedChild$")
			command.Env = append(os.Environ(), "FERRUGO_CACHED_ZERO_CHILD="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("ZST child %d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := cachedZeroRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != reference.String() {
			t.Fatalf("ZST case %d %+v\nGOT\n%sWANT\n%s", count, spec, got, reference.String())
		}
		count++
	}
	if scanner.Err() != nil || count != 624 || metadata.Cases != 624 || aborts != 208 || metadata.Aborts != 208 {
		t.Fatal("ZST coverage", count, aborts, scanner.Err())
	}
}
func TestCachedZeroSizedChild(t *testing.T) {
	data := os.Getenv("FERRUGO_CACHED_ZERO_CHILD")
	if data == "" {
		return
	}
	metadata := loadCachedZeroManifest(t)
	var spec cachedZeroWire
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := cachedZeroRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
