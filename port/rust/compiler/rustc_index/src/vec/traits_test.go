// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type addressValue struct {
	number   int
	metadata []byte
}

func TestBorrowedElementTraitPlaces(t *testing.T) {
	idx := index.UsizeIdx()
	ops := index.ValueOps[addressValue]{}
	source := index.IndexVecFromRaw([]addressValue{{number: 2}}, idx, ops)
	target := index.IndexVecFromRaw([]addressValue{{number: 1}}, idx, ops)
	sourceCell, targetCell := source.At(0), target.At(0)
	var events []string
	ops.CloneRef = func(cell *addressValue) addressValue {
		events = append(events, fmt.Sprintf("clone_source:%t", cell == sourceCell))
		return addressValue{number: cell.number + 10}
	}
	ops.Drop = func(cell *addressValue) {
		if cell.number == 1 {
			events = append(events, fmt.Sprintf("drop_target:%t", cell == targetCell))
		}
	}
	source.Ops = ops
	target.Ops = ops
	source.AsSlice().CloneInto(target)
	if targetCell.number != 12 || !reflect.DeepEqual(events, []string{"clone_source:true", "drop_target:true"}) {
		t.Fatalf("clone_into state=%v events=%v", targetCell.number, events)
	}
	*targetCell = addressValue{number: 1}
	events = nil
	ops.Drop = func(cell *addressValue) {
		if cell.number == 1 {
			events = append(events, fmt.Sprintf("drop_target:%t", cell == targetCell))
			panic("drop")
		}
	}
	source.Ops = ops
	target.Ops = ops
	mustPanic(t, func() { source.AsSlice().CloneInto(target) })
	if targetCell.number != 12 || !reflect.DeepEqual(events, []string{"clone_source:true", "drop_target:true"}) {
		t.Fatalf("DropAndReplace state=%v events=%v", targetCell.number, events)
	}
	source.Ops.Drop = nil
	target.Ops.Drop = nil
	source.Close()
	target.Close()
}

func TestBorrowedEqualityDebugSearchHashAndZSTViews(t *testing.T) {
	idx := index.UsizeIdx()
	values := []addressValue{{number: 10}, {number: 20}, {number: 30}}
	v := index.IndexVecFromRaw(values, idx, index.ValueOps[addressValue]{})
	var events []string
	v.Ops.EqualRef = func(a, b *addressValue) bool { events = append(events, "eq"); return a.number == b.number }
	v.Ops.NotEqualRef = func(a, b *addressValue) bool {
		events = append(events, fmt.Sprintf("ne_same:%t", a == b))
		return a.number != b.number
	}
	if !v.Equal(v) || !reflect.DeepEqual(events, []string{"ne_same:true", "ne_same:true", "ne_same:true"}) {
		t.Fatalf("slice ne dispatch=%v", events)
	}
	events = nil
	v.Ops.DebugRef = func(cell *addressValue) string {
		good := false
		for i := uint(0); i < v.Len(); i++ {
			good = good || cell == v.At(i)
		}
		events = append(events, fmt.Sprintf("debug_cell:%t", good))
		return fmt.Sprint(cell.number)
	}
	if got := v.Debug(); got != "[10, 20, 30]" || !reflect.DeepEqual(events, []string{"debug_cell:true", "debug_cell:true", "debug_cell:true"}) {
		t.Fatalf("debug=%s events=%v", got, events)
	}
	query := addressValue{number: 20}
	events = nil
	v.Ops.CompareRef = func(a, b *addressValue) int {
		good := false
		for i := uint(0); i < v.Len(); i++ {
			good = good || a == v.At(i)
		}
		events = append(events, fmt.Sprintf("compare:%t:%t", good, b == &query))
		if a.number < b.number {
			return -1
		}
		if a.number > b.number {
			return 1
		}
		return 0
	}
	if key, ok := v.BinarySearchRef(&query); !ok || key != 1 {
		t.Fatalf("search=(%d,%t)", key, ok)
	}
	for _, event := range events {
		if event != "compare:true:true" {
			t.Fatalf("borrowed Ord=%v", events)
		}
	}
	events = nil
	v.Hash(func(raw index.IndexRaw[addressValue]) {
		index.HashIndexRawRef(raw, &events, func(log *[]string, n uint) { *log = append(*log, fmt.Sprintf("length:%d", n)) }, func(log *[]string, cell *addressValue) {
			good := false
			for i := uint(0); i < v.Len(); i++ {
				good = good || cell == v.At(i)
			}
			*log = append(*log, fmt.Sprintf("hash_cell:%t", good))
		}, nil)
	})
	if !reflect.DeepEqual(events, []string{"length:3", "hash_cell:true", "hash_cell:true", "hash_cell:true"}) {
		t.Fatalf("borrowed hash=%v", events)
	}
	token := 7
	zst := index.IndexVecFromElemN(addressValue{metadata: []byte{byte(token)}}, 2, idx, index.ValueOps[addressValue]{RustZeroSized: true})
	cell := zst.At(0)
	view := zst.AsSlice()
	iter := view.Iter()
	iterCell, _ := iter.Next()
	rawRange := zst.Raw.Slice(1, 2)
	if view.At(0) != cell || iterCell != cell || rawRange.At(0) != cell {
		t.Fatal("ZST borrowed views copied the zero cell")
	}
	zst.Close()
	v.Close()
}

