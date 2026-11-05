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

type itemsBooleanCase struct {
	Mode                 string
	Action, Length, Stop int
	NextFail             int  `json:"next_fail"`
	CallFail             int  `json:"call_fail"`
	ItemFail             int  `json:"item_fail"`
	EnvFail              bool `json:"env_fail"`
	TokenFail            bool `json:"token_fail"`
	IteratorFail         bool `json:"iterator_fail"`
	Resume               bool
	Outcome, Stdout      string
}
type itemsBooleanState struct{ next int }
type itemsBooleanFunctionState struct{ calls int }
type itemsBooleanManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type itemsBooleanRunner struct {
	spec      itemsBooleanCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *itemsBooleanRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *itemsBooleanRunner) raise(value string, c rustfmt.Context) {
	if r.spec.Resume {
		c.ResumeUnwind(value)
	}
	location := r.locations[value]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: value, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *itemsBooleanRunner) dropItem(value *uint64, c rustfmt.Context) {
	r.event("item:%d:%t", *value, c.Panicking())
	if int(*value) == r.spec.ItemFail {
		r.raise("item_drop", c)
	}
}
func (r *itemsBooleanRunner) dropIterator(state *itemsBooleanState, c rustfmt.Context) {
	r.event("iterator:%d:%t", state.next, c.Panicking())
	if r.spec.IteratorFail {
		r.raise("iterator_drop", c)
	}
}
func (r *itemsBooleanRunner) newIterator() *UnordItems[uint64] {
	owner := rustvec.NewOwned(itemsBooleanState{}, rustvec.ValueOps[itemsBooleanState]{Drop: r.dropIterator})
	var next func(rustvec.Ref[itemsBooleanState]) *rustvec.Owned[uint64]
	next = func(ref rustvec.Ref[itemsBooleanState]) *rustvec.Owned[uint64] {
		index := ref.Value.next
		ref.Value.next++
		r.event("next:%d:%t", index, ref.Context.Panicking())
		if index == r.spec.NextFail {
			r.raise("next", ref.Context)
		}
		if index < r.spec.Length {
			return rustvec.NewOwned(uint64(index), rustvec.ValueOps[uint64]{Drop: r.dropItem})
		}
		return nil
	}
	booleans := &UnordBooleanOps[itemsBooleanState, uint64]{DefaultTryFold: true}
	if r.spec.Action/2 == 1 || r.spec.Action/2 >= 3 {
		booleans.DefaultTryFold = false
		booleans.TryFold = func(ref rustvec.Ref[itemsBooleanState], consume UnordTryFoldFunction[uint64]) bool {
			r.event("try_fold")
			switch r.spec.Action / 2 {
			case 3:
				consume.Close(ref.Context)
				r.event("after_check_drop")
				return true
			case 4:
				consume.Forget(ref.Context)
				r.event("after_check_forget")
				return true
			case 5:
				value := next(ref)
				if value == nil {
					return true
				}
				result := consume.CallOnce(value, ref.Context)
				r.event("after_check_once")
				return result
			}
			for {
				value := next(ref)
				if value == nil {
					return true
				}
				if !consume.CallMut(value, ref.Context) {
					return false
				}
			}
		}
	} else if r.spec.Action/2 == 2 {
		override := func(ref rustvec.Ref[itemsBooleanState], f UnordFunction[uint64, bool], name string) bool {
			r.event(name)
			return f.CallMut(rustvec.NewOwned(uint64(99), rustvec.ValueOps[uint64]{Drop: r.dropItem}), ref.Context).Take()
		}
		booleans.All = func(ref rustvec.Ref[itemsBooleanState], f UnordFunction[uint64, bool]) bool {
			return override(ref, f, "all_override")
		}
		booleans.Any = func(ref rustvec.Ref[itemsBooleanState], f UnordFunction[uint64, bool]) bool {
			return override(ref, f, "any_override")
		}
	}
	return NewUnordItems(owner, UnordIteratorOps[itemsBooleanState, uint64]{Next: next, SizeHint: func(rustvec.Ref[itemsBooleanState]) rustvec.Hint { return rustvec.Hint{} }, Count: func(*rustvec.Owned[itemsBooleanState], rustfmt.Context) uint { return uint(r.spec.Length) }, Boolean: booleans})
}
func (r *itemsBooleanRunner) dropToken(c rustfmt.Context) {
	r.event("token:%t", c.Panicking())
	if r.spec.TokenFail {
		r.raise("token_drop", c)
	}
}
func (r *itemsBooleanRunner) dropFunction(state *itemsBooleanFunctionState, c rustfmt.Context) {
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			bagItemsUnwind(unwind, func() { r.dropToken(unwind) })
			c.ResumeUnwind(p)
		}
		r.dropToken(c)
	}()
	r.event("function:%d:%t", state.calls, c.Panicking())
	if r.spec.EnvFail {
		r.raise("function_drop", c)
	}
}
func (r *itemsBooleanRunner) newFunction() UnordFunction[uint64, bool] {
	body := func(ref rustvec.Ref[itemsBooleanFunctionState], argument *rustvec.Owned[uint64], which string) *rustvec.Owned[bool] {
		call := ref.Value.calls
		ref.Value.calls++
		r.event("%s:%d:%d:%t", which, call, *argument.Borrow(), ref.Context.Panicking())
		if call == r.spec.CallFail {
			r.raise("call", ref.Context)
		}
		return rustvec.Own(int(*argument.Borrow()) != r.spec.Stop)
	}
	return NewUnordFunction(rustvec.NewOwned(itemsBooleanFunctionState{}, rustvec.ValueOps[itemsBooleanFunctionState]{Drop: r.dropFunction}), UnordFunctionOps[itemsBooleanFunctionState, uint64, bool]{
		Call: func(ref rustvec.Ref[itemsBooleanFunctionState], arg *rustvec.Owned[uint64]) *rustvec.Owned[bool] {
			return body(ref, arg, "shared")
		},
		CallMut: func(ref rustvec.Ref[itemsBooleanFunctionState], arg *rustvec.Owned[uint64]) *rustvec.Owned[bool] {
			return body(ref, arg, "mut")
		},
		CallOnce: func(state *rustvec.Owned[itemsBooleanFunctionState], arg *rustvec.Owned[uint64], c rustfmt.Context) *rustvec.Owned[bool] {
			return body(rustvec.Ref[itemsBooleanFunctionState]{Value: state.Borrow(), Context: c}, arg, "once")
		},
	})
}
func (r *itemsBooleanRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		items := r.newIterator()
		f := r.newFunction()
		var value bool
		if r.spec.Action%2 == 1 {
			value = items.Any(f, c)
		} else {
			value = items.All(f, c)
		}
		r.event("returned:%t", value)
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemsBooleanRunner) run() {
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
func itemsBooleanDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func loadItemsBooleanManifest(t *testing.T) itemsBooleanManifest {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/items/boolean_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m itemsBooleanManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || itemsBooleanDigest(b) != h {
			t.Fatalf("Items boolean source binding %s: %v", p, e)
		}
	}
	return m
}
func TestOriginalUnordItemsBoolean(t *testing.T) {
	m := loadItemsBooleanManifest(t)
	b, e := os.ReadFile("testdata/unord_items_boolean.jsonl.gz")
	if e != nil || itemsBooleanDigest(b) != m.Fixture {
		t.Fatal("Items fixture digest", e)
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
	if itemsBooleanDigest(raw) != m.Raw {
		t.Fatal("Items raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemsBooleanCase
		if e = json.Unmarshal(scanner.Bytes(), &spec); e != nil {
			t.Fatal(e)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			encoded, _ := json.Marshal(spec)
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsBooleanChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_BOOLEAN="+string(encoded))
			out, e := command.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("Items boolean case%d abort: %v %s", count, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			runner := itemsBooleanRunner{spec: spec, locations: m.Locations, out: &out}
			runner.run()
			got = out.String()
		}
		if got != spec.Stdout {
			t.Fatalf("Items boolean case%d %#v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 1224 || m.Cases != 1224 || aborts != m.Aborts {
		t.Fatal("Items coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsBooleanChild(t *testing.T) {
	value := os.Getenv("FERRUGO_UNORD_ITEMS_BOOLEAN")
	if value == "" {
		return
	}
	m := loadItemsBooleanManifest(t)
	var spec itemsBooleanCase
	if e := json.Unmarshal([]byte(value), &spec); e != nil {
		t.Fatal(e)
	}
	runner := itemsBooleanRunner{spec: spec, locations: m.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
