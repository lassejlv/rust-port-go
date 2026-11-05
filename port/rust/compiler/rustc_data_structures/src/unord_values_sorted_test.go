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
	"unsafe"
)

type valuesSortedRequest struct {
	Length, Action     int
	Cache              bool
	ConsumerFail       int `json:"consumer_fail"`
	Skip               int
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
type valuesSortedCase struct {
	Mode            string
	Unstable        bool
	Request         valuesSortedRequest
	Outcome, Stdout string
}
type valuesSortedManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type valuesSortedValue struct {
	key int32
	id  uint32
}
type valuesSortedRunner struct {
	spec                     valuesSortedCase
	locations                map[string]rustfmt.SourceLocation
	out                      io.Writer
	keys, compares, keyDrops int
}

func (r *valuesSortedRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *valuesSortedRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func valuesSortedFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *valuesSortedRunner) itemDrop(value *valuesSortedValue, c rustfmt.Context) {
	r.event("item:%d:%t", value.id, c.Panicking())
	if int(value.id) == r.spec.Request.ItemDropFail {
		r.raise("item_drop", c)
	}
}
func (r *valuesSortedRunner) valueDrop(value *uint32, c rustfmt.Context) {
	r.event("value:%d:%t", *value, c.Panicking())
	if int(*value) == r.spec.Request.ItemDropFail {
		r.raise("value_drop", c)
	}
}
func (r *valuesSortedRunner) keyTraits() rusthashmap.KeyOps[valuesSortedValue] {
	return rusthashmap.KeyOps[valuesSortedValue]{Hash: func(v *valuesSortedValue, h rusthashmap.Hasher, _ rustfmt.Context) { rusthashmap.HashU32(h, v.id) }, Equal: func(a, b *valuesSortedValue, _ rustfmt.Context) bool { return a.id == b.id }, Drop: r.itemDrop}
}
func (r *valuesSortedRunner) compare(label string, a, b *valuesSortedValue, c rustfmt.Context) rustsort.Ordering {
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
func (r *valuesSortedRunner) keyDrop(value *valuesSortedValue, c rustfmt.Context) {
	n := r.keyDrops
	r.keyDrops++
	r.event("key_drop:%d:%t", value.id, c.Panicking())
	if n == r.spec.Request.KeyDropFail {
		r.raise("key_drop", c)
	}
}
func (r *valuesSortedRunner) keyOps() UnordStableKeyOps[valuesSortedValue, valuesSortedValue] {
	return UnordStableKeyOps[valuesSortedValue, valuesSortedValue]{
		Project: func(value *valuesSortedValue, _ StableHashCtxt, c rustfmt.Context) *rustvec.Owned[valuesSortedValue] {
			n := r.keys
			r.keys++
			r.event("key:%d:%t", value.id, c.Panicking())
			if n == r.spec.Request.KeyFail {
				r.raise("key", c)
			}
			return rustvec.NewOwned(*value, rustvec.ValueOps[valuesSortedValue]{Drop: r.keyDrop})
		},
		KeyType: rustsort.Type{Size: 8, Align: 4, Freeze: true, Copy: false}, PointerBits: 64,
		Cached: rustsort.CachedKeyOps[valuesSortedValue]{Drop: r.keyDrop, PartialCompare: func(a, b *valuesSortedValue, c rustfmt.Context) (rustsort.Ordering, bool) {
			return r.compare("partial", a, b, c), true
		}},
	}
}
func (r *valuesSortedRunner) display(values []*uint32, original unsafe.Pointer) {
	r.event("returned:%d:%d", len(values), cap(values))
	ids := []string{}
	for _, value := range values {
		ids = append(ids, strconv.FormatUint(uint64(*value), 10))
	}
	r.event("ids:%s", strings.Join(ids, ","))
	r.event("reuse:%t", original != nil && original == unsafe.Pointer(unsafe.SliceData(values)))
}
func (r *valuesSortedRunner) execute(c rustfmt.Context) {
	request := r.spec.Request
	collection := NewUnordMap(UnordMapConfig[valuesSortedValue, uint32]{Layout: rusthashmap.Layout{Size: 12, Align: 4}, Profile: rusthashmap.DefaultProfile(), Keys: r.keyTraits(), Values: rusthashmap.ValueOps[uint32]{Drop: r.valueDrop}}, c)
	for n := 0; n < request.Length; n++ {
		collection.Insert(valuesSortedValue{key: int32((n*7 + 3) % 5), id: uint32(n)}, uint32(n), c)
	}
	owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordMap[valuesSortedValue, uint32]]{Drop: func(v **UnordMap[valuesSortedValue, uint32], c rustfmt.Context) { (*v).Close(c) }})
	rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordMap[valuesSortedValue, uint32]]) struct{} {
		var iter rustvec.Iterator[*uint32]
		var original unsafe.Pointer
		if request.Action == 1 || request.Action == 2 {
			iter = UnordMapValuesSorted(collection, nil, request.Cache, r.keyOps(), c)
		} else {
			vector := UnordMapToSorted(collection, nil, request.Cache, r.keyOps(), c)
			if vector.Len() != 0 {
				original = unsafe.Pointer(unsafe.SliceData(vector.Raw.AsGoSlice()))
			}
			iter = unordValuesSortedIterator(vector, c)
		}
		iterOwner := rustvec.NewOwned(iter, rustvec.ValueOps[rustvec.Iterator[*uint32]]{Drop: func(it *rustvec.Iterator[*uint32], c rustfmt.Context) { it.CloseWithContext(c) }})
		rustvec.ConsumeValue(iterOwner, c, func(*rustvec.Owned[rustvec.Iterator[*uint32]]) struct{} {
			for n := 0; n < request.Skip; n++ {
				value, ok := iter.Next(c)
				if ok {
					r.event("step:%d", *value)
				} else {
					r.event("step:none")
				}
			}
			hint := iter.SizeHint(c)
			r.event("hint:%d:%d", hint.Lower, hint.Upper)
			collect := func(it rustvec.Iterator[*uint32]) []*uint32 {
				config := rustvec.DefaultConfig(8)
				config.Context = c
				return rustvec.Collect(rustvec.FromIterator(it), config, rustvec.ElementOps[*uint32]{})
			}
			calls := 0
			consume := func(value *uint32) {
				r.event("consume:%d:%t", *value, c.Panicking())
				n := calls
				calls++
				if n == request.ConsumerFail {
					r.raise("consume", c)
				}
			}
			switch request.Action {
			case 0:
				r.display(collect(iter), original)
			case 1:
				r.display(collect(BorrowUnordSortedValues(iter)), original)
				hint = iter.SizeHint(c)
				r.event("after_hint:%d:%d", hint.Lower, hint.Upper)
			case 2:
				iter.ForEachWithContext(c, consume)
				r.event("consumed:%d", calls)
			case 3:
				message := ""
				func() {
					defer func() {
						if p := recover(); p != nil {
							message = fmt.Sprint(p)
						}
					}()
					BorrowUnordSortedValues(iter).ForEachWithContext(c, consume)
				}()
				r.event("inner_caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
				hint = iter.SizeHint(c)
				r.event("after_hint:%d:%d", hint.Lower, hint.Upper)
				r.display(collect(iter), original)
			}
			return struct{}{}
		})
		return struct{}{}
	})
}
func (r *valuesSortedRunner) body(c rustfmt.Context) {
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
func (r *valuesSortedRunner) run() {
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
func valuesSortedDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadValuesSortedManifest(t *testing.T) valuesSortedManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/item-sort/values_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata valuesSortedManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || valuesSortedDigest(data) != want {
				t.Fatalf("sort binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordValuesSorted(t *testing.T) {
	metadata := loadValuesSortedManifest(t)
	data, err := os.ReadFile("testdata/unord_values_sorted.jsonl.gz")
	if err != nil || valuesSortedDigest(data) != metadata.Fixture {
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
	if valuesSortedDigest(raw) != metadata.Raw {
		t.Fatal("sort raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec valuesSortedCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordValuesSortedChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_VALUES_SORTED="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("sort abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := valuesSortedRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("sort case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 2112 || metadata.Cases != 2112 || aborts != metadata.Aborts {
		t.Fatal("sort coverage", count, aborts, scanner.Err())
	}
}
func TestUnordValuesSortedChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_VALUES_SORTED")
	if data == "" {
		return
	}
	metadata := loadValuesSortedManifest(t)
	var spec valuesSortedCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := valuesSortedRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
