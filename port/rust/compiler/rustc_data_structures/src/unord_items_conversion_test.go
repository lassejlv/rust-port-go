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
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type conversionRequest struct {
	Nested    bool
	Length    int
	NextFail  int  `json:"next_fail"`
	CallFail  bool `json:"call_fail"`
	ItemFail  int  `json:"item_fail"`
	InputFail bool `json:"input_fail"`
	HashFail  int  `json:"hash_fail"`
	Resume    bool
}
type conversionCase struct {
	Mode            string
	Family          int
	Request         conversionRequest
	Outcome, Stdout string
}
type conversionManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type conversionValue struct{ value, id uint64 }
type conversionInput[T any] struct {
	values *rustcindex.OwnedIterator[T]
	calls  int
}
type conversionRunner struct {
	spec      conversionCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
	observe   bool
	hashes    int
}

func (r *conversionRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *conversionRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func conversionFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		bagItemsUnwind(unwind, func() { drop(unwind) })
		c.ResumeUnwind(p)
	}
	drop(c)
}
func (r *conversionRunner) dropValue(v *conversionValue, c rustfmt.Context) {
	r.event("item:%d:%t", v.id, c.Panicking())
	if int(v.id) == r.spec.Request.ItemFail {
		r.raise("item_drop", c)
	}
}
func (r *conversionRunner) keyTraits() rusthashmap.KeyOps[conversionValue] {
	return rusthashmap.KeyOps[conversionValue]{
		Hash: func(v *conversionValue, h rusthashmap.Hasher, c rustfmt.Context) {
			if r.observe {
				n := r.hashes
				r.hashes++
				r.event("hash:%d:%t", v.id, c.Panicking())
				if n == r.spec.Request.HashFail {
					r.raise("hash", c)
				}
			}
			rusthashmap.HashU64(h, v.value)
		},
		Equal: func(a, b *conversionValue, c rustfmt.Context) bool {
			if r.observe {
				r.event("eq:%d:%d:%t", a.id, b.id, c.Panicking())
			}
			return a.value == b.value
		}, Drop: r.dropValue}
}
func conversionV(n int, key bool) conversionValue {
	id := 200
	if key {
		id = 100
	}
	return conversionValue{value: uint64(n % 3), id: uint64(n + id)}
}
func conversionNext[T any](r *conversionRunner, state *conversionInput[T], c rustfmt.Context) (T, bool) {
	n := state.calls
	state.calls++
	r.event("next:%d:%t", n, c.Panicking())
	if n == r.spec.Request.NextFail {
		r.raise("next", c)
	}
	return state.values.Next()
}
func conversionDrop[T any](r *conversionRunner, state *conversionInput[T], c rustfmt.Context) {
	defer conversionFieldCleanup(c, state.values.CloseWithRuntimeContext)
	r.event("input_drop:%d:%d:%t", state.calls, state.values.Len(), c.Panicking())
	if r.spec.Request.InputFail {
		r.raise("input_drop", c)
	}
}
func conversionNested[T any](r *conversionRunner, consume rustvec.BorrowedConsumer[T], value T, c rustfmt.Context) {
	func() {
		defer func() {
			if p := recover(); p != nil {
				if c.ProcessingPanic {
					c.ResumeUnwind(p)
				}
				unwind := c
				unwind.Unwinding = true
				bagItemsUnwind(unwind, func() { r.event("nested_consume:%t", unwind.Panicking()); consume.Call(value, unwind) })
			}
		}()
		location := r.locations["nested"]
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "nested", Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
	}()
}
func conversionNew[T any](r *conversionRunner, values []T, size uint, drop func(*T, rustfmt.Context)) (*UnordItems[T], UnordCollectionIteratorOps[conversionInput[T], T]) {
	vector := rustcindex.IndexVecFromRaw(values, rustcindex.UsizeIdx(), rustcindex.ValueOps[T]{RustSize: size, DropWithRuntimeContext: drop})
	owner := rustvec.NewOwned(conversionInput[T]{values: vector.IntoIter()}, rustvec.ValueOps[conversionInput[T]]{Drop: func(s *conversionInput[T], c rustfmt.Context) { conversionDrop(r, s, c) }})
	ops := rustvec.IteratorOps[conversionInput[T], T]{Next: func(ref rustvec.Ref[conversionInput[T]]) (T, bool) { return conversionNext(r, ref.Value, ref.Context) }, SizeHint: func(ref rustvec.Ref[conversionInput[T]]) rustvec.Hint {
		n := ref.Value.values.Len()
		r.event("hint:%d", n)
		return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
	}}
	each := func(owner *rustvec.Owned[conversionInput[T]], consume rustvec.BorrowedConsumer[T], label string) {
		c := owner.Context()
		r.event("%s", label)
		if r.spec.Request.CallFail {
			r.raise(label, c)
		}
		if r.spec.Request.Nested {
			value, ok := conversionNext(r, owner.Borrow(), c)
			if ok {
				conversionNested(r, consume, value, c)
			}
		}
		for {
			value, ok := conversionNext(r, owner.Borrow(), c)
			if !ok {
				return
			}
			consume.Call(value, c)
		}
	}
	ops.ForEach = func(owner *rustvec.Owned[conversionInput[T]], consume func(T)) {
		each(owner, rustvec.NewBorrowedConsumer(func(value T, _ rustfmt.Context) { consume(value) }), "for_each")
	}
	ops.FoldUnit = func(owner *rustvec.Owned[conversionInput[T]], consume func(T)) {
		each(owner, rustvec.NewBorrowedConsumer(func(value T, _ rustfmt.Context) { consume(value) }), "fold")
	}
	scoped := rustvec.ScopedIteratorOps[conversionInput[T], T]{ForEachScoped: func(owner *rustvec.Owned[conversionInput[T]], consume rustvec.BorrowedConsumer[T]) {
		each(owner, consume, "for_each")
	}, FoldUnitScoped: func(owner *rustvec.Owned[conversionInput[T]], consume rustvec.BorrowedConsumer[T]) {
		each(owner, consume, "fold")
	}}
	items := NewUnordItems(owner, UnordIteratorOps[conversionInput[T], T]{Next: func(ref rustvec.Ref[conversionInput[T]]) *rustvec.Owned[T] {
		v, ok := ops.Next(ref)
		if !ok {
			return nil
		}
		return rustvec.NewOwned(v, rustvec.ValueOps[T]{Drop: drop})
	}, SizeHint: ops.SizeHint, Count: func(*rustvec.Owned[conversionInput[T]], rustfmt.Context) uint { panic("unused Input::count") }})
	return items, UnordCollectionIteratorOps[conversionInput[T], T]{Iterator: ops, Scoped: scoped}
}
func conversionIndexInput[T any](r *conversionRunner, owner *rustvec.Owned[conversionInput[T]], c rustfmt.Context) rustcindex.IndexIntoIterator[T] {
	state := owner.Take()
	return rustcindex.IndexIntoIterator[T]{IntoIterWithContext: func(rustfmt.Context) rustcindex.IndexValueIterator[T] {
		return rustcindex.IndexValueIterator[T]{NextWithContext: func(c rustfmt.Context) (T, bool) { return conversionNext(r, &state, c) }, SizeHintWithContext: func(rustfmt.Context) (uint, uint, bool) {
			n := state.values.Len()
			r.event("hint:%d", n)
			return n, n, true
		}, CloseWithRuntimeContext: func(c rustfmt.Context) { conversionDrop(r, &state, c) }}
	}}
}
func (r *conversionRunner) execute(c rustfmt.Context) {
	r.observe = false
	request := r.spec.Request
	switch r.spec.Family {
	case 0, 3:
		config := UnordMapConfig[conversionValue, conversionValue]{Layout: rusthashmap.Layout{Size: 32, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: r.keyTraits(), Values: rusthashmap.ValueOps[conversionValue]{Drop: r.dropValue}}
		values := make([]rusthashmap.Pair[conversionValue, conversionValue], request.Length)
		for n := range values {
			values[n] = rusthashmap.Pair[conversionValue, conversionValue]{Key: conversionV(n, true), Value: conversionV(n, false)}
		}
		items, iterator := conversionNew(r, values, 32, func(p *rusthashmap.Pair[conversionValue, conversionValue], c rustfmt.Context) {
			dropUnordPair(p, config, c)
		})
		collection := NewUnordMap(config, c)
		if r.spec.Family == 3 {
			collection.Insert(conversionValue{value: 0, id: 90}, conversionValue{value: 0, id: 190}, c)
		}
		owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordMap[conversionValue, conversionValue]]{Drop: func(v **UnordMap[conversionValue, conversionValue], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[*UnordMap[conversionValue, conversionValue]]) struct{} {
			r.observe = true
			if r.spec.Family == 0 {
				collection = UnordMapFromItems(items, iterator, config, c)
				(*owner.Borrow()).Close(c)
				*owner.Borrow() = collection
			} else {
				UnordMapExtendItems(collection, items, iterator, c)
			}
			r.event("returned:%d", collection.Len())
			return struct{}{}
		})
	case 1, 4, 6:
		config := UnordSetConfig[conversionValue]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: r.keyTraits()}
		if r.spec.Family == 6 {
			collection := NewUnordSet(config, c)
			owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordSet[conversionValue]]{Drop: func(v **UnordSet[conversionValue], c rustfmt.Context) { (*v).Close(c) }})
			rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[*UnordSet[conversionValue]]) struct{} {
				for n := 0; n < request.Length; n++ {
					collection.Insert(conversionV(n, true), c)
				}
				r.observe = true
				raw := rustvec.NewOwned(collection.inner, rustvec.ValueOps[*rusthashmap.Set[conversionValue]]{Drop: func(v **rusthashmap.Set[conversionValue], c rustfmt.Context) { (*v).Close(c) }})
				collection.live = false
				converted := UnordSetFromFxSet(raw, config, c)
				*owner.Borrow() = converted
				r.event("returned:%d", converted.Len())
				return struct{}{}
			})
		} else {
			values := make([]conversionValue, request.Length)
			for n := range values {
				values[n] = conversionV(n, true)
			}
			items, iterator := conversionNew(r, values, 16, r.dropValue)
			collection := NewUnordSet(config, c)
			if r.spec.Family == 4 {
				collection.Insert(conversionValue{value: 0, id: 90}, c)
			}
			owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordSet[conversionValue]]{Drop: func(v **UnordSet[conversionValue], c rustfmt.Context) { (*v).Close(c) }})
			rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[*UnordSet[conversionValue]]) struct{} {
				r.observe = true
				if r.spec.Family == 1 {
					collection = UnordSetFromItems(items, iterator, config, c)
					(*owner.Borrow()).Close(c)
					*owner.Borrow() = collection
				} else {
					UnordSetExtendItems(collection, items, iterator, c)
				}
				r.event("returned:%d", collection.Len())
				return struct{}{}
			})
		}
	case 2, 5, 7, 8:
		config := UnordBagConfig[conversionValue]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, Values: UnordBagValueOps[conversionValue]{Drop: r.dropValue}}
		var items *UnordItems[conversionValue]
		if r.spec.Family < 7 {
			values := make([]conversionValue, request.Length)
			for n := range values {
				values[n] = conversionV(n, true)
			}
			items, _ = conversionNew(r, values, 16, r.dropValue)
		} else {
			source := NewUnordBag(config, c)
			for n := 0; n < request.Length; n++ {
				source.Push(conversionV(n, true), c)
			}
			items = source.IntoItems(c).General(c)
		}
		collection := NewUnordBag(config, c)
		if r.spec.Family == 5 || r.spec.Family == 8 {
			collection.Push(conversionValue{value: 0, id: 90}, c)
		}
		owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordBag[conversionValue]]{Drop: func(v **UnordBag[conversionValue], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[*UnordBag[conversionValue]]) struct{} {
			r.observe = true
			switch r.spec.Family {
			case 2:
				collection = UnordBagFromItems(items, UnordBagFromIteratorOps[conversionInput[conversionValue], conversionValue]{FromIterator: func(state *rustvec.Owned[conversionInput[conversionValue]], c rustfmt.Context) *rustcindex.IndexVec[uint, conversionValue] {
					return rustcindex.IndexVecFromIteratorWithConfigAndContext(conversionIndexInput(r, state, c), rustcindex.UsizeIdx(), config.indexOps(), config.Profile, c)
				}}, config, c)
				(*owner.Borrow()).Close(c)
				*owner.Borrow() = collection
			case 5:
				UnordBagExtendItems(collection, items, UnordBagExtendIteratorOps[conversionInput[conversionValue], conversionValue]{Extend: func(vector *rustcindex.IndexVec[uint, conversionValue], state *rustvec.Owned[conversionInput[conversionValue]], c rustfmt.Context) {
					vector.ExtendWithContext(conversionIndexInput(r, state, c), c)
				}}, c)
			case 7:
				collection = UnordBagFromItems(items, UnordBagOwnedFromIterator(config), config, c)
				(*owner.Borrow()).Close(c)
				*owner.Borrow() = collection
			case 8:
				UnordBagExtendItems(collection, items, UnordBagOwnedExtendIterator[conversionValue](), c)
			}
			r.event("returned:%d", collection.Len())
			return struct{}{}
		})
	}
	r.event("after_drop")
}
func (r *conversionRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		r.execute(c)
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *conversionRunner) run() {
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
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
	func() {
		defer func() {
			if recover() != nil && r.spec.Mode == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		location := r.locations["outer."+r.spec.Mode]
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func conversionDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadConversionManifest(t *testing.T) conversionManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/items/conversion_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata conversionManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || conversionDigest(data) != want {
				t.Fatalf("sort binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordItemsConversion(t *testing.T) {
	metadata := loadConversionManifest(t)
	data, err := os.ReadFile("testdata/unord_items_conversion.jsonl.gz")
	if err != nil || conversionDigest(data) != metadata.Fixture {
		t.Fatal("sort fixture", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 16<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if conversionDigest(raw) != metadata.Raw {
		t.Fatal("sort raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec conversionCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsConversionChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_CONVERSION="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("sort abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := conversionRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("sort case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 1296 || metadata.Cases != 1296 || aborts != metadata.Aborts {
		t.Fatal("sort coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsConversionChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_CONVERSION")
	if data == "" {
		return
	}
	metadata := loadConversionManifest(t)
	var spec conversionCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := conversionRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
