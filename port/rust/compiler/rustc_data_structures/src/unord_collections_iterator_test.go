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

type ordinaryCollectionRequest struct {
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
}
type ordinaryCollectionCase struct {
	Mode            string
	Family          int
	Request         ordinaryCollectionRequest
	Outcome, Stdout string
}
type ordinaryCollectionManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type ordinaryCollectionValue struct{ value, id uint64 }
type ordinaryCollectionInput[T any] struct {
	values *rustcindex.OwnedIterator[T]
	calls  int
}
type ordinaryCollectionProducer[T any] struct {
	input *rustvec.Owned[ordinaryCollectionInput[T]]
}
type ordinaryCollectionRunner struct {
	spec      ordinaryCollectionCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
	observe   bool
	hashes    int
}

func (r *ordinaryCollectionRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *ordinaryCollectionRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func ordinaryCollectionFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *ordinaryCollectionRunner) dropValue(v *ordinaryCollectionValue, c rustfmt.Context) {
	r.event("item:%d:%t", v.id, c.Panicking())
	if int(v.id) == r.spec.Request.ItemFail {
		r.raise("item_drop", c)
	}
}
func (r *ordinaryCollectionRunner) keyTraits() rusthashmap.KeyOps[ordinaryCollectionValue] {
	return rusthashmap.KeyOps[ordinaryCollectionValue]{
		Hash: func(v *ordinaryCollectionValue, h rusthashmap.Hasher, c rustfmt.Context) {
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
		Equal: func(a, b *ordinaryCollectionValue, c rustfmt.Context) bool {
			if r.observe {
				r.event("eq:%d:%d:%t", a.id, b.id, c.Panicking())
			}
			return a.value == b.value
		}, Drop: r.dropValue}
}
func ordinaryCollectionV(n int, key bool) ordinaryCollectionValue {
	id := 200
	if key {
		id = 100
	}
	return ordinaryCollectionValue{value: uint64(n % 3), id: uint64(n + id)}
}
func ordinaryCollectionNext[T any](r *ordinaryCollectionRunner, state *ordinaryCollectionInput[T], c rustfmt.Context) (T, bool) {
	n := state.calls
	state.calls++
	r.event("next:%d:%t", n, c.Panicking())
	if n == r.spec.Request.NextFail {
		r.raise("next", c)
	}
	return state.values.Next()
}
func ordinaryCollectionDrop[T any](r *ordinaryCollectionRunner, state *ordinaryCollectionInput[T], c rustfmt.Context) {
	defer ordinaryCollectionFieldCleanup(c, state.values.CloseWithRuntimeContext)
	r.event("input_drop:%d:%d:%t", state.calls, state.values.Len(), c.Panicking())
	if r.spec.Request.InputFail {
		r.raise("input_drop", c)
	}
}
func ordinaryCollectionNested[T any](r *ordinaryCollectionRunner, consume rustvec.BorrowedConsumer[T], value T, c rustfmt.Context) {
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
func ordinaryCollectionNew[T any](r *ordinaryCollectionRunner, values []T, size uint, drop func(*T, rustfmt.Context)) rustvec.IntoIterator[T] {
	vector := rustcindex.IndexVecFromRaw(values, rustcindex.UsizeIdx(), rustcindex.ValueOps[T]{RustSize: size, DropWithRuntimeContext: drop})
	owner := rustvec.NewOwned(ordinaryCollectionInput[T]{values: vector.IntoIter()}, rustvec.ValueOps[ordinaryCollectionInput[T]]{Drop: func(s *ordinaryCollectionInput[T], c rustfmt.Context) { ordinaryCollectionDrop(r, s, c) }})
	ops := rustvec.IteratorOps[ordinaryCollectionInput[T], T]{Next: func(ref rustvec.Ref[ordinaryCollectionInput[T]]) (T, bool) {
		return ordinaryCollectionNext(r, ref.Value, ref.Context)
	}, SizeHint: func(ref rustvec.Ref[ordinaryCollectionInput[T]]) rustvec.Hint {
		n := ref.Value.values.Len()
		r.event("hint:%d", n)
		return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
	}}
	each := func(owner *rustvec.Owned[ordinaryCollectionInput[T]], consume rustvec.BorrowedConsumer[T], label string) {
		c := owner.Context()
		r.event("%s", label)
		if r.spec.Request.CallFail {
			r.raise(label, c)
		}
		if r.spec.Request.Nested {
			value, ok := ordinaryCollectionNext(r, owner.Borrow(), c)
			if ok {
				ordinaryCollectionNested(r, consume, value, c)
			}
		}
		for {
			value, ok := ordinaryCollectionNext(r, owner.Borrow(), c)
			if !ok {
				return
			}
			consume.Call(value, c)
		}
	}
	ops.ForEach = func(owner *rustvec.Owned[ordinaryCollectionInput[T]], consume func(T)) {
		each(owner, rustvec.NewBorrowedConsumer(func(value T, _ rustfmt.Context) { consume(value) }), "for_each")
	}
	ops.FoldUnit = func(owner *rustvec.Owned[ordinaryCollectionInput[T]], consume func(T)) {
		each(owner, rustvec.NewBorrowedConsumer(func(value T, _ rustfmt.Context) { consume(value) }), "fold")
	}
	scoped := rustvec.ScopedIteratorOps[ordinaryCollectionInput[T], T]{ForEachScoped: func(owner *rustvec.Owned[ordinaryCollectionInput[T]], consume rustvec.BorrowedConsumer[T]) {
		each(owner, consume, "for_each")
	}, FoldUnitScoped: func(owner *rustvec.Owned[ordinaryCollectionInput[T]], consume rustvec.BorrowedConsumer[T]) {
		each(owner, consume, "fold")
	}}
	producer := rustvec.NewOwned(ordinaryCollectionProducer[T]{input: owner}, rustvec.ValueOps[ordinaryCollectionProducer[T]]{Drop: func(state *ordinaryCollectionProducer[T], c rustfmt.Context) {
		defer ordinaryCollectionFieldCleanup(c, func(current rustfmt.Context) {
			if state.input != nil {
				state.input.DropWithContext(current)
			}
		})
		r.event("producer_drop:%t:%t", state.input != nil, c.Panicking())
		if r.spec.Request.ProducerFail {
			r.raise("producer_drop", c)
		}
	}})
	return rustvec.NewIntoIteratorFromOwned(producer, rustvec.IntoIteratorOps[ordinaryCollectionProducer[T], T]{IntoIter: func(owner *rustvec.Owned[ordinaryCollectionProducer[T]]) rustvec.Iterator[T] {
		c := owner.Context()
		r.event("into_iter:%t", c.Panicking())
		if r.spec.Request.IntoFail {
			r.raise("into_iter", c)
		}
		input := owner.Borrow().input
		owner.Borrow().input = nil
		return rustvec.NewScopedIteratorFromOwned(input, ops, scoped)
	}})
}

func (r *ordinaryCollectionRunner) execute(c rustfmt.Context) {
	r.observe = false
	request := r.spec.Request
	switch r.spec.Family {
	case 0, 2:
		config := UnordMapConfig[ordinaryCollectionValue, ordinaryCollectionValue]{Layout: rusthashmap.Layout{Size: 32, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: r.keyTraits(), Values: rusthashmap.ValueOps[ordinaryCollectionValue]{Drop: r.dropValue}}
		values := make([]rusthashmap.Pair[ordinaryCollectionValue, ordinaryCollectionValue], request.Length)
		for n := range values {
			values[n] = rusthashmap.Pair[ordinaryCollectionValue, ordinaryCollectionValue]{Key: ordinaryCollectionV(n, true), Value: ordinaryCollectionV(n, false)}
		}
		input := ordinaryCollectionNew(r, values, 32, func(v *rusthashmap.Pair[ordinaryCollectionValue, ordinaryCollectionValue], current rustfmt.Context) {
			defer ordinaryCollectionFieldCleanup(current, func(current rustfmt.Context) { r.dropValue(&v.Value, current) })
			r.dropValue(&v.Key, current)
		})
		collection := NewUnordMap(config, c)
		if r.spec.Family == 2 {
			collection.Insert(ordinaryCollectionValue{value: 0, id: 90}, ordinaryCollectionValue{value: 0, id: 190}, c)
		}
		owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordMap[ordinaryCollectionValue, ordinaryCollectionValue]]{Drop: func(v **UnordMap[ordinaryCollectionValue, ordinaryCollectionValue], current rustfmt.Context) {
			(*v).Close(current)
		}})
		rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[*UnordMap[ordinaryCollectionValue, ordinaryCollectionValue]]) struct{} {
			r.observe = true
			if r.spec.Family == 0 {
				collection = UnordMapFromIteratorScoped(input, config, c)
				(*owner.Borrow()).Close(c)
				*owner.Borrow() = collection
			} else {
				collection.ExtendScoped(input, c)
			}
			r.event("returned:%d", collection.Len())
			return struct{}{}
		})
	case 1, 3:
		config := UnordSetConfig[ordinaryCollectionValue]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: r.keyTraits()}
		values := make([]ordinaryCollectionValue, request.Length)
		for n := range values {
			values[n] = ordinaryCollectionV(n, true)
		}
		input := ordinaryCollectionNew(r, values, 16, r.dropValue)
		collection := NewUnordSet(config, c)
		if r.spec.Family == 3 {
			collection.Insert(ordinaryCollectionValue{value: 0, id: 90}, c)
		}
		owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordSet[ordinaryCollectionValue]]{Drop: func(v **UnordSet[ordinaryCollectionValue], current rustfmt.Context) { (*v).Close(current) }})
		rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[*UnordSet[ordinaryCollectionValue]]) struct{} {
			r.observe = true
			if r.spec.Family == 1 {
				collection = UnordSetFromIteratorScoped(input, config, c)
				(*owner.Borrow()).Close(c)
				*owner.Borrow() = collection
			} else {
				collection.ExtendScoped(input, c)
			}
			r.event("returned:%d", collection.Len())
			return struct{}{}
		})
	}
	r.event("after_drop")
}
func (r *ordinaryCollectionRunner) body(c rustfmt.Context) {
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
func (r *ordinaryCollectionRunner) run() {
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
func ordinaryCollectionDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadOrdinaryCollectionManifest(t *testing.T) ordinaryCollectionManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/collections-iterator/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata ordinaryCollectionManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || ordinaryCollectionDigest(data) != want {
				t.Fatalf("ordinary iterator binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordCollectionsIterator(t *testing.T) {
	metadata := loadOrdinaryCollectionManifest(t)
	data, err := os.ReadFile("testdata/unord_collections_iterator.jsonl.gz")
	if err != nil || ordinaryCollectionDigest(data) != metadata.Fixture {
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
	if ordinaryCollectionDigest(raw) != metadata.Raw {
		t.Fatal("ordinary iterator raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec ordinaryCollectionCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordCollectionsIteratorChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_CONVERSION="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("ordinary iterator abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := ordinaryCollectionRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("ordinary iterator case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 720 || metadata.Cases != 720 || aborts != metadata.Aborts {
		t.Fatal("ordinary iterator coverage", count, aborts, scanner.Err())
	}
}
func TestUnordCollectionsIteratorChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_CONVERSION")
	if data == "" {
		return
	}
	metadata := loadOrdinaryCollectionManifest(t)
	var spec ordinaryCollectionCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := ordinaryCollectionRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
