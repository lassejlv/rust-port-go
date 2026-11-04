// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex_test

import (
	"reflect"
	"testing"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

func TestDefaultsOverflowAndOverrides(t *testing.T) {
	type key struct {
		n        uint
		metadata []byte
	}
	var trace []string
	adapter := index.NewIdx(func(n uint) key { trace = append(trace, "new"); return key{n, []byte{1, 2}} }, func(k key) uint { trace = append(trace, "index"); return k.n })
	k := key{4, []byte{1, 2}}
	if got := adapter.Plus(k, 3); got.n != 7 {
		t.Fatalf("plus=%#v", got)
	}
	if !reflect.DeepEqual(trace, []string{"index", "new"}) {
		t.Fatalf("trace=%v", trace)
	}
	trace = nil
	adapter.IncrementBy(&k, 2)
	if k.n != 6 || !reflect.DeepEqual(trace, []string{"index", "new"}) {
		t.Fatalf("increment=%#v trace=%v", k, trace)
	}
	adapter.PlusOverride = func(k key, n uint) key { trace = append(trace, "plus override"); return key{k.n + n + 10, k.metadata} }
	trace = nil
	adapter.IncrementBy(&k, 2)
	if k.n != 18 || !reflect.DeepEqual(trace, []string{"plus override"}) {
		t.Fatalf("overridden plus=%#v trace=%v", k, trace)
	}
	adapter.IncrementByOverride = func(k *key, n uint) { k.n = n; panic("increment override") }
	mustPanic(t, func() { adapter.IncrementBy(&k, 27) })
	if k.n != 27 {
		t.Fatal("override mutation must survive panic")
	}
	checked := index.UsizeIdx()
	maximum := ^uint(0)
	mustPanic(t, func() { checked.Plus(maximum, 1) })
	k2 := maximum
	mustPanic(t, func() { checked.IncrementBy(&k2, 1) })
	if k2 != maximum {
		t.Fatal("default increment assigned before failure")
	}
	unchecked := index.UsizeIdxWithConfig(index.IdxConfig{OverflowChecks: false})
	if got := unchecked.Plus(maximum, 1); got != 0 {
		t.Fatalf("wrapping plus=%d", got)
	}
	maxU32 := uint(^uint32(0))
	if maximum > maxU32 {
		beyond := maxU32
		beyond++
		mustPanic(t, func() { index.U32Idx().New(beyond) })
		mustPanic(t, func() { index.U32Idx().Plus(^uint32(0), 1) })
	}
}

func TestStandardRangeConversions(t *testing.T) {
	adapter := index.UsizeIdx()
	for _, tc := range []struct{ got, want index.TypedSliceIndexResult }{
		{index.SingleSliceIdx(adapter).IntoSliceIdx(7), index.TypedSliceIndexResult{Kind: index.SliceIndexSingle, Index: 7}},
		{index.FullSliceIdx().IntoSliceIdx(index.RangeFull{}), index.TypedSliceIndexResult{Kind: index.SliceIndexFull}},
		{index.RangeSliceIdx(adapter).IntoSliceIdx(index.Range[uint]{Start: 9, End: 2}), index.TypedSliceIndexResult{Kind: index.SliceIndexRange, Start: 9, End: 2}},
		{index.RangeFromSliceIdx(adapter).IntoSliceIdx(index.RangeFrom[uint]{Start: 3}), index.TypedSliceIndexResult{Kind: index.SliceIndexFrom, Start: 3}},
		{index.RangeToSliceIdx(adapter).IntoSliceIdx(index.RangeTo[uint]{End: 4}), index.TypedSliceIndexResult{Kind: index.SliceIndexTo, End: 4}},
		{index.RangeInclusiveSliceIdx(adapter).IntoSliceIdx(index.RangeInclusive[uint]{Start: 0, End: ^uint(0), Exhausted: true}), index.TypedSliceIndexResult{Kind: index.SliceIndexInclusive, Start: 0, End: ^uint(0)}},
		{index.RangeToInclusiveSliceIdx(adapter).IntoSliceIdx(index.RangeToInclusive[uint]{End: ^uint(0)}), index.TypedSliceIndexResult{Kind: index.SliceIndexToInclusive, End: ^uint(0)}},
	} {
		if tc.got != tc.want {
			t.Errorf("conversion=%#v, want %#v", tc.got, tc.want)
		}
	}
	var calls []uint
	traced := index.NewIdx(func(n uint) uint { return n }, func(n uint) uint {
		calls = append(calls, n)
		if n == 2 {
			panic("second endpoint")
		}
		return n
	})
	mustPanic(t, func() { index.RangeSliceIdx(traced).IntoSliceIdx(index.Range[uint]{Start: 1, End: 2}) })
	if !reflect.DeepEqual(calls, []uint{1, 2}) {
		t.Fatalf("endpoint conversion order=%v", calls)
	}
	custom := index.IntoSliceIdx[string, index.RangeTo[uint]]{Convert: func(s string) index.RangeTo[uint] { return index.RangeTo[uint]{End: uint(len(s))} }}
	if got := custom.IntoSliceIdx("hello"); got.End != 5 {
		t.Fatalf("custom associated output=%#v", got)
	}
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
