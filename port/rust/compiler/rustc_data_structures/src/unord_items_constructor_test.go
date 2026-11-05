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

type itemsConstructorCase struct {
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
type itemsConstructorState struct{ next int }
type itemsConstructorFunctionState struct{ calls int }
type itemsConstructorManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type itemsConstructorRunner struct {
	spec      itemsConstructorCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *itemsConstructorRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *itemsConstructorRunner) raise(value string, c rustfmt.Context) {
	if r.spec.Resume {
		c.ResumeUnwind(value)
	}
	key := value
	if value == "constructor" {
		key = fmt.Sprintf("constructor.%d", r.spec.Action)
	}
	location := r.locations[key]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: value, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *itemsConstructorRunner) dropItem(value *uint64, c rustfmt.Context) {
	r.event("item:%d:%t", *value, c.Panicking())
	if int(*value) == r.spec.ItemFail {
		r.raise("item_drop", c)
	}
}
func (r *itemsConstructorRunner) dropIterator(state *itemsConstructorState, c rustfmt.Context) {
	r.event("iterator:%d:%t", state.next, c.Panicking())
	if r.spec.IteratorFail {
		r.raise("iterator_drop", c)
	}
}
func (r *itemsConstructorRunner) newIterator() *UnordItems[uint64] {
	owner := rustvec.NewOwned(itemsConstructorState{}, rustvec.ValueOps[itemsConstructorState]{Drop: r.dropIterator})
	var next func(rustvec.Ref[itemsConstructorState]) *rustvec.Owned[uint64]
	next = func(ref rustvec.Ref[itemsConstructorState]) *rustvec.Owned[uint64] {
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
	booleans := &UnordBooleanOps[itemsConstructorState, uint64]{DefaultTryFold: true}
	if r.spec.Action/2 == 1 {
		booleans.DefaultTryFold = false
		booleans.TryFold = func(ref rustvec.Ref[itemsConstructorState], consume UnordTryFoldFunction[uint64]) bool {
			r.event("try_fold")
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
		override := func(ref rustvec.Ref[itemsConstructorState], f UnordFunction[uint64, bool], name string) bool {
			r.event(name)
			return f.CallMut(rustvec.NewOwned(uint64(99), rustvec.ValueOps[uint64]{Drop: r.dropItem}), ref.Context).Take()
		}
		booleans.All = func(ref rustvec.Ref[itemsConstructorState], f UnordFunction[uint64, bool]) bool {
			return override(ref, f, "all_override")
		}
		booleans.Any = func(ref rustvec.Ref[itemsConstructorState], f UnordFunction[uint64, bool]) bool {
			return override(ref, f, "any_override")
		}
	}
	return NewUnordItems(owner, UnordIteratorOps[itemsConstructorState, uint64]{Next: next, SizeHint: func(rustvec.Ref[itemsConstructorState]) rustvec.Hint { return rustvec.Hint{} }, Count: func(*rustvec.Owned[itemsConstructorState], rustfmt.Context) uint { return uint(r.spec.Length) }, Boolean: booleans})
}
func (r *itemsConstructorRunner) dropToken(c rustfmt.Context) {
	r.event("token:%t", c.Panicking())
	if r.spec.TokenFail {
		r.raise("token_drop", c)
	}
}
func (r *itemsConstructorRunner) dropFunction(state *itemsConstructorFunctionState, c rustfmt.Context) {
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
func (r *itemsConstructorRunner) newFunction() UnordFunction[uint64, bool] {
	body := func(ref rustvec.Ref[itemsConstructorFunctionState], argument *rustvec.Owned[uint64], which string) *rustvec.Owned[bool] {
		call := ref.Value.calls
		ref.Value.calls++
		r.event("%s:%d:%d:%t", which, call, *argument.Borrow(), ref.Context.Panicking())
		if call == r.spec.CallFail {
			r.raise("call", ref.Context)
		}
		return rustvec.Own(int(*argument.Borrow()) != r.spec.Stop)
	}
	return NewUnordFunction(rustvec.NewOwned(itemsConstructorFunctionState{}, rustvec.ValueOps[itemsConstructorFunctionState]{Drop: r.dropFunction}), UnordFunctionOps[itemsConstructorFunctionState, uint64, bool]{
		Call: func(ref rustvec.Ref[itemsConstructorFunctionState], arg *rustvec.Owned[uint64]) *rustvec.Owned[bool] {
			return body(ref, arg, "shared")
		},
		CallMut: func(ref rustvec.Ref[itemsConstructorFunctionState], arg *rustvec.Owned[uint64]) *rustvec.Owned[bool] {
			return body(ref, arg, "mut")
		},
		CallOnce: func(state *rustvec.Owned[itemsConstructorFunctionState], arg *rustvec.Owned[uint64], c rustfmt.Context) *rustvec.Owned[bool] {
			return body(rustvec.Ref[itemsConstructorFunctionState]{Value: state.Borrow(), Context: c}, arg, "once")
		},
	})
}

func (r *itemsConstructorRunner) newFlatFunction() UnordFunction[uint64, *UnordItems[uint64]] {
	body := func(ref rustvec.Ref[itemsConstructorFunctionState], argument *rustvec.Owned[uint64], which string) *rustvec.Owned[*UnordItems[uint64]] {
		call := ref.Value.calls
		ref.Value.calls++
		r.event("%s:%d:%d:%t", which, call, *argument.Borrow(), ref.Context.Panicking())
		if call == r.spec.CallFail {
			r.raise("call", ref.Context)
		}
		items := r.newIterator()
		items.owner.(*unordIteratorCell[itemsConstructorState, uint64]).state.Borrow().next = 77
		return rustvec.NewOwned(items, rustvec.ValueOps[*UnordItems[uint64]]{Drop: func(items **UnordItems[uint64], c rustfmt.Context) { (*items).Close(c) }})
	}
	return NewUnordFunction(rustvec.NewOwned(itemsConstructorFunctionState{}, rustvec.ValueOps[itemsConstructorFunctionState]{Drop: r.dropFunction}), UnordFunctionOps[itemsConstructorFunctionState, uint64, *UnordItems[uint64]]{
		Call: func(ref rustvec.Ref[itemsConstructorFunctionState], arg *rustvec.Owned[uint64]) *rustvec.Owned[*UnordItems[uint64]] {
			return body(ref, arg, "shared")
		},
		CallMut: func(ref rustvec.Ref[itemsConstructorFunctionState], arg *rustvec.Owned[uint64]) *rustvec.Owned[*UnordItems[uint64]] {
			return body(ref, arg, "mut")
		},
		CallOnce: func(state *rustvec.Owned[itemsConstructorFunctionState], arg *rustvec.Owned[uint64], c rustfmt.Context) *rustvec.Owned[*UnordItems[uint64]] {
			return body(rustvec.Ref[itemsConstructorFunctionState]{Value: state.Borrow(), Context: c}, arg, "once")
		},
	})
}

type itemsConstructorMapState struct {
	input    *rustvec.Owned[itemsConstructorState]
	function UnordFunction[uint64, bool]
}

func (r *itemsConstructorRunner) dropMap(state *itemsConstructorMapState, c rustfmt.Context) {
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			bagItemsUnwind(unwind, func() { state.function.Close(unwind) })
			c.ResumeUnwind(p)
		}
		state.function.Close(c)
	}()
	state.input.DropWithContext(c)
}
func (r *itemsConstructorRunner) selectedConstructor(inputOps UnordIteratorOps[itemsConstructorState, uint64]) UnordIteratorConstructor[itemsConstructorState, uint64, UnordFunction[uint64, bool], itemsConstructorMapState, bool] {
	next := func(state *itemsConstructorMapState, c rustfmt.Context) *rustvec.Owned[bool] {
		value := inputOps.Next(rustvec.Ref[itemsConstructorState]{Value: state.input.Borrow(), Context: c})
		if value == nil {
			return nil
		}
		return state.function.CallMut(value, c)
	}
	return UnordIteratorConstructor[itemsConstructorState, uint64, UnordFunction[uint64, bool], itemsConstructorMapState, bool]{
		Construct: func(input *rustvec.Owned[itemsConstructorState], f *rustvec.Owned[UnordFunction[uint64, bool]], c rustfmt.Context) *rustvec.Owned[itemsConstructorMapState] {
			if r.spec.Action == 2 {
				r.event("map_override")
				value := f.Take().CallOnce(rustvec.NewOwned(uint64(99), rustvec.ValueOps[uint64]{Drop: r.dropItem}), c)
				value.DropWithContext(c)
				r.raise("constructor", c)
			}
			state := itemsConstructorMapState{input: input.MoveWithContext(rustfmt.Context{}), function: f.Take().Move(rustfmt.Context{})}
			return rustvec.NewOwned(state, rustvec.ValueOps[itemsConstructorMapState]{Drop: r.dropMap})
		},
		Result: UnordIteratorOps[itemsConstructorMapState, bool]{
			Next: func(ref rustvec.Ref[itemsConstructorMapState]) *rustvec.Owned[bool] {
				return next(ref.Value, ref.Context)
			},
			SizeHint: func(ref rustvec.Ref[itemsConstructorMapState]) rustvec.Hint {
				return inputOps.SizeHint(rustvec.Ref[itemsConstructorState]{Value: ref.Value.input.Borrow(), Context: ref.Context})
			},
			Count: func(owner *rustvec.Owned[itemsConstructorMapState], c rustfmt.Context) uint {
				state := owner.Take()
				return rustvec.ConsumeValue(state.input, c, func(input *rustvec.Owned[itemsConstructorState]) uint {
					defer unordFunctionParameterCleanup(state.function, c)
					count := uint(0)
					for {
						value := inputOps.Next(rustvec.Ref[itemsConstructorState]{Value: input.Borrow(), Context: c})
						if value == nil {
							break
						}
						state.function.CallMut(value, c).DropWithContext(c)
						count++
					}
					return count
				})
			},
			Boolean: &UnordBooleanOps[itemsConstructorMapState, bool]{DefaultTryFold: true},
		},
	}
}
func (r *itemsConstructorRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		items := r.newIterator()
		if r.spec.Action == 3 {
			// The selected foreign Iterator::flat_map override always raises.
			// Its never-produced iterator representation is a fail-fast unused
			// test interface, not a substituted std Flatten implementation.
			selected := UnordIteratorConstructor[itemsConstructorState, uint64, UnordFunction[uint64, itemsConstructorState], struct{}, uint64]{
				Construct: func(input *rustvec.Owned[itemsConstructorState], f *rustvec.Owned[UnordFunction[uint64, itemsConstructorState]], c rustfmt.Context) *rustvec.Owned[struct{}] {
					r.event("flat_map_override")
					inner := f.Take().CallOnce(rustvec.NewOwned(uint64(99), rustvec.ValueOps[uint64]{Drop: r.dropItem}), c)
					r.event("inner_returned")
					inner.DropWithContext(c)
					r.raise("constructor", c)
					return nil
				},
				Result: UnordIteratorOps[struct{}, uint64]{
					Next:     func(rustvec.Ref[struct{}]) *rustvec.Owned[uint64] { panic("unused never-returned flat-map iterator") },
					SizeHint: func(rustvec.Ref[struct{}]) rustvec.Hint { panic("unused never-returned flat-map iterator") },
					Count:    func(*rustvec.Owned[struct{}], rustfmt.Context) uint { panic("unused never-returned flat-map iterator") },
				},
			}
			FlatMapUnordItems(items, r.newFlatFunction(), selected, c)
			r.event("unexpected_return")
			return
		}
		f := r.newFunction()
		dictionary := items.owner.(*unordIteratorCell[itemsConstructorState, uint64]).ops
		mapped := MapUnordItems(items, f, r.selectedConstructor(dictionary), c)
		switch r.spec.Action {
		case 0:
			value, ok := mapped.GetOnly(c)
			if ok {
				r.event("returned:%t", value.Take())
			} else {
				r.event("returned:none")
			}
		case 1:
			r.event("returned:%d", mapped.Count(c))
		default:
			r.event("unexpected_return")
		}
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemsConstructorRunner) run() {
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
func itemsConstructorDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func loadItemsConstructorManifest(t *testing.T) itemsConstructorManifest {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/items/constructor_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m itemsConstructorManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || itemsConstructorDigest(b) != h {
			t.Fatalf("Items boolean source binding %s: %v", p, e)
		}
	}
	return m
}
func TestOriginalUnordItemsConstructor(t *testing.T) {
	m := loadItemsConstructorManifest(t)
	b, e := os.ReadFile("testdata/unord_items_constructor.jsonl.gz")
	if e != nil || itemsConstructorDigest(b) != m.Fixture {
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
	if itemsConstructorDigest(raw) != m.Raw {
		t.Fatal("Items raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemsConstructorCase
		if e = json.Unmarshal(scanner.Bytes(), &spec); e != nil {
			t.Fatal(e)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			encoded, _ := json.Marshal(spec)
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsConstructorChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_CONSTRUCTOR="+string(encoded))
			out, e := command.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("Items boolean case%d abort: %v %s", count, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			runner := itemsConstructorRunner{spec: spec, locations: m.Locations, out: &out}
			runner.run()
			got = out.String()
		}
		if got != spec.Stdout {
			t.Fatalf("Items boolean case%d %#v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 408 || m.Cases != 408 || aborts != m.Aborts {
		t.Fatal("Items coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsConstructorChild(t *testing.T) {
	value := os.Getenv("FERRUGO_UNORD_ITEMS_CONSTRUCTOR")
	if value == "" {
		return
	}
	m := loadItemsConstructorManifest(t)
	var spec itemsConstructorCase
	if e := json.Unmarshal([]byte(value), &spec); e != nil {
		t.Fatal(e)
	}
	runner := itemsConstructorRunner{spec: spec, locations: m.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
