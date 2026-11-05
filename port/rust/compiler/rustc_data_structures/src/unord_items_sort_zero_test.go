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
	"strings"
	"testing"
)

type itemZeroSortRequest struct {
	Length             uint
	Action             int
	KeyFail            int  `json:"key_fail"`
	CmpFail            int  `json:"cmp_fail"`
	KeyDropFail        int  `json:"key_drop_fail"`
	InputDropFail      bool `json:"input_drop_fail"`
	BorrowFail         bool `json:"borrow_fail"`
	CollectionDropFail bool `json:"collection_drop_fail"`
	ProjectDropFail    bool `json:"project_drop_fail"`
	TokenFail          bool `json:"token_fail"`
	Resume             bool
}
type itemZeroSortCase struct {
	Mode            string
	Unstable        bool
	Request         itemZeroSortRequest
	Outcome, Stdout string
}
type itemZeroSortManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type itemZeroSortValue struct {
	key int32
	id  uint32
}
type itemZeroSortZero struct{}
type itemZeroSortInput struct{ length uint }
type itemZeroSortCollection struct {
	values *rustcindex.IndexVec[uint, itemZeroSortZero]
}
type itemZeroSortProjection struct{ calls int }
type itemZeroSortRunner struct {
	spec                     itemZeroSortCase
	locations                map[string]rustfmt.SourceLocation
	out                      io.Writer
	keys, compares, keyDrops int
}

