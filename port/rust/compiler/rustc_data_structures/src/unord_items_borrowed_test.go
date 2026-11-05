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
	"strconv"
	"strings"
	"testing"
)

type borrowedItemsRequest struct {
	Length    int
	CloneFail int `json:"clone_fail"`
	DropFail  int `json:"drop_fail"`
	Resume    bool
}
type borrowedItemsCase struct {
	Mode            string
	Family          int
	Request         borrowedItemsRequest
	Outcome, Stdout string
}
type borrowedItemsManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type borrowedItemsValue struct{ value, id uint64 }
type borrowedItemsState[S any] struct{ input *rustvec.Owned[S] }
type borrowedItemsRunner struct {
	spec      borrowedItemsCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
	clones    int
}

func (r *borrowedItemsRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *borrowedItemsRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func borrowedItemsFieldCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
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
func (r *borrowedItemsRunner) drop(v *borrowedItemsValue, c rustfmt.Context) {
	r.event("item:%d:%t", v.id, c.Panicking())
	if int(v.id) == r.spec.Request.DropFail {
		r.raise("item_drop", c)
	}
}
func (r *borrowedItemsRunner) clone(v *borrowedItemsValue, c rustfmt.Context) *rustvec.Owned[borrowedItemsValue] {
	n := r.clones
	r.clones++
	r.event("clone:%d:%t", v.id, c.Panicking())
	if n == r.spec.Request.CloneFail {
		r.raise("clone", c)
	}
	return rustvec.NewOwned(borrowedItemsValue{value: v.value, id: v.id + 1000}, rustvec.ValueOps[borrowedItemsValue]{Drop: r.drop})
}

type borrowedItemsFoldUint[S any] func(*rustvec.Owned[S], uint, func(uint, *borrowedItemsValue, rustfmt.Context) uint, rustfmt.Context) uint

func borrowedItemsConstructor[S any](r *borrowedItemsRunner, input UnordIteratorOps[S, *borrowedItemsValue], fold borrowedItemsFoldUint[S], copied bool) UnordUnaryIteratorConstructor[S, *borrowedItemsValue, borrowedItemsState[S], borrowedItemsValue] {
	return UnordUnaryIteratorConstructor[S, *borrowedItemsValue, borrowedItemsState[S], borrowedItemsValue]{
		Construct: func(owner *rustvec.Owned[S], c rustfmt.Context) *rustvec.Owned[borrowedItemsState[S]] {
			return rustvec.NewOwned(borrowedItemsState[S]{input: owner.MoveWithContext(rustfmt.Context{})}, rustvec.ValueOps[borrowedItemsState[S]]{Drop: func(s *borrowedItemsState[S], c rustfmt.Context) { s.input.DropWithContext(c) }})
		},
		Result: UnordIteratorOps[borrowedItemsState[S], borrowedItemsValue]{
			Next: func(ref rustvec.Ref[borrowedItemsState[S]]) *rustvec.Owned[borrowedItemsValue] {
				value := input.Next(rustvec.Ref[S]{Value: ref.Value.input.Borrow(), Context: ref.Context})
				if value == nil {
					return nil
				}
				pointer := value.Take()
				if copied {
					return rustvec.Own(*pointer)
				}
				return r.clone(pointer, ref.Context)
			},
			SizeHint: func(ref rustvec.Ref[borrowedItemsState[S]]) rustvec.Hint {
				return input.SizeHint(rustvec.Ref[S]{Value: ref.Value.input.Borrow(), Context: ref.Context})
			},
			// Cloned::count selects I::fold and clones/drops each yielded item.
			// Copied::count independently selects I::count.
			Count: func(owner *rustvec.Owned[borrowedItemsState[S]], c rustfmt.Context) uint {
				state := owner.Take()
				return rustvec.ConsumeValue(state.input, c, func(inputOwner *rustvec.Owned[S]) uint {
					if copied {
						return input.Count(inputOwner, c)
					}
					return fold(inputOwner, 0, func(acc uint, pointer *borrowedItemsValue, current rustfmt.Context) uint {
						return rustvec.ConsumeValue(r.clone(pointer, current), current, func(*rustvec.Owned[borrowedItemsValue]) uint { return acc + 1 })
					}, c)
				})
			},
		},
	}
}
func borrowedItemsExecute[S any](r *borrowedItemsRunner, items *UnordItems[*borrowedItemsValue], fold borrowedItemsFoldUint[S], c rustfmt.Context) {
	input := items.owner.(*unordIteratorCell[S, *borrowedItemsValue]).ops
	copied := (r.spec.Family >= 4 && r.spec.Family <= 7) || r.spec.Family == 9
	if fold == nil {
		// These three actual concrete borrowed collection iterators use their
		// selected standard fold body; no custom I::fold is replaced here.
		fold = func(owner *rustvec.Owned[S], acc uint, f func(uint, *borrowedItemsValue, rustfmt.Context) uint, current rustfmt.Context) uint {
			for {
				value := input.Next(rustvec.Ref[S]{Value: owner.Borrow(), Context: current})
				if value == nil {
					return acc
				}
				acc = f(acc, value.Take(), current)
			}
		}
	}
	selected := borrowedItemsConstructor(r, input, fold, copied)
	var result *UnordItems[borrowedItemsValue]
	if copied {
		result = CopiedBorrowedUnordItems(items, selected, c)
	} else {
		result = ClonedBorrowedUnordItems(items, selected, c)
	}
	if r.spec.Family == 1 || r.spec.Family == 5 || r.spec.Family >= 8 {
		r.event("returned:%d", result.Count(c))
	} else {
		value, _ := result.GetOnly(c)
		label := "none"
		if value != nil {
			label = strconv.FormatUint(value.Borrow().id, 10)
		}
		r.event("returned:%s", label)
		if value != nil {
			value.DropWithContext(c)
		}
	}
}
func (r *borrowedItemsRunner) execute(c rustfmt.Context) {
	copied := (r.spec.Family >= 4 && r.spec.Family <= 7) || r.spec.Family == 9
	size := uint(16)
	var drop func(*borrowedItemsValue, rustfmt.Context) = r.drop
	if copied {
		size = 8
		drop = nil
	}
	key := rusthashmap.KeyOps[borrowedItemsValue]{Hash: func(v *borrowedItemsValue, h rusthashmap.Hasher, _ rustfmt.Context) { rusthashmap.HashU64(h, v.value) }, Equal: func(a, b *borrowedItemsValue, _ rustfmt.Context) bool { return a.value == b.value }, Drop: drop}
	switch r.spec.Family {
	case 0, 1, 4, 5:
		config := UnordBagConfig[borrowedItemsValue]{Layout: rusthashmap.Layout{Size: size, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, Values: UnordBagValueOps[borrowedItemsValue]{Drop: drop}}
		bag := NewUnordBag(config, c)
		for n := 0; n < r.spec.Request.Length; n++ {
			bag.Push(borrowedItemsValue{value: uint64(n), id: uint64(n)}, c)
		}
		owner := rustvec.NewOwned(bag, rustvec.ValueOps[*UnordBag[borrowedItemsValue]]{Drop: func(v **UnordBag[borrowedItemsValue], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordBag[borrowedItemsValue]]) struct{} {
			borrowedItemsExecute[*rustcindex.RefIterator[borrowedItemsValue]](r, bag.Items(c).General(c), nil, c)
			return struct{}{}
		})
	case 2, 6:
		set := NewUnordSet(UnordSetConfig[borrowedItemsValue]{Layout: rusthashmap.Layout{Size: size, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: key}, c)
		for n := 0; n < r.spec.Request.Length; n++ {
			set.Insert(borrowedItemsValue{value: uint64(n), id: uint64(n)}, c)
		}
		owner := rustvec.NewOwned(set, rustvec.ValueOps[*UnordSet[borrowedItemsValue]]{Drop: func(v **UnordSet[borrowedItemsValue], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordSet[borrowedItemsValue]]) struct{} {
			borrowedItemsExecute[*rusthashmap.SetIter[borrowedItemsValue]](r, set.Items(c).General(c), nil, c)
			return struct{}{}
		})
	case 3, 7:
		m := NewUnordMap(UnordMapConfig[borrowedItemsValue, uint64]{Layout: rusthashmap.Layout{Size: size + 8, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: key}, c)
		for n := 0; n < r.spec.Request.Length; n++ {
			m.Insert(borrowedItemsValue{value: uint64(n), id: uint64(n)}, uint64(n), c)
		}
		owner := rustvec.NewOwned(m, rustvec.ValueOps[*UnordMap[borrowedItemsValue, uint64]]{Drop: func(v **UnordMap[borrowedItemsValue, uint64], c rustfmt.Context) { (*v).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordMap[borrowedItemsValue, uint64]]) struct{} {
			borrowedItemsExecute[*rusthashmap.Iter[borrowedItemsValue, uint64]](r, m.Keys(c), nil, c)
			return struct{}{}
		})
	case 8, 9:
		value := rustvec.NewOwned(borrowedItemsValue{value: 99, id: 99}, rustvec.ValueOps[borrowedItemsValue]{Drop: drop})
		rustvec.ConsumeValue(value, c, func(owner *rustvec.Owned[borrowedItemsValue]) struct{} {
			input := UnordIteratorOps[*borrowedItemsValue, *borrowedItemsValue]{
				Next: func(ref rustvec.Ref[*borrowedItemsValue]) *rustvec.Owned[*borrowedItemsValue] {
					r.raise("UNEXPECTED next", ref.Context)
					return nil
				},
				SizeHint: func(rustvec.Ref[*borrowedItemsValue]) rustvec.Hint { return rustvec.Hint{} },
				Count:    func(*rustvec.Owned[*borrowedItemsValue], rustfmt.Context) uint { r.event("count_override"); return 777 },
			}
			fold := func(owner *rustvec.Owned[*borrowedItemsValue], acc uint, f func(uint, *borrowedItemsValue, rustfmt.Context) uint, current rustfmt.Context) uint {
				r.event("fold_override")
				pointer := *owner.Borrow()
				acc = f(acc, pointer, current)
				return f(acc, pointer, current)
			}
			borrowedItemsExecute[*borrowedItemsValue](r, NewUnordItems(rustvec.Own(owner.Borrow()), input), fold, c)
			return struct{}{}
		})
	}
}
func (r *borrowedItemsRunner) body(c rustfmt.Context) {
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
func (r *borrowedItemsRunner) run() {
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
func borrowedItemsDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadBorrowedItemsManifest(t *testing.T) borrowedItemsManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/borrowed-items/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata borrowedItemsManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || borrowedItemsDigest(data) != want {
				t.Fatalf("borrowed-items binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordItemsBorrowed(t *testing.T) {
	metadata := loadBorrowedItemsManifest(t)
	data, err := os.ReadFile("testdata/unord_items_borrowed.jsonl.gz")
	if err != nil || borrowedItemsDigest(data) != metadata.Fixture {
		t.Fatal("borrowed-items fixture", err)
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
	if borrowedItemsDigest(raw) != metadata.Raw {
		t.Fatal("borrowed-items raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec borrowedItemsCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsBorrowedChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_BORROWED="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("borrowed-items abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := borrowedItemsRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("borrowed-items case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 720 || metadata.Cases != 720 || aborts != metadata.Aborts {
		t.Fatal("borrowed-items coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsBorrowedChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_BORROWED")
	if data == "" {
		return
	}
	metadata := loadBorrowedItemsManifest(t)
	var spec borrowedItemsCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := borrowedItemsRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