func TestOwnedFactoriesAndRangeDestruction(t *testing.T) {
	var events []string
	idx := index.NewIdx(func(v uint) uint { return v }, func(v uint) uint { events = append(events, fmt.Sprintf("index:%d", v)); return v })
	v := index.IndexVecFromRaw([]int{0}, idx, index.ValueOps[int]{})
	factory := func() index.IndexFactory[int] {
		return index.IndexFactory[int]{Call: func() int { events = append(events, "fill"); return 9 }, CloseWithContext: func(ctx index.IndexDropContext) { events = append(events, fmt.Sprintf("close:%t", ctx.Unwinding)) }}
	}
	v.EnsureContainsElemOwned(0, factory())
	if !reflect.DeepEqual(events, []string{"index:0", "index:0", "close:false"}) {
		t.Fatalf("unused factory order=%v", events)
	}
	events = nil
	v.EnsureContainsElemOwned(1, factory())
	if !reflect.DeepEqual(events, []string{"index:1", "fill", "close:false", "index:1"}) {
		t.Fatalf("growth factory order=%v", events)
	}
	events = nil
	bounds := index.IndexRangeBounds{StartBound: func() index.IndexBound {
		events = append(events, "start")
		return index.IndexBound{Kind: index.IndexIncluded, Value: 0}
	}, EndBound: func() index.IndexBound {
		events = append(events, "end")
		return index.IndexBound{Kind: index.IndexExcluded, Value: 1}
	}, Close: func() { events = append(events, "bounds_drop"); panic("bounds drop") }}
	mustPanic(t, func() { v.Drain(bounds) })
	if v.Len() != 2 || !reflect.DeepEqual(v.Raw.AsGoSlice(), []int{0, 9}) || !reflect.DeepEqual(events, []string{"start", "end", "bounds_drop"}) {
		t.Fatalf("bounds Drop changed source vector: values=%v events=%v", v.Raw.AsGoSlice(), events)
	}
	events = nil
	copyBounds := index.IntoSliceIdx[string, index.TypedSliceIndexResult]{Convert: func(string) index.TypedSliceIndexResult {
		events = append(events, "convert")
		return index.TypedSliceIndexResult{Kind: index.SliceIndexRange, Start: 0, End: 1}
	}}
	index.IndexSliceCopyWithinWith(v.AsMutSlice(), "source", copyBounds, func(r index.TypedSliceIndexResult) index.IndexRangeBounds {
		return index.IndexRangeBounds{StartBound: func() index.IndexBound {
			events = append(events, "start")
			return index.IndexBound{Kind: index.IndexIncluded, Value: r.Start}
		}, EndBound: func() index.IndexBound {
			events = append(events, "end")
			return index.IndexBound{Kind: index.IndexExcluded, Value: r.End}
		}}
	}, 1)
	if !reflect.DeepEqual(events, []string{"convert", "index:1", "start", "end"}) || !reflect.DeepEqual(v.Raw.AsGoSlice(), []int{0, 0}) {
		t.Fatalf("custom copy order=%v values=%v", events, v.Raw.AsGoSlice())
	}
	v.Close()
}

