// SPDX-License-Identifier: MIT OR Apache-2.0
// Source-aligned cases from rust-lang/rust compiler/rustc_index/src/vec/tests.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179; original macro expansion and all
// five unchanged Rust tests are retained by the reference-build tool.
package vec_test

import (
	"reflect"
	"strconv"
	"testing"
	"unsafe"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

// Go does not perform Rust enum niche inference. This selected test key uses an
// explicit u32 encoding, including the five reserved option discriminants. The
// actual Rust layout/Step assertions run unchanged in the development oracle;
// these test-local representations are not a port of newtype_index! or its ABI.
type myIdx uint32
type myIdxNiche uint32
type myIdxTagged struct{ tag, value uint32 }

const myIdxMax uint32 = 0xfffffffa

func myIdxAdapter() index.Idx[myIdx] {
	return index.NewIdx(func(n uint) myIdx {
		if n > uint(myIdxMax) {
			panic("MyIdx max")
		}
		return myIdx(n)
	}, func(k myIdx) uint { return uint(k) })
}

// These selected Step operations come from the unchanged macro expansion in
// expanded-{normal,nightly}.rs retained by the reference-build tool. These are
// test support for this
// invocation, not a general newtype_index! or Rust range library port.
func myIdxForwardChecked(start myIdx, amount uint) (myIdx, bool) {
	n := uint(start)
	if amount > ^uint(0)-n {
		return 0, false
	}
	return myIdxAdapter().New(n + amount), true
}
func myIdxBackwardChecked(start myIdx, amount uint) (myIdx, bool) {
	n := uint(start)
	if amount > n {
		return 0, false
	}
	return myIdxAdapter().New(n - amount), true
}
func myIdxStepsBetween(start, end myIdx) (uint, uint, bool) {
	if end < start {
		return 0, 0, false
	}
	n := uint(end - start)
	return n, n, true
}

type myIdxStepRange struct{ start, end myIdx }

func myIdxRange() *myIdxStepRange {
	return &myIdxStepRange{start: myIdxAdapter().New(1), end: myIdxAdapter().New(4)}
}
func (r *myIdxStepRange) Next() (myIdx, bool) {
	if r.start >= r.end {
		return 0, false
	}
	value := r.start
	r.start, _ = myIdxForwardChecked(value, 1)
	return value, true
}
func (r *myIdxStepRange) NextBack() (myIdx, bool) {
	if r.start >= r.end {
		return 0, false
	}
	r.end, _ = myIdxBackwardChecked(r.end, 1)
	return r.end, true
}
func (r *myIdxStepRange) Count() uint {
	n, _, _ := myIdxStepsBetween(r.start, r.end)
	r.start = r.end
	return n
}
func (r *myIdxStepRange) SizeHint() (uint, uint) {
	a, b, _ := myIdxStepsBetween(r.start, r.end)
	return a, b
}

func TestIndexSizeIsOptimized(t *testing.T) {
	if unsafe.Sizeof(myIdx(0)) != 4 {
		t.Fatal("selected key encoding must occupy four bytes")
	}
	for depth := uint32(1); depth <= 5; depth++ {
		encodedNone := myIdxNiche(myIdxMax + depth)
		if unsafe.Sizeof(encodedNone) != 4 || uint32(encodedNone) <= myIdxMax {
			t.Fatalf("depth %d encoding=%x", depth, encodedNone)
		}
	}
	if unsafe.Sizeof(myIdxTagged{}) != 8 {
		t.Fatal("sixth option uses explicit tag and payload")
	}
}
func TestRangeIteratorIteratesForwards(t *testing.T) {
	it := myIdxRange()
	var got []myIdx
	for {
		v, ok := it.Next()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !reflect.DeepEqual(got, []myIdx{1, 2, 3}) {
		t.Fatalf("forward=%v", got)
	}
}
func TestRangeIteratorIteratesBackwards(t *testing.T) {
	it := myIdxRange()
	var got []myIdx
	for {
		v, ok := it.NextBack()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !reflect.DeepEqual(got, []myIdx{3, 2, 1}) {
		t.Fatalf("backward=%v", got)
	}
}
func TestRangeCountIsCorrect(t *testing.T) {
	it := myIdxRange()
	if n := it.Count(); n != 3 {
		t.Fatalf("count=%d", n)
	}
}
func TestRangeSizeHintIsCorrect(t *testing.T) {
	it := myIdxRange()
	a, b := it.SizeHint()
	if a != 3 || b != 3 {
		t.Fatalf("hint=(%d,%d)", a, b)
	}
}

func TestLogicalZeroSizedDomain(t *testing.T) {
	maximum := ^uint(0)
	ops := index.ValueOps[struct{}]{Copy: true, TrivialClone: true}
	v := index.IndexVecFromElemN(struct{}{}, maximum, index.UsizeIdx(), ops)
	if v.Len() != maximum || v.Raw.Cap() != maximum {
		t.Fatalf("ZST len=%d cap=%d", v.Len(), v.Raw.Cap())
	}
	if _, ok := v.Get(maximum - 1); !ok {
		t.Fatal("last ZST element missing")
	}
	if _, ok := v.Get(maximum); ok {
		t.Fatal("out-of-bounds ZST element accepted")
	}
	if key, found := v.BinarySearch(struct{}{}); !found || key != maximum-1 {
		t.Fatalf("ZST search=(%d,%t)", key, found)
	}
	it := v.Indices()
	if key, ok := it.NextBack(); !ok || key != maximum-1 {
		t.Fatalf("ZST iterator=(%d,%t)", key, ok)
	}
	_, _ = v.Pick2Mut(0, maximum-1)
	v.CopyWithin(index.TypedSliceIndexResult{Kind: index.SliceIndexRange, Start: 1, End: maximum}, 0)
	mustPanic(t, func() { v.Push(struct{}{}) })
	mustPanic(t, func() { v.Raw.AsGoSlice() })
	clone := v.Clone()
	if clone.Len() != maximum {
		t.Fatal("ZST clone truncated length")
	}
	clone.Close()
	v.Close()
	if v.Len() != 0 {
		t.Fatal("ZST close")
	}
	capacity := index.IndexVecWithCapacity(maximum, index.UsizeIdx(), ops)
	if capacity.Len() != 0 || capacity.Raw.Cap() != maximum {
		t.Fatal("ZST capacity constructor")
	}
	wrapping := index.IndexVecFromElemNWithConfig(struct{}{}, maximum, index.UsizeIdx(), ops, index.IndexStorageConfig{OverflowChecks: false, DebugAssertions: false})
	wrapping.ResizeToElem(maximum, func() struct{} { t.Fatal("wrapped resize must not fill"); return struct{}{} })
	if wrapping.Len() != 0 {
		t.Fatal("raw usize wrapping resize")
	}
	checked := index.IndexVecFromElemN(struct{}{}, maximum, index.UsizeIdx(), ops)
	mustPanic(t, func() { checked.ResizeToElem(maximum, func() struct{} { return struct{}{} }) })
	checked.Close()
	if strconv.IntSize == 64 {
		limited := index.IndexVecFromElemN(struct{}{}, maximum, index.U32Idx(), ops)
		mustPanic(t, func() { limited.Indices() })
		limited.Close()
	}
}

func TestStorageCallbacksAndMutation(t *testing.T) {
	type key struct {
		n        uint
		metadata []byte
	}
	var constructors []uint
	idx := index.NewIdx(func(n uint) key { constructors = append(constructors, n); return key{n, []byte{1}} }, func(k key) uint { return k.n })
	var clones, froms, drops []int
	ops := index.ValueOps[int]{Clone: func(v int) int { clones = append(clones, v); return v }, CloneFrom: func(dst *int, v int) { froms = append(froms, *dst); *dst = v }, Drop: func(v *int) { drops = append(drops, *v) }}
	v := index.IndexVecFromFnN(func(k key) int { return int(k.n) }, 4, idx, ops)
	if !reflect.DeepEqual(constructors, []uint{4, 0, 1, 2, 3}) {
		t.Fatalf("constructor order=%v", constructors)
	}
	*p(v.Get(key{1, nil})) = 11
	place := v.EnsureContainsElem(key{6, nil}, func() int { return 9 })
	*place = 66
	if got := v.Raw.AsGoSlice(); !reflect.DeepEqual(got, []int{0, 11, 2, 3, 9, 9, 66}) {
		t.Fatalf("growth=%v", got)
	}
	if a, b := v.Pick2Mut(key{0, nil}, key{6, nil}); a == nil || b == nil {
		t.Fatal("disjoint references")
	}
	copy := v.Clone()
	if len(clones) != 7 {
		t.Fatalf("clone calls=%v", clones)
	}
	copy.AsSlice().CloneInto(v)
	if len(froms) != 7 {
		t.Fatalf("clone_from calls=%v", froms)
	}
	before := len(froms)
	v.CloneFrom(copy)
	if len(froms) != before {
		t.Fatal("derived clone_from must clone whole vector instead of reusing element clone_from")
	}
	copy.Close()
	v.Close()
	if len(drops) == 0 {
		t.Fatal("drop callbacks were omitted")
	}
}
func p[T any](value *T, ok bool) *T {
	if !ok {
		panic("missing value")
	}
	return value
}

func TestDrainAndRangeConversions(t *testing.T) {
	idx := index.UsizeIdx()
	var dropped []int
	ops := index.ValueOps[int]{Drop: func(v *int) { dropped = append(dropped, *v) }}
	v := index.IndexVecFromRaw([]int{0, 1, 2, 3, 4}, idx, ops)
	var callbacks []string
	bounds := index.IndexRangeBounds{StartBound: func() index.IndexBound {
		callbacks = append(callbacks, "start")
		return index.IndexBound{Kind: index.IndexIncluded, Value: 1}
	}, EndBound: func() index.IndexBound {
		callbacks = append(callbacks, "end")
		return index.IndexBound{Kind: index.IndexExcluded, Value: 4}
	}}
	drain := v.DrainEnumerated(bounds)
	key, value, ok := drain.Next()
	if !ok || key != 1 || value != 1 {
		t.Fatalf("drain=(%d,%d,%t)", key, value, ok)
	}
	drain.Close()
	if !reflect.DeepEqual(callbacks, []string{"start", "start", "end"}) {
		t.Fatalf("bound call order=%v", callbacks)
	}
	if !reflect.DeepEqual(dropped, []int{2, 3}) || !reflect.DeepEqual(v.Raw.AsGoSlice(), []int{0, 4}) {
		t.Fatalf("drain result=%v dropped=%v", v.Raw.AsGoSlice(), dropped)
	}
	view, ok := v.GetIndex(index.TypedSliceIndexResult{Kind: index.SliceIndexInclusive, Start: 1, End: 0})
	if !ok || view.Raw.Len() != 0 {
		t.Fatalf("empty inclusive range=%#v,%t", view, ok)
	}
	if _, ok := v.GetIndex(index.TypedSliceIndexResult{Kind: index.SliceIndexInclusive, Start: 0, End: ^uint(0)}); ok {
		t.Fatal("inclusive maximum must fail bounds without overflow")
	}
	mustPanic(t, func() { v.Pick3Mut(0, 0, 100) })
	v.Close()
}

func TestOptionsAndInverse(t *testing.T) {
	mapv := index.NewOptionIndexVec(index.UsizeIdx(), index.ValueOps[int]{})
	if old := mapv.Insert(3, 9); old.Present {
		t.Fatal("initial insert")
	}
	if old := mapv.Insert(3, 10); !old.Present || old.Value != 9 {
		t.Fatalf("replace=%#v", old)
	}
	if value := mapv.GetOrInsertWith(3, func() int { t.Fatal("occupied option evaluated initializer"); return 0 }); *value != 10 {
		t.Fatal("occupied value")
	}
	if !mapv.Contains(3) || mapv.Contains(100) {
		t.Fatal("contains")
	}
	if got := mapv.DebugMapViewCompact().Debug(); got != "{3: 10}" {
		t.Fatalf("compact debug=%s", got)
	}
	if old := mapv.Remove(3); !old.Present || old.Value != 10 {
		t.Fatalf("remove=%#v", old)
	}
	v := index.IndexVecFromRaw([]uint{2, 0, 1}, index.UsizeIdx(), index.ValueOps[uint]{Copy: true, TrivialClone: true})
	inverse := v.AsSlice().InvertBijectiveMapping(index.UsizeIdx(), index.ValueOps[uint]{Copy: true, TrivialClone: true})
	if !reflect.DeepEqual(inverse.Raw.AsGoSlice(), []uint{1, 2, 0}) {
		t.Fatalf("inverse=%v", inverse.Raw.AsGoSlice())
	}
	v.Close()
	inverse.Close()
	mapv.Close()
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("expected panic")
		}
	}()
	f()
}
