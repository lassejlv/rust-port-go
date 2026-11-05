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

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type itemsCollectionsCase struct {
	Mode                         string
	Family, Action, Length, Stop int
	NextFail                     int  `json:"next_fail"`
	CallFail                     int  `json:"call_fail"`
	ItemFail                     int  `json:"item_fail"`
	EnvFail                      bool `json:"env_fail"`
	TokenFail                    bool `json:"token_fail"`
	IteratorFail                 bool `json:"iterator_fail"`
	Resume                       bool
	Outcome, Stdout              string
}
type itemsCollectionsState struct{ next int }
type itemsCollectionsFunctionState struct{ calls int }
type itemsCollectionsManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type itemsCollectionsRunner struct {
	spec      itemsCollectionsCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *itemsCollectionsRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *itemsCollectionsRunner) raise(value string, c rustfmt.Context) {
	if r.spec.Resume {
		c.ResumeUnwind(value)
	}
	location := r.locations[value]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: value, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *itemsCollectionsRunner) dropItem(value *uint64, c rustfmt.Context) {
	r.event("item:%d:%t", *value, c.Panicking())
	if int(*value) == r.spec.ItemFail {
		r.raise("item_drop", c)
	}
}
func (r *itemsCollectionsRunner) dropIterator(state *itemsCollectionsState, c rustfmt.Context) {
	r.event("iterator:%d:%t", state.next, c.Panicking())
	if r.spec.IteratorFail {
		r.raise("iterator_drop", c)
	}
}
func (r *itemsCollectionsRunner) dropToken(c rustfmt.Context) {
	r.event("token:%t", c.Panicking())
	if r.spec.TokenFail {
		r.raise("token_drop", c)
	}
}
func (r *itemsCollectionsRunner) dropFunction(state *itemsCollectionsFunctionState, c rustfmt.Context) {
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
func newItemsCollectionFunction[T any](r *itemsCollectionsRunner, key func(*T) uint64) UnordFunction[T, bool] {
	body := func(ref rustvec.Ref[itemsCollectionsFunctionState], argument *rustvec.Owned[T], which string) *rustvec.Owned[bool] {
		call := ref.Value.calls
		ref.Value.calls++
		id := key(argument.Borrow())
		r.event("%s:%d:%d:%t", which, call, id, ref.Context.Panicking())
		if call == r.spec.CallFail {
			r.raise("call", ref.Context)
		}
		return rustvec.Own(int(id) != r.spec.Stop)
	}
	return NewUnordFunction(rustvec.NewOwned(itemsCollectionsFunctionState{}, rustvec.ValueOps[itemsCollectionsFunctionState]{Drop: r.dropFunction}), UnordFunctionOps[itemsCollectionsFunctionState, T, bool]{
		Call: func(ref rustvec.Ref[itemsCollectionsFunctionState], arg *rustvec.Owned[T]) *rustvec.Owned[bool] {
			return body(ref, arg, "shared")
		},
		CallMut: func(ref rustvec.Ref[itemsCollectionsFunctionState], arg *rustvec.Owned[T]) *rustvec.Owned[bool] {
			return body(ref, arg, "mut")
		},
		CallOnce: func(state *rustvec.Owned[itemsCollectionsFunctionState], arg *rustvec.Owned[T], c rustfmt.Context) *rustvec.Owned[bool] {
			return body(rustvec.Ref[itemsCollectionsFunctionState]{Value: state.Borrow(), Context: c}, arg, "once")
		},
	})
}
func executeItemsCollection[T any](r *itemsCollectionsRunner, items *UnordItems[T], key func(*T) uint64, c rustfmt.Context) {
	switch r.spec.Action {
	case 0, 1:
		f := newItemsCollectionFunction(r, key)
		var result bool
		if r.spec.Action == 1 {
			result = items.Any(f, c)
		} else {
			result = items.All(f, c)
		}
		r.event("returned:%t", result)
	case 2:
		r.event("returned:%d", items.Count(c))
	case 3:
		result, ok := items.GetOnly(c)
		if ok {
			r.event("returned:%d", key(result.Borrow()))
			result.DropWithContext(c)
		} else {
			r.event("returned:none")
		}
	default:
		panic("source collection action")
	}
}
func (r *itemsCollectionsRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		keys := rusthashmap.KeyOps[uint64]{Hash: func(value *uint64, h rusthashmap.Hasher, _ rustfmt.Context) { rusthashmap.HashU64(h, *value) }, Equal: func(a, b *uint64, _ rustfmt.Context) bool { return *a == *b }, Drop: r.dropItem}
		switch r.spec.Family {
		case 0, 3, 6:
			config := UnordMapConfig[uint64, uint64]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys, Values: rusthashmap.ValueOps[uint64]{Drop: r.dropItem}}
			collection := NewUnordMap(config, c)
			for n := 0; n < r.spec.Length; n++ {
				collection.Insert(uint64(n), uint64(n+100), c)
			}
			owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordMap[uint64, uint64]]{Drop: func(value **UnordMap[uint64, uint64], c rustfmt.Context) { (*value).Close(c) }})
			rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordMap[uint64, uint64]]) struct{} {
				switch r.spec.Family {
				case 0:
					executeItemsCollection(r, collection.IntoItems(c), func(pair *rusthashmap.Pair[uint64, uint64]) uint64 { return pair.Key }, c)
				case 3:
					executeItemsCollection(r, collection.Items(c).General(c), func(pair *rusthashmap.BorrowedPair[uint64, uint64]) uint64 { return *pair.Key }, c)
				case 6:
					executeItemsCollection(r, collection.Keys(c), func(key **uint64) uint64 { return **key }, c)
				}
				return struct{}{}
			})
		case 1, 4:
			collection := NewUnordSet(UnordSetConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: keys}, c)
			for n := 0; n < r.spec.Length; n++ {
				collection.Insert(uint64(n), c)
			}
			owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordSet[uint64]]{Drop: func(value **UnordSet[uint64], c rustfmt.Context) { (*value).Close(c) }})
			rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordSet[uint64]]) struct{} {
				if r.spec.Family == 1 {
					executeItemsCollection(r, collection.IntoItems(c), func(key *uint64) uint64 { return *key }, c)
				} else {
					executeItemsCollection(r, collection.Items(c).General(c), func(key **uint64) uint64 { return **key }, c)
				}
				return struct{}{}
			})
		case 2, 5:
			collection := NewUnordBag(UnordBagConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, Values: UnordBagValueOps[uint64]{Drop: r.dropItem}}, c)
			for n := 0; n < r.spec.Length; n++ {
				collection.Push(uint64(n), c)
			}
			owner := rustvec.NewOwned(collection, rustvec.ValueOps[*UnordBag[uint64]]{Drop: func(value **UnordBag[uint64], c rustfmt.Context) { (*value).Close(c) }})
			rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordBag[uint64]]) struct{} {
				if r.spec.Family == 2 {
					executeItemsCollection(r, collection.IntoItems(c).General(c), func(key *uint64) uint64 { return *key }, c)
				} else {
					executeItemsCollection(r, collection.Items(c).General(c), func(key **uint64) uint64 { return **key }, c)
				}
				return struct{}{}
			})
		default:
			panic("source collection family")
		}
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemsCollectionsRunner) run() {
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
func itemsCollectionsDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func loadItemsCollectionsManifest(t *testing.T) itemsCollectionsManifest {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/items/collections_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m itemsCollectionsManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || itemsCollectionsDigest(b) != h {
			t.Fatalf("Items boolean source binding %s: %v", p, e)
		}
	}
	return m
}
func TestOriginalUnordItemsCollections(t *testing.T) {
	m := loadItemsCollectionsManifest(t)
	b, e := os.ReadFile("testdata/unord_items_collections.jsonl.gz")
	if e != nil || itemsCollectionsDigest(b) != m.Fixture {
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
	if itemsCollectionsDigest(raw) != m.Raw {
		t.Fatal("Items raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemsCollectionsCase
		if e = json.Unmarshal(scanner.Bytes(), &spec); e != nil {
			t.Fatal(e)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			encoded, _ := json.Marshal(spec)
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsCollectionsChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_COLLECTIONS="+string(encoded))
			out, e := command.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("Items boolean case%d abort: %v %s", count, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			runner := itemsCollectionsRunner{spec: spec, locations: m.Locations, out: &out}
			runner.run()
			got = out.String()
		}
		if got != spec.Stdout {
			t.Fatalf("Items boolean case%d %#v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 2016 || m.Cases != 2016 || aborts != 374 || m.Aborts != 374 {
		t.Fatal("Items coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsCollectionsChild(t *testing.T) {
	value := os.Getenv("FERRUGO_UNORD_ITEMS_COLLECTIONS")
	if value == "" {
		return
	}
	m := loadItemsCollectionsManifest(t)
	var spec itemsCollectionsCase
	if e := json.Unmarshal([]byte(value), &spec); e != nil {
		t.Fatal(e)
	}
	runner := itemsCollectionsRunner{spec: spec, locations: m.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
