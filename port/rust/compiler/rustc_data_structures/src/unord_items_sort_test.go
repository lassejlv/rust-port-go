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
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

type itemSortRequest struct {
	Length, Action     int
	KeyFail            int  `json:"key_fail"`
	CmpFail            int  `json:"cmp_fail"`
	KeyDropFail        int  `json:"key_drop_fail"`
	ItemDropFail       int  `json:"item_drop_fail"`
	InputDropFail      bool `json:"input_drop_fail"`
	BorrowFail         bool `json:"borrow_fail"`
	CollectionDropFail bool `json:"collection_drop_fail"`
	ProjectDropFail    bool `json:"project_drop_fail"`
	TokenFail          bool `json:"token_fail"`
	Resume             bool
}
type itemSortCase struct {
	Mode            string
	Unstable        bool
	Request         itemSortRequest
	Outcome, Stdout string
}
type itemSortManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type itemSortValue struct {
	key int32
	id  uint32
}
type itemSortInput struct {
	values *rustcindex.OwnedIterator[itemSortValue]
}
type itemSortCollection struct {
	values *rustcindex.IndexVec[uint, itemSortValue]
}
type itemSortProjection struct{ calls int }
type itemSortRunner struct {
	spec                     itemSortCase
	locations                map[string]rustfmt.SourceLocation
	out                      io.Writer
	keys, compares, keyDrops int
}