func TestOwnedFactoryPanicReturnPlaces(t *testing.T) {
	var events []string
	ops := index.ValueOps[int]{DropWithContext: func(v *int, ctx index.IndexDropContext) {
		events = append(events, fmt.Sprintf("drop:%d:%t", *v, ctx.Unwinding))
	}}
	mustPanic(t, func() {
		index.IndexVecFromFnNOwned(index.IndexFnMut[uint, int]{Call: func(i uint) int { events = append(events, fmt.Sprintf("fill:%d", i)); return int(i) }, Close: func() { events = append(events, "factory_drop"); panic("factory") }}, 2, index.UsizeIdx(), ops)
	})
	if !reflect.DeepEqual(events, []string{"fill:0", "fill:1", "factory_drop", "drop:0:true", "drop:1:true"}) {
		t.Fatalf("owned map cleanup=%v", events)
	}
	events = nil
	mapv := index.NewOptionIndexVec(index.UsizeIdx(), ops)
	mustPanic(t, func() {
		mapv.GetOrInsertWithOwned(0, index.IndexOnceFactory[int]{Call: func() int { events = append(events, "make:7"); return 7 }, Close: func() { events = append(events, "factory_drop"); panic("factory") }})
	})
	if mapv.Contains(0) || !reflect.DeepEqual(events, []string{"make:7", "factory_drop"}) {
		t.Fatalf("FnOnce return place=%v state=%v", events, mapv.Raw.AsGoSlice())
	}
	mapv.Close()
}

func TestOwnedCollectionAndArrayCapacity(t *testing.T) {
	ops := index.ValueOps[uint8]{Copy: true, TrivialClone: true, RustSize: 1}
	for _, tc := range []struct {
		length   uint
		consume  bool
		capacity uint
	}{{4, false, 16}, {12, true, 16}, {4, true, 8}} {
		v := index.IndexVecWithCapacity(16, index.UsizeIdx(), ops)
		for i := uint(0); i < tc.length; i++ {
			v.Push(uint8(i))
		}
		it := v.IntoIter()
		if tc.consume {
			_, _ = it.Next()
		}
		collected := index.IndexVecFromIterator(it.IntoIterator(), index.UsizeIdx(), ops)
		if collected.Raw.Cap() != tc.capacity {
			t.Fatalf("owned collect len=%d consume=%t capacity=%d want %d", tc.length, tc.consume, collected.Raw.Cap(), tc.capacity)
		}
		collected.Close()
	}
	array := make([]uint8, 2, 16)
	array[0], array[1] = 1, 2
	oldCell := &array[0]
	v := index.IndexVecFromArray(array, index.UsizeIdx(), ops)
	if v.Raw.Cap() != 2 || v.At(0) == oldCell || !reflect.DeepEqual(v.Raw.AsGoSlice(), []uint8{1, 2}) {
		t.Fatalf("fixed-array conversion cap=%d values=%v", v.Raw.Cap(), v.Raw.AsGoSlice())
	}
	boolean := index.NewOptionIndexVec(index.UsizeIdx(), index.ValueOps[bool]{Copy: true, TrivialClone: true})
	boolean.Insert(0, false)
	if boolean.Raw.Cap() != 8 {
		t.Fatalf("Option<bool> source niche capacity=%d", boolean.Raw.Cap())
	}
	custom := index.NewOptionIndexVecWithLayout(index.UsizeIdx(), index.ValueOps[bool]{Copy: true, TrivialClone: true, RustSize: 64}, index.OptionLayout{RustSize: 64})
	custom.Insert(0, true)
	if custom.Raw.Cap() != 4 {
		t.Fatalf("explicit option layout cap=%d", custom.Raw.Cap())
	}
	boolean.Close()
	custom.Close()
	v.Close()
}

