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

type itemsCoreCase struct {
	Mode                    string
	Action, Length, Pattern int
	NextFail                int  `json:"next_fail"`
	ItemFail                int  `json:"item_fail"`
	IteratorFail            bool `json:"iterator_fail"`
	Resume                  bool
	Outcome, Stdout         string
}
type itemsCoreState struct{ next int }
type itemsCoreManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type itemsCoreRunner struct {
	spec      itemsCoreCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *itemsCoreRunner) event(format string, args ...any) { fmt.Fprintf(r.out, format+"\n", args...) }
func (r *itemsCoreRunner) raise(value string, c rustfmt.Context) {
	if r.spec.Resume {
		c.ResumeUnwind(value)
	}
	location := r.locations[value]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: value, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *itemsCoreRunner) dropItem(value *uint64, c rustfmt.Context) {
	r.event("item:%d:%t", *value, c.Panicking())
	if int(*value) == r.spec.ItemFail {
		r.raise("item_drop", c)
	}
}
func (r *itemsCoreRunner) dropIterator(state *itemsCoreState, c rustfmt.Context) {
	r.event("iterator:%d:%t", state.next, c.Panicking())
	if r.spec.IteratorFail && (r.spec.Action != 4 || state.next == 99) {
		r.raise("iterator_drop", c)
	}
}
func (r *itemsCoreRunner) newIterator() *UnordItems[uint64] {
	owner := rustvec.NewOwned(itemsCoreState{}, rustvec.ValueOps[itemsCoreState]{Drop: r.dropIterator})
	return NewUnordItems(owner, UnordIteratorOps[itemsCoreState, uint64]{Next: func(ref rustvec.Ref[itemsCoreState]) *rustvec.Owned[uint64] {
		index := ref.Value.next
		ref.Value.next++
		r.event("next:%d:%t", index, ref.Context.Panicking())
		if index == r.spec.NextFail {
			r.raise("next", ref.Context)
		}
		if r.spec.Pattern == 1 {
			if index == 1 {
				return rustvec.NewOwned(uint64(7), rustvec.ValueOps[uint64]{Drop: r.dropItem})
			}
			return nil
		}
		if index < r.spec.Length {
			return rustvec.NewOwned(uint64(index), rustvec.ValueOps[uint64]{Drop: r.dropItem})
		}
		return nil
	}, SizeHint: func(ref rustvec.Ref[itemsCoreState]) rustvec.Hint { r.event("hint"); return rustvec.Hint{} }, Count: func(owner *rustvec.Owned[itemsCoreState], c rustfmt.Context) uint {
		r.event("count:%d", owner.Borrow().next)
		return uint(1000 + r.spec.Length)
	}, Clone: func(ref rustvec.Ref[itemsCoreState]) *rustvec.Owned[itemsCoreState] {
		r.event("clone:%d:%t", ref.Value.next, ref.Context.Panicking())
		if r.spec.Action == 4 && r.spec.NextFail == 0 {
			r.raise("clone", ref.Context)
		}
		return rustvec.NewOwned(*ref.Value, rustvec.ValueOps[itemsCoreState]{Drop: r.dropIterator})
	}, CloneItem: true})
}
func (r *itemsCoreRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		items := r.newIterator()
		switch r.spec.Action {
		case 0:
			result, ok := items.GetOnly(c)
			if ok {
				r.event("returned:%d", *result.Borrow())
				result.DropWithContext(c)
			} else {
				r.event("returned:none")
			}
		case 1:
			r.event("returned:%d", items.Count(c))
		case 2:
			cloned := items.Clone(c)
			r.event("returned:%d", cloned.Count(c))
			items.Close(c)
		case 3:
			items.Close(c)
			empty := EmptyUnordItems[uint64](true)
			cloned := empty.Clone(c)
			r.event("empty:%d:%d", empty.Count(c), cloned.Count(c))
		case 4:
			destination := r.newIterator()
			destination.owner.(*unordIteratorCell[itemsCoreState, uint64]).state.Borrow().next = 99
			own := func(items *UnordItems[uint64]) *rustvec.Owned[*UnordItems[uint64]] {
				return rustvec.NewOwned(items, rustvec.ValueOps[*UnordItems[uint64]]{Drop: func(items **UnordItems[uint64], c rustfmt.Context) { (*items).Close(c) }})
			}
			rustvec.ConsumeValue(own(items), c, func(*rustvec.Owned[*UnordItems[uint64]]) struct{} {
				return rustvec.ConsumeValue(own(destination), c, func(*rustvec.Owned[*UnordItems[uint64]]) struct{} {
					destination.CloneFrom(items, c)
					r.event("returned:%d", destination.Count(c))
					return struct{}{}
				})
			})
		default:
			panic("source Items action")
		}
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemsCoreRunner) run() {
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
func itemsCoreDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func loadItemsCoreManifest(t *testing.T) itemsCoreManifest {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/items/core_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m itemsCoreManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || itemsCoreDigest(b) != h {
			t.Fatalf("Items source binding %s: %v", p, e)
		}
	}
	return m
}
func TestOriginalUnordItemsCore(t *testing.T) {
	m := loadItemsCoreManifest(t)
	b, e := os.ReadFile("testdata/unord_items_core.jsonl.gz")
	if e != nil || itemsCoreDigest(b) != m.Fixture {
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
	if itemsCoreDigest(raw) != m.Raw {
		t.Fatal("Items raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemsCoreCase
		if e = json.Unmarshal(scanner.Bytes(), &spec); e != nil {
			t.Fatal(e)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			encoded, _ := json.Marshal(spec)
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsCoreChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_CORE="+string(encoded))
			out, e := command.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("Items case%d abort: %v %s", count, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			runner := itemsCoreRunner{spec: spec, locations: m.Locations, out: &out}
			runner.run()
			got = out.String()
		}
		if got != spec.Stdout {
			t.Fatalf("Items case%d %#v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 126 || m.Cases != 126 || aborts != 40 || m.Aborts != 40 {
		t.Fatal("Items coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsCoreChild(t *testing.T) {
	value := os.Getenv("FERRUGO_UNORD_ITEMS_CORE")
	if value == "" {
		return
	}
	m := loadItemsCoreManifest(t)
	var spec itemsCoreCase
	if e := json.Unmarshal([]byte(value), &spec); e != nil {
		t.Fatal(e)
	}
	runner := itemsCoreRunner{spec: spec, locations: m.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