func (r *itemSortRunner) event(format string, args ...any) { fmt.Fprintf(r.out, format+"\n", args...) }
func (r *itemSortRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func itemSortFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *itemSortRunner) itemDrop(value *itemSortValue, c rustfmt.Context) {
	r.event("item:%d:%t", value.id, c.Panicking())
	if int(value.id) == r.spec.Request.ItemDropFail {
		r.raise("item_drop", c)
	}
}
func (r *itemSortRunner) inputDrop(input *itemSortInput, c rustfmt.Context) {
	defer itemSortFieldCleanup(c, input.values.CloseWithRuntimeContext)
	r.event("input_drop:%d:%t", input.values.Len(), c.Panicking())
	if r.spec.Request.InputDropFail {
		r.raise("input_drop", c)
	}
}
func itemSortIDs(values *rustcindex.IndexVec[uint, itemSortValue]) string {
	var ids []string
	for i := uint(0); i < values.Len(); i++ {
		ids = append(ids, strconv.FormatUint(uint64(values.Raw.At(i).id), 10))
	}
	return strings.Join(ids, ",")
}
func (r *itemSortRunner) collectionDrop(collection *itemSortCollection, c rustfmt.Context) {
	defer itemSortFieldCleanup(c, collection.values.CloseWithRuntimeContext)
	r.event("collection_drop:%s:%t", itemSortIDs(collection.values), c.Panicking())
	if r.spec.Request.CollectionDropFail {
		r.raise("collection_drop", c)
	}
}
func (r *itemSortRunner) indexOps() rustcindex.ValueOps[itemSortValue] {
	return rustcindex.ValueOps[itemSortValue]{RustSize: 8, DropWithRuntimeContext: r.itemDrop}
}
func (r *itemSortRunner) next(input *itemSortInput, c rustfmt.Context) (itemSortValue, bool) {
	value, ok := input.values.Next()
	label := "none"
	if ok {
		label = strconv.FormatUint(uint64(value.id), 10)
	}
	r.event("next:%s:%t", label, c.Panicking())
	return value, ok
}
func (r *itemSortRunner) newInput() *UnordItems[itemSortValue] {
	values := make([]itemSortValue, r.spec.Request.Length)
	for i := range values {
		values[i] = itemSortValue{key: int32((i*7 + 3) % 5), id: uint32(i)}
	}
	vector := rustcindex.IndexVecFromRaw(values, rustcindex.UsizeIdx(), r.indexOps())
	state := rustvec.NewOwned(itemSortInput{values: vector.IntoIter()}, rustvec.ValueOps[itemSortInput]{Drop: r.inputDrop})
	return NewUnordItems(state, UnordIteratorOps[itemSortInput, itemSortValue]{
		Next: func(ref rustvec.Ref[itemSortInput]) *rustvec.Owned[itemSortValue] {
			value, ok := r.next(ref.Value, ref.Context)
			if !ok {
				return nil
			}
			return rustvec.NewOwned(value, rustvec.ValueOps[itemSortValue]{Drop: r.itemDrop})
		},
		SizeHint: func(ref rustvec.Ref[itemSortInput]) rustvec.Hint {
			n := ref.Value.values.Len()
			r.event("hint:%d", n)
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(*rustvec.Owned[itemSortInput], rustfmt.Context) uint { panic("unused test Input::count") },
	})
}
func (r *itemSortRunner) collector() UnordSortCollector[itemSortInput, itemSortValue, itemSortCollection] {
	return UnordSortCollector[itemSortInput, itemSortValue, itemSortCollection]{
		Collect: func(owner *rustvec.Owned[itemSortInput], c rustfmt.Context) *rustvec.Owned[itemSortCollection] {
			r.event("collect")
			r.event("collection_from")
			r.event("collect")
			input := owner.Take()
			iterator := rustcindex.IndexIntoIterator[itemSortValue]{IntoIterWithContext: func(rustfmt.Context) rustcindex.IndexValueIterator[itemSortValue] {
				return rustcindex.IndexValueIterator[itemSortValue]{
					NextWithContext: func(c rustfmt.Context) (itemSortValue, bool) { return r.next(&input, c) },
					SizeHintWithContext: func(c rustfmt.Context) (uint, uint, bool) {
						n := input.values.Len()
						r.event("hint:%d", n)
						return n, n, true
					},
					CloseWithRuntimeContext: func(c rustfmt.Context) { r.inputDrop(&input, c) },
				}
			}}
			values := rustcindex.IndexVecFromIteratorWithConfigAndContext(iterator, rustcindex.UsizeIdx(), r.indexOps(), rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, c)
			r.event("collection_ready:%d:%d", values.Len(), values.Raw.Cap())
			return rustvec.NewOwned(itemSortCollection{values: values}, rustvec.ValueOps[itemSortCollection]{Drop: r.collectionDrop})
		},
		BorrowMut: func(ref rustvec.Ref[itemSortCollection]) UnordSortableSlice[itemSortValue] {
			r.event("borrow:%d", ref.Value.values.Len())
			if r.spec.Request.BorrowFail {
				r.raise("borrow", ref.Context)
			}
			return UnordSortableSlice[itemSortValue]{Length: ref.Value.values.Len(), Values: ref.Value.values.Raw.AsGoSlice(), Type: rustsort.Type{Size: 8, Align: 4, Freeze: true, Copy: false}}
		},
	}
}
func (r *itemSortRunner) compare(label string, a, b *itemSortValue, c rustfmt.Context) rustsort.Ordering {
	n := r.compares
	r.compares++
	r.event("%s:%d:%d:%t", label, a.id, b.id, c.Panicking())
	if n == r.spec.Request.CmpFail {
		r.raise("compare", c)
	}
	if a.key < b.key {
		return rustsort.LessThan
	}
	if a.key > b.key {
		return rustsort.GreaterThan
	}
	return rustsort.Equal
}
func (r *itemSortRunner) keyDrop(value *itemSortValue, c rustfmt.Context) {
	n := r.keyDrops
	r.keyDrops++
	r.event("key_drop:%d:%t", value.id, c.Panicking())
	if n == r.spec.Request.KeyDropFail {
		r.raise("key_drop", c)
	}
}
func (r *itemSortRunner) keyOps() UnordStableKeyOps[itemSortValue, itemSortValue] {
	return UnordStableKeyOps[itemSortValue, itemSortValue]{
		Project: func(value *itemSortValue, _ StableHashCtxt, c rustfmt.Context) *rustvec.Owned[itemSortValue] {
			n := r.keys
			r.keys++
			r.event("key:%d:%t", value.id, c.Panicking())
			if n == r.spec.Request.KeyFail {
				r.raise("key", c)
			}
			return rustvec.NewOwned(*value, rustvec.ValueOps[itemSortValue]{Drop: r.keyDrop})
		},
		KeyType: rustsort.Type{Size: 8, Align: 4, Freeze: true, Copy: false}, PointerBits: 64,
		Cached: rustsort.CachedKeyOps[itemSortValue]{Drop: r.keyDrop, PartialCompare: func(a, b *itemSortValue, c rustfmt.Context) (rustsort.Ordering, bool) {
			return r.compare("partial", a, b, c), true
		}},
	}
}
func (r *itemSortRunner) tokenDrop(c rustfmt.Context) {
	r.event("token:%t", c.Panicking())
	if r.spec.Request.TokenFail {
		r.raise("token_drop", c)
	}
}
func (r *itemSortRunner) projectionDrop(value *itemSortProjection, c rustfmt.Context) {
	defer itemSortFieldCleanup(c, r.tokenDrop)
	r.event("projection_drop:%d:%t", value.calls, c.Panicking())
	if r.spec.Request.ProjectDropFail {
		r.raise("projection_drop", c)
	}
}
func (r *itemSortRunner) projection() UnordFunction[rustvec.Ref[itemSortValue], rustvec.Ref[itemSortValue]] {
	body := func(ref rustvec.Ref[itemSortProjection], arg *rustvec.Owned[rustvec.Ref[itemSortValue]], label string) *rustvec.Owned[rustvec.Ref[itemSortValue]] {
		n := ref.Value.calls
		ref.Value.calls++
		r.event("%s:%d:%t", label, arg.Borrow().Value.id, ref.Context.Panicking())
		if n == r.spec.Request.KeyFail {
			r.raise("project", ref.Context)
		}
		return rustvec.Own(*arg.Borrow())
	}
	return NewUnordFunction(rustvec.NewOwned(itemSortProjection{}, rustvec.ValueOps[itemSortProjection]{Drop: r.projectionDrop}), UnordFunctionOps[itemSortProjection, rustvec.Ref[itemSortValue], rustvec.Ref[itemSortValue]]{
		Call: func(ref rustvec.Ref[itemSortProjection], arg *rustvec.Owned[rustvec.Ref[itemSortValue]]) *rustvec.Owned[rustvec.Ref[itemSortValue]] {
			return body(ref, arg, "project_shared")
		},
		CallMut: func(ref rustvec.Ref[itemSortProjection], arg *rustvec.Owned[rustvec.Ref[itemSortValue]]) *rustvec.Owned[rustvec.Ref[itemSortValue]] {
			return body(ref, arg, "project_mut")
		},
		CallOnce: func(owner *rustvec.Owned[itemSortProjection], arg *rustvec.Owned[rustvec.Ref[itemSortValue]], c rustfmt.Context) *rustvec.Owned[rustvec.Ref[itemSortValue]] {
			return body(rustvec.Ref[itemSortProjection]{Value: owner.Borrow(), Context: c}, arg, "project_once")
		},
	})
}
func (r *itemSortRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		items := r.newInput()
		var result *rustvec.Owned[itemSortCollection]
		if r.spec.Request.Action == 2 {
			result = CollectUnordItemsStableOrdByKey(items, r.projection(), r.collector(), UnordStableCompare[itemSortValue]{CanUseUnstableSort: r.spec.Unstable, Compare: func(a, b *itemSortValue, c rustfmt.Context) rustsort.Ordering { return r.compare("stable", a, b, c) }}, c)
		} else {
			result = CollectUnordItemsSorted(items, nil, r.spec.Request.Action == 1, r.collector(), r.keyOps(), c)
		}
		r.event("returned:%s", itemSortIDs(result.Borrow().values))
		result.DropWithContext(c)
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemSortRunner) run() {
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
func itemSortDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadItemSortManifest(t *testing.T) itemSortManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/item-sort/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata itemSortManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || itemSortDigest(data) != want {
				t.Fatalf("sort binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordItemsSort(t *testing.T) {
	metadata := loadItemSortManifest(t)
	data, err := os.ReadFile("testdata/unord_items_sort.jsonl.gz")
	if err != nil || itemSortDigest(data) != metadata.Fixture {
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
	if itemSortDigest(raw) != metadata.Raw {
		t.Fatal("sort raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemSortCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsSortChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_SORT="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("sort abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := itemSortRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("sort case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 864 || metadata.Cases != 864 || aborts != metadata.Aborts {
		t.Fatal("sort coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsSortChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_SORT")
	if data == "" {
		return
	}
	metadata := loadItemSortManifest(t)
	var spec itemSortCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := itemSortRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