func TestCloneIntoReservesBeforeClonePanic(t *testing.T) {
	source := index.IndexVecFromRaw([]uint8{7}, index.UsizeIdx(), index.ValueOps[uint8]{CloneRef: func(*uint8) uint8 { panic("clone") }, RustSize: 1})
	target := index.NewIndexVec(index.UsizeIdx(), source.Ops)
	mustPanic(t, func() { source.AsSlice().CloneInto(target) })
	if target.Len() != 0 || target.Raw.Cap() != 8 {
		t.Fatalf("clone tail panic len=%d cap=%d", target.Len(), target.Raw.Cap())
	}
	source.Close()
	target.Close()
}

func TestDebugFormatterBorrowedFlagsAndWriterDispatch(t *testing.T) {
	values := []int64{15, -1}
	var chunks []string
	opts := index.IndexFormatOptions{Fill: '*', Align: index.IndexAlignRight, Width: 10, Precision: 3, HasWidth: true, HasPrecision: true}
	ops := index.ValueOps[int64]{FormatDebug: func(value *int64, f *index.IndexDebugFormatter) error {
		if value != &values[0] && value != &values[1] {
			t.Fatal("Debug received a copied element")
		}
		if f.Options != opts {
			t.Fatalf("options changed: %#v", f.Options)
		}
		if *value < 0 {
			return f.WriteString("x\ny")
		}
		if err := f.WriteString("E"); err != nil {
			return err
		}
		return index.FormatDebugInt(*value, 64, f)
	}}
	v := index.IndexVecFromRaw(values, index.UsizeIdx(), ops)
	f := index.IndexDebugFormatter{Options: opts, Write: func(s string) error { chunks = append(chunks, s); return nil }}
	if err := v.FormatDebug(&f); err != nil {
		t.Fatal(err)
	}
	want := []string{"[", "E", "*", "*", "*", "*", "*", "*", "*", "*", "15", ", ", "x\ny", "]"}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("writes=%q", chunks)
	}
	chunks = nil
	f.Options = index.IndexFormatOptions{Fill: 0, HasWidth: true, Width: 3}
	f.WriteChar = func(c rune) error { chunks = append(chunks, "char:"+string(c)); return nil }
	if err := index.FormatDebugInt(-1, 64, &f); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(chunks, []string{"char:\x00", "char:-", "1"}) {
		t.Fatalf("independent write_char: %q", chunks)
	}
}

func TestDebugFormatterPrettyOptionAndMapError(t *testing.T) {
	var events []string
	idx := index.NewIdx(func(n uint) uint { events = append(events, fmt.Sprint("new:", n)); return n }, func(n uint) uint { return n })
	ops := index.ValueOps[int]{FormatDebug: func(v *int, f *index.IndexDebugFormatter) error {
		events = append(events, "value")
		return index.FormatDebugInt(int64(*v), 64, f)
	}}
	v := index.IndexVecFromRaw([]index.Option[int]{index.Some(15), index.None[int]()}, idx, index.OptionValueOps(ops))
	var out strings.Builder
	f := index.IndexDebugFormatter{Options: index.IndexFormatOptions{Fill: ' ', Alternate: true}, Write: func(s string) error { out.WriteString(s); return nil }}
	if err := v.FormatDebug(&f); err != nil {
		t.Fatal(err)
	}
	if want := "[\n    Some(\n        15,\n    ),\n    None,\n]"; out.String() != want {
		t.Fatalf("pretty option=%q", out.String())
	}
	events = nil
	f.Write = func(s string) error { events = append(events, "write:"+s); return errors.New("sink") }
	if err := v.DebugMapView().FormatDebug(&f); err == nil {
		t.Fatal("writer error lost")
	}
	if !reflect.DeepEqual(events, []string{"write:{", "new:2", "new:0", "new:1"}) {
		t.Fatalf("map failure=%q", events)
	}
	var context index.IndexFormatContext
	plain := index.IndexVecFromRaw([]int{1}, index.UsizeIdx(), index.ValueOps[int]{DebugRefWithContext: func(_ *int, c index.IndexFormatContext) string { context = c; return "element" }})
	if text := plain.Debug(); text != "[element]" || context.Panicking() {
		t.Fatalf("normal debug context=%#v text=%q", context, text)
	}
}

