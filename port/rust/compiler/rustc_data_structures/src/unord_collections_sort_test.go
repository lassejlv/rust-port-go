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
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

type collectionSortRequest struct {
	Length, Action     int
	Cache              bool
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
type collectionSortCase struct {
	Mode            string
	Unstable        bool
	Request         collectionSortRequest
	Outcome, Stdout string
}
type collectionSortManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type collectionSortValue struct {
	key int32
	id  uint32
}
type collectionSortRunner struct {
	spec                     collectionSortCase
	locations                map[string]rustfmt.SourceLocation
	out                      io.Writer
	keys, compares, keyDrops int
}

func (r *collectionSortRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *collectionSortRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func collectionSortFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *collectionSortRunner) itemDrop(value *collectionSortValue, c rustfmt.Context) {
	r.event("item:%d:%t", value.id, c.Panicking())
	if int(value.id) == r.spec.Request.ItemDropFail {
		r.raise("item_drop", c)
	}
}
func (r *collectionSortRunner) valueDrop(value *uint32, c rustfmt.Context) {
	r.event("value:%d:%t", *value, c.Panicking())
	if int(*value) == r.spec.Request.ItemDropFail {
		r.raise("value_drop", c)
	}
}
func (r *collectionSortRunner) keyTraits() rusthashmap.KeyOps[collectionSortValue] {
	return rusthashmap.KeyOps[collectionSortValue]{Hash: func(v *collectionSortValue, h rusthashmap.Hasher, _ rustfmt.Context) { rusthashmap.HashU32(h, v.id) }, Equal: func(a, b *collectionSortValue, _ rustfmt.Context) bool { return a.id == b.id }, Drop: r.itemDrop}
}
func collectionSortDisplay[T any](r *collectionSortRunner, v *rustcindex.IndexVec[uint, T], id func(*T) string, c rustfmt.Context) {
	r.event("returned:%d:%d", v.Len(), v.Raw.Cap())
	ids := []string{}
	for n := uint(0); n < v.Len(); n++ {
		ids = append(ids, id(v.Raw.At(n)))
	}
	r.event("ids:%s", strings.Join(ids, ","))
	v.CloseWithRuntimeContext(c)
}
func (r *collectionSortRunner) compare(label string, a, b *collectionSortValue, c rustfmt.Context) rustsort.Ordering {
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
func (r *collectionSortRunner) keyDrop(value *collectionSortValue, c rustfmt.Context) {
	n := r.keyDrops
	r.keyDrops++
	r.event("key_drop:%d:%t", value.id, c.Panicking())
	if n == r.spec.Request.KeyDropFail {
		r.raise("key_drop", c)
	}
}
func (r *collectionSortRunner) keyOps() UnordStableKeyOps[collectionSortValue, collectionSortValue] {
	return UnordStableKeyOps[collectionSortValue, collectionSortValue]{
		Project: func(value *collectionSortValue, _ StableHashCtxt, c rustfmt.Context) *rustvec.Owned[collectionSortValue] {
			n := r.keys
			r.keys++
			r.event("key:%d:%t", value.id, c.Panicking())
			if n == r.spec.Request.KeyFail {
				r.raise("key", c)
			}
			return rustvec.NewOwned(*value, rustvec.ValueOps[collectionSortValue]{Drop: r.keyDrop})
		},
		KeyType: rustsort.Type{Size: 8, Align: 4, Freeze: true, Copy: false}, PointerBits: 64,
		Cached: rustsort.CachedKeyOps[collectionSortValue]{Drop: r.keyDrop, PartialCompare: func(a, b *collectionSortValue, c rustfmt.Context) (rustsort.Ordering, bool) {
			return r.compare("partial", a, b, c), true
		}},
	}
}
func (r *collectionSortRunner) execute(c rustfmt.Context) {
	request := r.spec.Request
	stable := UnordStableCompare[collectionSortValue]{CanUseUnstableSort: r.spec.Unstable, Compare: func(a, b *collectionSortValue, c rustfmt.Context) rustsort.Ordering {
		return r.compare("stable", a, b, c)
	}}
	if request.Action < 4 {
		collection := NewUnordMap(UnordMapConfig[collectionSortValue, uint32]{Layout: rusthashmap.Layout{Size: 12, Align: 4}, Profile: rusthashmap.DefaultProfile(), Keys: r.keyTraits(), Values: rusthashmap.ValueOps[uint32]{Drop: r.valueDrop}}, c)
		for n := 0; n < request.Length; n++ {
			collection.Insert(collectionSortValue{key: int32((n*7 + 3) % 5), id: uint32(n)}, uint32(n), c)
		}
		owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordMap[collectionSortValue, uint32]]{Drop: func(v **UnordMap[collectionSortValue, uint32], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordMap[collectionSortValue, uint32]]) struct{} {
			if request.Action < 2 {
				var result *rustcindex.IndexVec[uint, rusthashmap.BorrowedPair[collectionSortValue, uint32]]
				if request.Action == 0 {
					result = UnordMapToSorted(collection, nil, request.Cache, r.keyOps(), c)
				} else {
					result = UnordMapToSortedStableOrd(collection, stable, c)
				}
				collectionSortDisplay(r, result, func(v *rusthashmap.BorrowedPair[collectionSortValue, uint32]) string {
					return fmt.Sprintf("%d=%d", v.Key.id, *v.Value)
				}, c)
			} else {
				var result *rustcindex.IndexVec[uint, rusthashmap.Pair[collectionSortValue, uint32]]
				typ := rustsort.Type{Size: 12, Align: 4, Freeze: true}
				if request.Action == 2 {
					result = UnordMapIntoSorted(collection, nil, request.Cache, typ, r.keyOps(), c)
				} else {
					result = UnordMapIntoSortedStableOrd(collection, typ, stable, c)
				}
				collectionSortDisplay(r, result, func(v *rusthashmap.Pair[collectionSortValue, uint32]) string {
					return fmt.Sprintf("%d=%d", v.Key.id, v.Value)
				}, c)
			}
			return struct{}{}
		})
	} else {
		collection := NewUnordSet(UnordSetConfig[collectionSortValue]{Layout: rusthashmap.Layout{Size: 8, Align: 4}, Profile: rusthashmap.DefaultProfile(), Values: r.keyTraits()}, c)
		for n := 0; n < request.Length; n++ {
			collection.Insert(collectionSortValue{key: int32((n*7 + 3) % 5), id: uint32(n)}, c)
		}
		owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordSet[collectionSortValue]]{Drop: func(v **UnordSet[collectionSortValue], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordSet[collectionSortValue]]) struct{} {
			if request.Action < 6 {
				var result *rustcindex.IndexVec[uint, *collectionSortValue]
				if request.Action == 4 {
					result = UnordSetToSorted(collection, nil, request.Cache, r.keyOps(), c)
				} else {
					result = UnordSetToSortedStableOrd(collection, stable, c)
				}
				collectionSortDisplay(r, result, func(v **collectionSortValue) string { return strconv.FormatUint(uint64((*v).id), 10) }, c)
			} else {
				var result *rustcindex.IndexVec[uint, collectionSortValue]
				typ := rustsort.Type{Size: 8, Align: 4, Freeze: true}
				if request.Action == 6 {
					result = UnordSetIntoSorted(collection, nil, request.Cache, typ, r.keyOps(), c)
				} else {
					result = UnordSetIntoSortedStableOrd(collection, typ, stable, c)
				}
				collectionSortDisplay(r, result, func(v *collectionSortValue) string { return strconv.FormatUint(uint64(v.id), 10) }, c)
			}
			return struct{}{}
		})
	}
}
func (r *collectionSortRunner) body(c rustfmt.Context) {
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
func (r *collectionSortRunner) run() {
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
func collectionSortDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadCollectionSortManifest(t *testing.T) collectionSortManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/item-sort/collections_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata collectionSortManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || collectionSortDigest(data) != want {
				t.Fatalf("sort binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordCollectionsSort(t *testing.T) {
	metadata := loadCollectionSortManifest(t)
	data, err := os.ReadFile("testdata/unord_collections_sort.jsonl.gz")
	if err != nil || collectionSortDigest(data) != metadata.Fixture {
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
	if collectionSortDigest(raw) != metadata.Raw {
		t.Fatal("sort raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec collectionSortCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordCollectionsSortChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_COLLECTIONS_SORT="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("sort abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := collectionSortRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("sort case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 3072 || metadata.Cases != 3072 || aborts != metadata.Aborts {
		t.Fatal("sort coverage", count, aborts, scanner.Err())
	}
}
func TestUnordCollectionsSortChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_COLLECTIONS_SORT")
	if data == "" {
		return
	}
	metadata := loadCollectionSortManifest(t)
	var spec collectionSortCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := collectionSortRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
