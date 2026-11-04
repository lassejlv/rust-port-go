// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"fmt"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

func storageCustomRequests() []storageRequest {
	var out []storageRequest
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, bounds := range [][2]uint{{0, 0}, {0, 3}, {1, 2}, {2, 1}, {1, 5}} {
				for _, op := range []string{"custom_get", "custom_get_mut", "custom_index", "custom_index_mut", "custom_copy"} {
					for _, fail := range []string{"", "convert", "input_close", "index"} {
						out = append(out, storageRequest{Name: fmt.Sprintf("%s-%d-%d-%s", op, bounds[0], bounds[1], fail), Checked: checked, Debug: debug, Values: []int64{1, 3, 5}, Other: []int64{}, Capacity: 3, Limit: 64, Actions: []storageAction{{Op: op, A: bounds[0], B: bounds[1], C: 0, Panic: fail, PanicAt: 1}}})
					}
				}
			}
		}
	}
	return out
}

func storageCustomSlice(h *storageHarness, v *index.IndexVec[uint, int64], a storageAction) any {
	type sourceRange struct{ Start, End uint }
	convert := index.IntoSliceIdx[sourceRange, sourceRange]{Convert: func(input sourceRange) (result sourceRange) {
		defer func() {
			if p := recover(); p != nil {
				h.event("input_close", true)
				panic(p)
			}
			h.event("input_close", false)
		}()
		h.event("convert")
		return input
	}}
	get := func(raw index.IndexRaw[int64], r sourceRange) (index.SliceRef[int64], bool) {
		if r.Start > r.End || r.End > raw.Len() {
			return index.SliceRef[int64]{}, false
		}
		return index.SliceRef[int64]{Raw: raw.Slice(r.Start, r.End)}, true
	}
	indexed := func(raw index.IndexRaw[int64], r sourceRange) index.SliceRef[int64] {
		out, ok := get(raw, r)
		if !ok {
			panic("slice index bounds")
		}
		return out
	}
	adapter := index.SliceIndexer[int64, sourceRange]{Get: get, GetMut: get, Index: indexed, IndexMut: indexed}
	input := sourceRange{a.A, a.B}
	if a.Op == "custom_copy" {
		index.IndexSliceCopyWithinWith(v.AsMutSlice(), input, convert, func(r sourceRange) index.IndexRangeBounds {
			return index.BoundsFromSliceIndex(index.TypedSliceIndexResult{Kind: index.SliceIndexRange, Start: r.Start, End: r.End})
		}, a.C)
		return nil
	}
	var slice index.SliceRef[int64]
	var ok bool
	switch a.Op {
	case "custom_get":
		slice, ok = index.IndexSliceGetWith(v.AsSlice(), input, convert, adapter)
	case "custom_get_mut":
		slice, ok = index.IndexSliceGetMutWith(v.AsMutSlice(), input, convert, adapter)
	case "custom_index":
		slice = index.IndexSliceIndexWith(v.AsSlice(), input, convert, adapter)
		ok = true
	case "custom_index_mut":
		slice = index.IndexSliceIndexMutWith(v.AsMutSlice(), input, convert, adapter)
		ok = true
	default:
		panic("custom operation")
	}
	if !ok {
		return nil
	}
	out := []int64{}
	for i := uint(0); i < slice.Raw.Len(); i++ {
		value := slice.Raw.At(i)
		if a.Op == "custom_get_mut" || a.Op == "custom_index_mut" {
			*value += 1
		}
		out = append(out, *value)
	}
	return out
}
