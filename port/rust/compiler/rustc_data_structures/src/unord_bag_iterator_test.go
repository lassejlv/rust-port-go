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

type bagIteratorRequest struct {
	Nested       bool
	Length       int
	NextFail     int  `json:"next_fail"`
	CallFail     bool `json:"call_fail"`
	ItemFail     int  `json:"item_fail"`
	InputFail    bool `json:"input_fail"`
	HashFail     int  `json:"hash_fail"`
	Resume       bool
	ProducerFail bool `json:"producer_fail"`
	IntoFail     bool `json:"into_fail"`
	Forget       bool
	Initial      bool
}
type bagIteratorCase struct {
	Mode            string
	Family          int
	Request         bagIteratorRequest
	Outcome, Stdout string
}
type bagIteratorAllocation struct {
	Family, Length  int
	Initial, Reused bool
	Capacity        uint
	RustSize        uint `json:"rust_size"`
	RustAlign       uint `json:"rust_align"`
}
type bagIteratorManifest struct {
	Allocations   []bagIteratorAllocation
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type bagIteratorValue struct{ value, id uint64 }
type bagIteratorInput[T any] struct {
	values *rustcindex.OwnedIterator[T]
	calls  int
}
type bagIteratorProducer[I any] struct{ input *rustvec.Owned[I] }
type bagIteratorRunner struct {
	spec      bagIteratorCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
	observe   bool
	hashes    int
	capacity  uint
	reused    bool
}

func (r *bagIteratorRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *bagIteratorRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func bagIteratorFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *bagIteratorRunner) dropValue(v *bagIteratorValue, c rustfmt.Context) {
	r.event("item:%d:%t", v.id, c.Panicking())
	if int(v.id) == r.spec.Request.ItemFail {
		r.raise("item_drop", c)
	}
}
func bagIteratorV(n int, key bool) bagIteratorValue {
	id := 200
	if key {
		id = 100
	}
	return bagIteratorValue{value: uint64(n % 3), id: uint64(n + id)}
}
func bagIteratorNext[T any](r *bagIteratorRunner, state *bagIteratorInput[T], c rustfmt.Context) (T, bool) {
	n := state.calls
	state.calls++
	r.event("next:%d:%t", n, c.Panicking())
	if n == r.spec.Request.NextFail {
		r.raise("next", c)
	}
	return state.values.Next()
}
func bagIteratorDrop[T any](r *bagIteratorRunner, state *bagIteratorInput[T], c rustfmt.Context) {
	defer bagIteratorFieldCleanup(c, state.values.CloseWithRuntimeContext)
	r.event("input_drop:%d:%d:%t", state.calls, state.values.Len(), c.Panicking())
	if r.spec.Request.InputFail {
		r.raise("input_drop", c)
	}
}
func bagIteratorSource[I any](r *bagIteratorRunner, input *rustvec.Owned[I], dropLabel string) *rustvec.Owned[bagIteratorProducer[I]] {
	return rustvec.NewOwned(bagIteratorProducer[I]{input: input}, rustvec.ValueOps[bagIteratorProducer[I]]{Drop: func(state *bagIteratorProducer[I], c rustfmt.Context) {
		defer bagIteratorFieldCleanup(c, func(current rustfmt.Context) {
			if state.input != nil {
				state.input.DropWithContext(current)
			}
		})
		r.event("producer_drop:%t:%t", state.input != nil, c.Panicking())
		if r.spec.Request.ProducerFail {
			r.raise(dropLabel, c)
		}
	}})
}
func bagIteratorIntoIter[I any](r *bagIteratorRunner, source *rustvec.Owned[bagIteratorProducer[I]], intoLabel string, c rustfmt.Context) *rustvec.Owned[I] {
	return rustvec.ConsumeValue(source, c, func(source *rustvec.Owned[bagIteratorProducer[I]]) *rustvec.Owned[I] {
		r.event("into_iter:%t", c.Panicking())
		if r.spec.Request.IntoFail {
			r.raise(intoLabel, c)
		}
		input := source.Borrow().input
		source.Borrow().input = nil
		if r.spec.Request.Forget {
			source.Take()
		}
		return input.MoveWithContext(c)
	})
}
func (r *bagIteratorRunner) execute(c rustfmt.Context) {
	values := make([]bagIteratorValue, r.spec.Request.Length)
	for n := range values {
		values[n] = bagIteratorV(n, true)
	}
	raw := rustcindex.IndexVecFromRaw(values, rustcindex.UsizeIdx(), rustcindex.ValueOps[bagIteratorValue]{RustSize: 16, DropWithRuntimeContext: r.dropValue})
	config := UnordBagConfig[bagIteratorValue]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, Values: UnordBagValueOps[bagIteratorValue]{Drop: r.dropValue}}
	var sourcePointer *bagIteratorValue
	if len(values) > 0 {
		sourcePointer = raw.Raw.At(0)
	}
	bag := NewUnordBag(config, c)
	if r.spec.Request.Initial {
		bag.Push(bagIteratorValue{value: 0, id: 90}, c)
	}
	owner := rustvec.NewOwned(bag, rustvec.ValueOps[*UnordBag[bagIteratorValue]]{Drop: func(value **UnordBag[bagIteratorValue], current rustfmt.Context) { (*value).Close(current) }})
	rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordBag[bagIteratorValue]]) struct{} {
		if r.spec.Family == 0 {
			input := rustvec.NewOwned(bagIteratorInput[bagIteratorValue]{values: raw.IntoIter()}, rustvec.ValueOps[bagIteratorInput[bagIteratorValue]]{Drop: func(state *bagIteratorInput[bagIteratorValue], current rustfmt.Context) {
				bagIteratorDrop(r, state, current)
			}})
			source := bagIteratorSource(r, input, "producer_drop")
			selected := UnordBagExtendIteratorOps[bagIteratorProducer[bagIteratorInput[bagIteratorValue]], bagIteratorValue]{Extend: func(vector *rustcindex.IndexVec[uint, bagIteratorValue], source *rustvec.Owned[bagIteratorProducer[bagIteratorInput[bagIteratorValue]]], current rustfmt.Context) {
				vector.ExtendWithContext(rustcindex.IndexIntoIterator[bagIteratorValue]{IntoIterWithContext: func(current rustfmt.Context) rustcindex.IndexValueIterator[bagIteratorValue] {
					input := bagIteratorIntoIter(r, source, "into_iter", current)
					state := input.Take()
					return rustcindex.IndexValueIterator[bagIteratorValue]{NextWithContext: func(current rustfmt.Context) (bagIteratorValue, bool) { return bagIteratorNext(r, &state, current) }, SizeHintWithContext: func(rustfmt.Context) (uint, uint, bool) {
						n := state.values.Len()
						r.event("hint:%d", n)
						return n, n, true
					}, CloseWithRuntimeContext: func(current rustfmt.Context) { bagIteratorDrop(r, &state, current) }}
				}}, current)
			}}
			UnordBagExtendIntoIterator(bag, source, selected, c)
		} else {
			input := rustvec.NewOwned(raw.IntoIter(), rustvec.ValueOps[*rustcindex.OwnedIterator[bagIteratorValue]]{Drop: func(value **rustcindex.OwnedIterator[bagIteratorValue], current rustfmt.Context) {
				(*value).CloseWithRuntimeContext(current)
			}})
			source := bagIteratorSource(r, input, "raw_producer_drop")
			selected := UnordBagExtendIteratorOps[bagIteratorProducer[*rustcindex.OwnedIterator[bagIteratorValue]], bagIteratorValue]{Extend: func(vector *rustcindex.IndexVec[uint, bagIteratorValue], source *rustvec.Owned[bagIteratorProducer[*rustcindex.OwnedIterator[bagIteratorValue]]], current rustfmt.Context) {
				input := bagIteratorIntoIter(r, source, "raw_into_iter", current)
				vector.ExtendWithContext(input.Take().IntoIterator(), current)
			}}
			UnordBagExtendIntoIterator(bag, source, selected, c)
		}
		r.capacity = bag.inner.Raw.Cap()
		r.reused = sourcePointer != nil && bag.Len() > 0 && bag.inner.Raw.At(0) == sourcePointer
		r.event("returned:%d", bag.Len())
		return struct{}{}
	})
	r.event("after_drop")
}
func (r *bagIteratorRunner) body(c rustfmt.Context) {
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
func (r *bagIteratorRunner) run() {
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
func bagIteratorDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadBagIteratorManifest(t *testing.T) bagIteratorManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/bag-iterator/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata bagIteratorManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || bagIteratorDigest(data) != want {
				t.Fatalf("ordinary iterator binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordBagIterator(t *testing.T) {
	metadata := loadBagIteratorManifest(t)
	data, err := os.ReadFile("testdata/unord_bag_iterator.jsonl.gz")
	if err != nil || bagIteratorDigest(data) != metadata.Fixture {
		t.Fatal("ordinary iterator fixture", err)
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
	if bagIteratorDigest(raw) != metadata.Raw {
		t.Fatal("ordinary iterator raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec bagIteratorCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordBagIteratorChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_CONVERSION="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("ordinary iterator abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := bagIteratorRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("ordinary iterator case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 768 || metadata.Cases != 768 || aborts != metadata.Aborts {
		t.Fatal("ordinary iterator coverage", count, aborts, scanner.Err())
	}
	if len(metadata.Allocations) != 20 {
		t.Fatal("actual allocation coverage", len(metadata.Allocations))
	}
	for _, allocation := range metadata.Allocations {
		if allocation.RustSize != 16 || allocation.RustAlign != 8 {
			t.Fatal("actual source value layout", allocation)
		}
		spec := bagIteratorCase{Mode: "normal", Family: allocation.Family, Request: bagIteratorRequest{Length: allocation.Length, Initial: allocation.Initial, NextFail: -1, HashFail: -1, ItemFail: -1}}
		runner := bagIteratorRunner{spec: spec, locations: metadata.Locations, out: io.Discard}
		runner.run()
		if runner.capacity != allocation.Capacity || runner.reused != allocation.Reused {
			t.Fatalf("actual Vec extension capacity/identity %+v got %d/%t", allocation, runner.capacity, runner.reused)
		}
	}

}
func TestUnordBagIteratorChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_CONVERSION")
	if data == "" {
		return
	}
	metadata := loadBagIteratorManifest(t)
	var spec bagIteratorCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := bagIteratorRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
