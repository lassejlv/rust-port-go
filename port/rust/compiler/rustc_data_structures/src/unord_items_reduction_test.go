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
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

type reductionItemsRequest struct {
	Length      int
	NextFail    int  `json:"next_fail"`
	CompareFail int  `json:"compare_fail"`
	ItemFail    int  `json:"item_fail"`
	TotalFail   int  `json:"total_fail"`
	InputFail   bool `json:"input_fail"`
	TraitFail   bool `json:"trait_fail"`
	Resume      bool
}
type reductionItemsCase struct {
	Mode            string
	Family          int
	Request         reductionItemsRequest
	Outcome, Stdout string
}
type reductionItemsManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type reductionItemsValue struct{ number uint64 }
type reductionItemsTotal struct{ number uint64 }
type reductionItemsState[Tag any] struct{ next int }
type reductionPlain struct{}
type reductionBy struct{}
type reductionReduce struct{}
type reductionDirect struct{}
type reductionFold struct{}
type reductionItemsRunner struct {
	spec      reductionItemsCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
	compares  int
}

func (r *reductionItemsRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *reductionItemsRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *reductionItemsRunner) value(number uint64) *rustvec.Owned[reductionItemsValue] {
	return rustvec.NewOwned(reductionItemsValue{number}, rustvec.ValueOps[reductionItemsValue]{Drop: func(v *reductionItemsValue, c rustfmt.Context) {
		r.event("item:%d:%t", v.number, c.Panicking())
		if int(v.number) == r.spec.Request.ItemFail {
			r.raise("item_drop", c)
		}
	}})
}
func (r *reductionItemsRunner) total(number uint64) *rustvec.Owned[reductionItemsTotal] {
	return rustvec.NewOwned(reductionItemsTotal{number}, rustvec.ValueOps[reductionItemsTotal]{Drop: func(v *reductionItemsTotal, c rustfmt.Context) {
		r.event("total:%d:%t", v.number, c.Panicking())
		if int(v.number) == r.spec.Request.TotalFail {
			r.raise("total_drop", c)
		}
	}})
}
func reductionItemsCleanup[T any](current **rustvec.Owned[T], c rustfmt.Context) {
	drop := func(c rustfmt.Context) {
		if *current != nil {
			(*current).DropWithContext(c)
		}
	}
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
func reductionItemsNext[Tag any](r *reductionItemsRunner, owner *rustvec.Owned[reductionItemsState[Tag]], c rustfmt.Context) *rustvec.Owned[reductionItemsValue] {
	family := r.spec.Family
	if family == 4 || family == 5 || family == 6 || family == 7 || family == 10 || family == 11 {
		r.raise("UNEXPECTED next", c)
	}
	n := owner.Borrow().next
	owner.Borrow().next++
	r.event("next:%d:%t", n, c.Panicking())
	if n == r.spec.Request.NextFail {
		r.raise("next", c)
	}
	if n < r.spec.Request.Length {
		return r.value(uint64(n))
	}
	return nil
}
func (r *reductionItemsRunner) ordinal(left, right *rustvec.Owned[reductionItemsValue], max bool, c rustfmt.Context) *rustvec.Owned[reductionItemsValue] {
	return rustvec.ConsumeValue(left, c, func(left *rustvec.Owned[reductionItemsValue]) *rustvec.Owned[reductionItemsValue] {
		return rustvec.ConsumeValue(right, c, func(right *rustvec.Owned[reductionItemsValue]) *rustvec.Owned[reductionItemsValue] {
			label := "ord_min"
			if max {
				label = "ord_max"
			}
			r.event(label)
			n := r.compares
			r.compares++
			a, b := left.Borrow().number, right.Borrow().number
			r.event("cmp:%d:%d:%d:%t", n, a, b, c.Panicking())
			if n == r.spec.Request.CompareFail {
				r.raise("compare", c)
			}
			keepLeft := a%3 <= b%3
			if max {
				keepLeft = a%3 > b%3
			}
			if keepLeft {
				right.DropWithContext(c)
				return left.MoveWithContext(c)
			}
			left.DropWithContext(c)
			return right.MoveWithContext(c)
		})
	})
}
func reductionItemsFoldSelected[Tag, A any](r *reductionItemsRunner, owner *rustvec.Owned[reductionItemsState[Tag]], acc *rustvec.Owned[A], override bool, step func(*rustvec.Owned[A], *rustvec.Owned[reductionItemsValue], rustfmt.Context) *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[A] {
	defer reductionItemsCleanup(&acc, c)
	if override {
		r.event("fold_override")
		for _, n := range []uint64{98, 99} {
			old := acc
			acc = nil
			acc = step(old, r.value(n), c)
		}
		return acc.MoveWithContext(c)
	}
	for {
		value := reductionItemsNext(r, owner, c)
		if value == nil {
			return acc.MoveWithContext(c)
		}
		old := acc
		acc = nil
		acc = step(old, value, c)
	}
}
func (r *reductionItemsRunner) totalStep(acc *rustvec.Owned[reductionItemsTotal], value *rustvec.Owned[reductionItemsValue], sum bool, c rustfmt.Context) *rustvec.Owned[reductionItemsTotal] {
	return rustvec.ConsumeValue(acc, c, func(acc *rustvec.Owned[reductionItemsTotal]) *rustvec.Owned[reductionItemsTotal] {
		return rustvec.ConsumeValue(value, c, func(value *rustvec.Owned[reductionItemsValue]) *rustvec.Owned[reductionItemsTotal] {
			a, b := acc.Borrow().number, value.Borrow().number
			label := "product_step"
			number := a * (b + 1)
			if sum {
				label = "sum_step"
				number = a + b
			}
			r.event("%s:%d:%d:%t", label, a, b, c.Panicking())
			result := r.total(number)
			return rustvec.ConsumeValue(result, c, func(result *rustvec.Owned[reductionItemsTotal]) *rustvec.Owned[reductionItemsTotal] {
				value.DropWithContext(c)
				return result.MoveWithContext(c)
			})
		})
	})
}
func reductionItemsRun[Tag any](r *reductionItemsRunner, c rustfmt.Context) {
	family := r.spec.Family
	owner := rustvec.NewOwned(reductionItemsState[Tag]{}, rustvec.ValueOps[reductionItemsState[Tag]]{Drop: func(state *reductionItemsState[Tag], current rustfmt.Context) {
		r.event("input:%d:%t", state.next, current.Panicking())
		if r.spec.Request.InputFail {
			r.raise("input_drop", current)
		}
	}})
	// The concrete iterator dictionary keeps the selected Next and Count
	// facets, including FoldInput's independent fold. Reduction dispatch is
	// supplied separately because each source method can override it.
	ops := UnordIteratorOps[reductionItemsState[Tag], reductionItemsValue]{
		Next: func(ref rustvec.Ref[reductionItemsState[Tag]]) *rustvec.Owned[reductionItemsValue] {
			if family == 4 || family == 5 || family == 6 || family == 7 || family == 10 || family == 11 {
				r.raise("UNEXPECTED next", ref.Context)
			}
			n := ref.Value.next
			ref.Value.next++
			r.event("next:%d:%t", n, ref.Context.Panicking())
			if n == r.spec.Request.NextFail {
				r.raise("next", ref.Context)
			}
			if n < r.spec.Request.Length {
				return r.value(uint64(n))
			}
			return nil
		},
		SizeHint: func(rustvec.Ref[reductionItemsState[Tag]]) rustvec.Hint { return rustvec.Hint{} },
		Count: func(owner *rustvec.Owned[reductionItemsState[Tag]], current rustfmt.Context) uint {
			step := func(acc *rustvec.Owned[uint], value *rustvec.Owned[reductionItemsValue], current rustfmt.Context) *rustvec.Owned[uint] {
				return rustvec.ConsumeValue(acc, current, func(acc *rustvec.Owned[uint]) *rustvec.Owned[uint] {
					return rustvec.ConsumeValue(value, current, func(*rustvec.Owned[reductionItemsValue]) *rustvec.Owned[uint] { return rustvec.Own(*acc.Borrow() + 1) })
				})
			}
			result := reductionItemsFoldSelected(r, owner, rustvec.Own(uint(0)), family >= 12, step, current)
			return result.Take()
		},
	}
	items := NewUnordItems(owner, ops)
	if family <= 7 || family >= 14 {
		max := family%2 == 0
		selected := func(owner *rustvec.Owned[reductionItemsState[Tag]], current rustfmt.Context) *rustvec.Owned[reductionItemsValue] {
			if family == 6 || family == 7 {
				label := "min_override"
				if max {
					label = "max_override"
				}
				r.event(label)
				return r.value(77)
			}
			step := func(left, right *rustvec.Owned[reductionItemsValue], current rustfmt.Context) *rustvec.Owned[reductionItemsValue] {
				return r.ordinal(left, right, max, current)
			}
			if family == 4 || family == 5 {
				r.event("reduce_override")
				return step(r.value(98), r.value(99), current)
			}
			first := reductionItemsNext(r, owner, current)
			if first == nil {
				return nil
			}
			return reductionItemsFoldSelected(r, owner, first, family >= 14, step, current)
		}
		var result *rustvec.Owned[reductionItemsValue]
		if max {
			result = MaxUnordItems(items, selected, c)
		} else {
			result = MinUnordItems(items, selected, c)
		}
		label := "none"
		if result != nil {
			label = strconv.FormatUint(result.Borrow().number, 10)
		}
		r.event("returned:%s", label)
		if result != nil {
			result.DropWithContext(c)
		}
		return
	}
	sum := family%2 == 0
	selected := func(owner *rustvec.Owned[reductionItemsState[Tag]], current rustfmt.Context) *rustvec.Owned[reductionItemsTotal] {
		direct := family == 10 || family == 11
		if direct {
			label := "product_override"
			if sum {
				label = "sum_override"
			}
			r.event(label)
		}
		trait := func() *rustvec.Owned[reductionItemsTotal] {
			label := "product_trait"
			if sum {
				label = "sum_trait"
			}
			r.event(label)
			if r.spec.Request.TraitFail {
				r.raise(label, current)
			}
			seed := uint64(1)
			if sum {
				seed = 0
			}
			acc := r.total(seed)
			step := func(acc *rustvec.Owned[reductionItemsTotal], value *rustvec.Owned[reductionItemsValue], current rustfmt.Context) *rustvec.Owned[reductionItemsTotal] {
				return r.totalStep(acc, value, sum, current)
			}
			return reductionItemsFoldSelected(r, owner, acc, family >= 12, step, current)
		}
		if !direct {
			return trait()
		}
		// DirectInput's real override passes std::iter::once(Value(77)) to
		// S::sum/product; the once owns its value even if the trait panics.
		once := r.value(77)
		return rustvec.ConsumeValue(once, current, func(once *rustvec.Owned[reductionItemsValue]) *rustvec.Owned[reductionItemsTotal] {
			label := "product_trait"
			if sum {
				label = "sum_trait"
			}
			r.event(label)
			if r.spec.Request.TraitFail {
				r.raise(label, current)
			}
			seed := uint64(1)
			if sum {
				seed = 0
			}
			return r.totalStep(r.total(seed), once, sum, current)
		})
	}
	var result *rustvec.Owned[reductionItemsTotal]
	if sum {
		result = SumUnordItems(items, selected, c)
	} else {
		result = ProductUnordItems(items, selected, c)
	}
	r.event("returned:%d", result.Borrow().number)
	result.DropWithContext(c)
}
func (r *reductionItemsRunner) execute(c rustfmt.Context) {
	switch r.spec.Family {
	case 0, 1, 8, 9:
		reductionItemsRun[reductionPlain](r, c)
	case 2, 3:
		reductionItemsRun[reductionBy](r, c)
	case 4, 5:
		reductionItemsRun[reductionReduce](r, c)
	case 6, 7, 10, 11:
		reductionItemsRun[reductionDirect](r, c)
	default:
		reductionItemsRun[reductionFold](r, c)
	}
}
func (r *reductionItemsRunner) body(c rustfmt.Context) {
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
func (r *reductionItemsRunner) run() {
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
func reductionItemsDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadReductionItemsManifest(t *testing.T) reductionItemsManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/reductions/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata reductionItemsManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || reductionItemsDigest(data) != want {
				t.Fatalf("reductions binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordItemsReduction(t *testing.T) {
	metadata := loadReductionItemsManifest(t)
	data, err := os.ReadFile("testdata/unord_items_reduction.jsonl.gz")
	if err != nil || reductionItemsDigest(data) != metadata.Fixture {
		t.Fatal("reductions fixture", err)
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
	if reductionItemsDigest(raw) != metadata.Raw {
		t.Fatal("reductions raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec reductionItemsCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsReductionChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_BORROWED="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("reductions abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := reductionItemsRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("reductions case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 2112 || metadata.Cases != 2112 || aborts != metadata.Aborts {
		t.Fatal("reductions coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsReductionChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_BORROWED")
	if data == "" {
		return
	}
	metadata := loadReductionItemsManifest(t)
	var spec reductionItemsCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := reductionItemsRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
