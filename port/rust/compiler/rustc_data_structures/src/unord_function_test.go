// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

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
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type functionCase struct {
	Mode            string
	Kind, Flags     int
	Resume          bool
	Outcome, Stdout string
}
type functionManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type functionState struct {
	calls uint64
	token bool
}
type functionRunner struct {
	spec      functionCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *functionRunner) event(format string, args ...any) { fmt.Fprintf(r.out, format+"\n", args...) }
func (r *functionRunner) raise(value string, c rustfmt.Context) {
	if r.spec.Resume {
		c.ResumeUnwind(value)
	}
	location := r.locations[value]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: value, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *functionRunner) dropValue(value *uint64, c rustfmt.Context) {
	r.event("value:%d:%t", *value, c.Panicking())
	if *value == 7 && r.spec.Flags&1 != 0 {
		r.raise("argument", c)
	}
}
func (r *functionRunner) dropToken(c rustfmt.Context) {
	r.event("token:%t", c.Panicking())
	if r.spec.Flags&4 != 0 {
		r.raise("token", c)
	}
}
func (r *functionRunner) dropEnvironment(state *functionState, c rustfmt.Context) {
	fieldsDone := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !fieldsDone && state.token {
				state.token = false
				unwind := c
				unwind.Unwinding = true
				func() {
					defer func() {
						if recover() != nil {
							unwind.AbortCleanup()
						}
					}()
					r.dropToken(unwind)
				}()
			}
			c.ResumeUnwind(p)
		}
	}()
	r.event("environment:%d:%t", state.calls, c.Panicking())
	if r.spec.Flags&2 != 0 {
		r.raise("environment", c)
	}
	fieldsDone = true
	if state.token {
		state.token = false
		r.dropToken(c)
	}
}
func (r *functionRunner) result(value uint64) *rustvec.Owned[uint64] {
	return rustvec.NewOwned(value, rustvec.ValueOps[uint64]{Drop: r.dropValue})
}
func (r *functionRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		state := rustvec.NewOwned(functionState{calls: 10, token: true}, rustvec.ValueOps[functionState]{Drop: r.dropEnvironment})
		operation := func(name string, state *functionState, argument *rustvec.Owned[uint64], base uint64, c rustfmt.Context) *rustvec.Owned[uint64] {
			state.calls++
			r.event("%s:%d:%t", name, state.calls, c.Panicking())
			return r.result(*argument.Borrow() + base)
		}
		fn := NewUnordFunction(state, UnordFunctionOps[functionState, uint64, uint64]{Call: func(state rustvec.Ref[functionState], argument *rustvec.Owned[uint64]) *rustvec.Owned[uint64] {
			return operation("shared", state.Value, argument, 1000, state.Context)
		}, CallMut: func(state rustvec.Ref[functionState], argument *rustvec.Owned[uint64]) *rustvec.Owned[uint64] {
			return operation("mutable", state.Value, argument, 2000, state.Context)
		}, CallOnce: func(state *rustvec.Owned[functionState], argument *rustvec.Owned[uint64], c rustfmt.Context) *rustvec.Owned[uint64] {
			return operation("once", state.Borrow(), argument, 3000, c)
		}, Clone: func(state rustvec.Ref[functionState]) *rustvec.Owned[functionState] {
			r.event("clone:%d:%t", state.Value.calls, state.Context.Panicking())
			return rustvec.NewOwned(functionState{calls: state.Value.calls, token: true}, rustvec.ValueOps[functionState]{Drop: r.dropEnvironment})
		}})
		if r.spec.Kind == 3 {
			finish := func(value *rustvec.Owned[uint64]) {
				r.event("call_returned:%d", *value.Borrow())
				value.DropWithContext(c)
			}
			finish(fn.CallMut(r.result(7), c))
			finish(fn.CallMut(r.result(7), c))
			cloned := fn.Clone(c)
			finish(cloned.Call(r.result(7), c))
			cloned.Close(c)
			fn.Close(c)
			return
		}
		var output *rustvec.Owned[uint64]
		callMessage := ""
		func() {
			defer func() {
				if p := recover(); p != nil {
					callMessage = fmt.Sprint(p)
				}
			}()
			argument := r.result(7)
			switch r.spec.Kind {
			case 0:
				output = fn.Call(argument, c)
			case 1:
				output = fn.CallMut(argument, c)
			case 2:
				output = fn.CallOnce(argument, c)
			default:
				panic("source Fn kind")
			}
		}()
		if output != nil {
			r.event("call_returned:%d", *output.Borrow())
			output.DropWithContext(c)
		} else {
			r.event("call_caught:%s", hex.EncodeToString([]byte(callMessage)))
		}
		if r.spec.Kind != 2 {
			fn.Close(c)
		}
	}()
	r.event("body_caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *functionRunner) run() {
	var c rustfmt.Context
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c = rustfmt.WithPanicOperations(c, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
		location := rustfmt.SourceLocation{}
		if info.Location != nil {
			location = *info.Location
		}
		r.event("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(location.File)), location.Line, location.Column)
		if r.spec.Mode == "processing" {
			r.body(c)
		}
	}})
	if r.spec.Mode == "normal" {
		r.body(c)
		return
	}
	outer := r.locations["outer."+r.spec.Mode]
	func() {
		defer func() {
			if recover() != nil && r.spec.Mode == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &outer, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func functionDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func loadFunctionManifest(t *testing.T) functionManifest {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/items/function_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m functionManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || functionDigest(b) != h {
			t.Fatalf("Fn source binding %s: %v", p, e)
		}
	}
	return m
}
func TestOriginalUnordFunction(t *testing.T) {
	m := loadFunctionManifest(t)
	b, e := os.ReadFile("testdata/unord_function.jsonl.gz")
	if e != nil || functionDigest(b) != m.Fixture {
		t.Fatal("Fn fixture digest", e)
	}
	z, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(io.LimitReader(z, 2<<20))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if functionDigest(raw) != m.Raw {
		t.Fatal("Fn raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec functionCase
		if e = json.Unmarshal(scanner.Bytes(), &spec); e != nil {
			t.Fatal(e)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			encoded, _ := json.Marshal(spec)
			command := exec.Command(os.Args[0], "-test.run=^TestUnordFunctionChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_FUNCTION="+string(encoded))
			out, e := command.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("Fn case%d abort: %v %s", count, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			runner := functionRunner{spec: spec, locations: m.Locations, out: &out}
			runner.run()
			got = out.String()
		}
		if got != spec.Stdout {
			t.Fatalf("Fn case%d %#v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 147 || m.Cases != 147 || aborts != 74 || m.Aborts != 74 {
		t.Fatal("Fn coverage", count, aborts, scanner.Err())
	}
}
func TestUnordFunctionChild(t *testing.T) {
	value := os.Getenv("FERRUGO_UNORD_FUNCTION")
	if value == "" {
		return
	}
	m := loadFunctionManifest(t)
	var spec functionCase
	if e := json.Unmarshal([]byte(value), &spec); e != nil {
		t.Fatal(e)
	}
	runner := functionRunner{spec: spec, locations: m.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