func (r *itemZeroSortRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *itemZeroSortRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func itemZeroSortFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *itemZeroSortRunner) inputDrop(input *itemZeroSortInput, c rustfmt.Context) {
	r.event("input_drop:%d:%t", input.length, c.Panicking())
	if r.spec.Request.InputDropFail {
		r.raise("input_drop", c)
	}
}
func (r *itemZeroSortRunner) collectionDrop(collection *itemZeroSortCollection, c rustfmt.Context) {
	defer itemZeroSortFieldCleanup(c, collection.values.CloseWithRuntimeContext)
	r.event("collection_drop:%d:%t", collection.values.Len(), c.Panicking())
	if r.spec.Request.CollectionDropFail {
		r.raise("collection_drop", c)
	}
}
func (r *itemZeroSortRunner) newInput() *UnordItems[itemZeroSortZero] {
	state := rustvec.NewOwned(itemZeroSortInput{length: r.spec.Request.Length}, rustvec.ValueOps[itemZeroSortInput]{Drop: r.inputDrop})
	return NewUnordItems(state, UnordIteratorOps[itemZeroSortInput, itemZeroSortZero]{
		Next: func(rustvec.Ref[itemZeroSortInput]) *rustvec.Owned[itemZeroSortZero] {
			panic("unexpected ZST Input::next")
		},
		SizeHint: func(ref rustvec.Ref[itemZeroSortInput]) rustvec.Hint {
			n := ref.Value.length
			r.event("hint:%d", n)
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(*rustvec.Owned[itemZeroSortInput], rustfmt.Context) uint { panic("unused ZST Input::count") },
	})
}
func (r *itemZeroSortRunner) collector() UnordSortCollector[itemZeroSortInput, itemZeroSortZero, itemZeroSortCollection] {
	return UnordSortCollector[itemZeroSortInput, itemZeroSortZero, itemZeroSortCollection]{
		Collect: func(owner *rustvec.Owned[itemZeroSortInput], c rustfmt.Context) *rustvec.Owned[itemZeroSortCollection] {
			r.event("collect")
			r.event("collection_from")
			length := rustvec.ConsumeValue(owner, c, func(owner *rustvec.Owned[itemZeroSortInput]) uint {
				n := owner.Borrow().length
				r.event("hint:%d", n)
				return n
			})
			raw := rustcindex.IndexRawZeroSized(&itemZeroSortZero{}, length)
			values := rustcindex.IndexVecFromStorage(raw, rustcindex.UsizeIdx(), rustcindex.ValueOps[itemZeroSortZero]{RustZeroSized: true})
			r.event("collection_ready:%d:%d", values.Len(), values.Raw.Cap())
			return rustvec.NewOwned(itemZeroSortCollection{values: values}, rustvec.ValueOps[itemZeroSortCollection]{Drop: r.collectionDrop})
		},
		BorrowMut: func(ref rustvec.Ref[itemZeroSortCollection]) UnordSortableSlice[itemZeroSortZero] {
			n := ref.Value.values.Len()
			r.event("borrow:%d", n)
			if r.spec.Request.BorrowFail {
				r.raise("borrow", ref.Context)
			}
			var zero *itemZeroSortZero
			if n != 0 {
				zero = ref.Value.values.Raw.At(0)
			}
			return UnordSortableSlice[itemZeroSortZero]{Length: n, Zero: zero, Type: rustsort.Type{Size: 0, Align: 1, Freeze: true, Copy: false}}
		},
	}
}
func (r *itemZeroSortRunner) compare(label string, a, b *itemZeroSortValue, c rustfmt.Context) rustsort.Ordering {
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
func (r *itemZeroSortRunner) keyDrop(value *itemZeroSortValue, c rustfmt.Context) {
	n := r.keyDrops
	r.keyDrops++
	r.event("key_drop:%d:%t", value.id, c.Panicking())
	if n == r.spec.Request.KeyDropFail {
		r.raise("key_drop", c)
	}
}
func (r *itemZeroSortRunner) keyOps() UnordStableKeyOps[itemZeroSortZero, itemZeroSortValue] {
	return UnordStableKeyOps[itemZeroSortZero, itemZeroSortValue]{
		Project: func(value *itemZeroSortZero, _ StableHashCtxt, c rustfmt.Context) *rustvec.Owned[itemZeroSortValue] {
			n := r.keys
			r.keys++
			r.event("key:%d:%t", n, c.Panicking())
			if n == r.spec.Request.KeyFail {
				r.raise("key", c)
			}
			return rustvec.NewOwned(itemZeroSortValue{key: int32((n*7 + 3) % 5), id: uint32(n)}, rustvec.ValueOps[itemZeroSortValue]{Drop: r.keyDrop})
		},
		KeyType: rustsort.Type{Size: 8, Align: 4, Freeze: true, Copy: false}, PointerBits: 64,
		Cached: rustsort.CachedKeyOps[itemZeroSortValue]{Drop: r.keyDrop, PartialCompare: func(a, b *itemZeroSortValue, c rustfmt.Context) (rustsort.Ordering, bool) {
			return r.compare("partial", a, b, c), true
		}},
	}
}
func (r *itemZeroSortRunner) tokenDrop(c rustfmt.Context) {
	r.event("token:%t", c.Panicking())
	if r.spec.Request.TokenFail {
		r.raise("token_drop", c)
	}
}
func (r *itemZeroSortRunner) projectionDrop(value *itemZeroSortProjection, c rustfmt.Context) {
	defer itemZeroSortFieldCleanup(c, r.tokenDrop)
	r.event("projection_drop:%d:%t", value.calls, c.Panicking())
	if r.spec.Request.ProjectDropFail {
		r.raise("projection_drop", c)
	}
}
func (r *itemZeroSortRunner) projection() UnordFunction[rustvec.Ref[itemZeroSortZero], rustvec.Ref[itemZeroSortZero]] {
	unused := func(rustvec.Ref[itemZeroSortProjection], *rustvec.Owned[rustvec.Ref[itemZeroSortZero]]) *rustvec.Owned[rustvec.Ref[itemZeroSortZero]] {
		panic("unexpected ZST projection")
	}
	return NewUnordFunction(rustvec.NewOwned(itemZeroSortProjection{}, rustvec.ValueOps[itemZeroSortProjection]{Drop: r.projectionDrop}), UnordFunctionOps[itemZeroSortProjection, rustvec.Ref[itemZeroSortZero], rustvec.Ref[itemZeroSortZero]]{
		Call: unused, CallMut: unused, CallOnce: func(*rustvec.Owned[itemZeroSortProjection], *rustvec.Owned[rustvec.Ref[itemZeroSortZero]], rustfmt.Context) *rustvec.Owned[rustvec.Ref[itemZeroSortZero]] {
			panic("unexpected ZST projection")
		},
	})
}
func (r *itemZeroSortRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		items := r.newInput()
		var result *rustvec.Owned[itemZeroSortCollection]
		if r.spec.Request.Action == 2 {
			result = CollectUnordItemsStableOrdByKey(items, r.projection(), r.collector(), UnordStableCompare[itemZeroSortZero]{CanUseUnstableSort: r.spec.Unstable, Compare: func(a, b *itemZeroSortZero, c rustfmt.Context) rustsort.Ordering { panic("unexpected ZST stable_cmp") }}, c)
		} else {
			result = CollectUnordItemsSorted(items, nil, r.spec.Request.Action == 1, r.collector(), r.keyOps(), c)
		}
		r.event("returned:%d", result.Borrow().values.Len())
		result.DropWithContext(c)
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemZeroSortRunner) run() {
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{rustvec.PanicCapacityOverflowSite: r.locations["capacity overflow"]}, CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
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
func itemZeroSortDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadItemZeroSortManifest(t *testing.T) itemZeroSortManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/item-sort/zero_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata itemZeroSortManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || itemZeroSortDigest(data) != want {
				t.Fatalf("sort binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordItemsSortZero(t *testing.T) {
	metadata := loadItemZeroSortManifest(t)
	data, err := os.ReadFile("testdata/unord_items_sort_zero.jsonl.gz")
	if err != nil || itemZeroSortDigest(data) != metadata.Fixture {
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
	if itemZeroSortDigest(raw) != metadata.Raw {
		t.Fatal("sort raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemZeroSortCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsSortZeroChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_SORT_ZERO="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("sort abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := itemZeroSortRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("sort case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 828 || metadata.Cases != 828 || aborts != metadata.Aborts {
		t.Fatal("sort coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsSortZeroChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_SORT_ZERO")
	if data == "" {
		return
	}
	metadata := loadItemZeroSortManifest(t)
	var spec itemZeroSortCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := itemZeroSortRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