func TestIteratorShortcutsAndMapperPanicResumption(t *testing.T) {
	zst := index.IndexVecFromElemN(struct{}{}, ^uint(0), index.UsizeIdx(), index.ValueOps[struct{}]{})
	if n := zst.Iter().Count(); n != ^uint(0) {
		t.Fatalf("full uint count=%d", n)
	}
	if _, ok := zst.Iter().Nth(^uint(0) - 1); !ok {
		t.Fatal("full uint nth failed")
	}
	if _, ok := zst.IterMut().NthBack(^uint(0) - 1); !ok {
		t.Fatal("full uint nth_back failed")
	}
	if _, ok := zst.Iter().Last(); !ok {
		t.Fatal("full uint last failed")
	}
	zst.Close()
	var events []uint
	panicAt := ^uint(0)
	idx := index.NewIdx(func(n uint) uint {
		events = append(events, n)
		if n == panicAt {
			panic("new")
		}
		return n
	}, func(n uint) uint { return n })
	v := index.IndexVecFromRaw([]int{10, 20, 30, 40}, idx, index.ValueOps[int]{})
	it := v.IterEnumerated()
	panicAt = 1
	mustPanic(t, func() { it.Nth(2) })
	panicAt = ^uint(0)
	key, value, ok := it.Next()
	if !ok || key != 1 || *value != 30 {
		t.Fatalf("nth panic resumed at key=%d value=%v", key, value)
	}
	key, value, ok = it.NextBack()
	if !ok || key != 2 || *value != 40 {
		t.Fatalf("nth panic resumed back at key=%d value=%v", key, value)
	}
	if !reflect.DeepEqual(events, []uint{4, 0, 1, 1, 2}) {
		t.Fatalf("mapper trace=%v", events)
	}
	events = nil
	if n := v.Indices().Count(); n != 4 || !reflect.DeepEqual(events, []uint{4, 0, 1, 2, 3}) {
		t.Fatalf("mapped Count shortcuts constructors: %v", events)
	}
	events = nil
	if key, ok := v.Indices().Last(); !ok || key != 3 || !reflect.DeepEqual(events, []uint{4, 0, 1, 2, 3}) {
		t.Fatalf("mapped Last constructors: %v", events)
	}
}

func TestOwnedIteratorBulkSkipDropAndLast(t *testing.T) {
	var drops []int
	ops := index.ValueOps[int]{Drop: func(v *int) { drops = append(drops, *v) }}
	v := index.IndexVecFromRaw([]int{0, 1, 2, 3}, index.UsizeIdx(), ops)
	it := v.IntoIter()
	value, ok := it.NthBack(2)
	if !ok || value != 1 || !reflect.DeepEqual(drops, []int{2, 3}) {
		t.Fatalf("bulk reverse skip=%d,%v drops=%v", value, ok, drops)
	}
	it.Close()
	if !reflect.DeepEqual(drops, []int{2, 3, 0}) {
		t.Fatalf("remaining Drop=%v", drops)
	}
	drops = nil
	v = index.IndexVecFromRaw([]int{0, 1, 2, 3}, index.UsizeIdx(), ops)
	value, ok = v.IntoIter().Last()
	if !ok || value != 3 || !reflect.DeepEqual(drops, []int{0, 1, 2}) {
		t.Fatalf("owned last=%d,%v drops=%v", value, ok, drops)
	}
}
