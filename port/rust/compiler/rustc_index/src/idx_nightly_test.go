//go:build ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex_test

import (
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"testing"
)

func TestNightlyRangeConversions(t *testing.T) {
	adapter := index.UsizeIdx()
	for _, tc := range []struct{ got, want index.TypedSliceIndexResult }{
		{index.CoreRangeSliceIdx(adapter).IntoSliceIdx(index.CoreRange[uint]{Start: 2, End: 1}), index.TypedSliceIndexResult{Kind: index.SliceIndexCoreRange, Start: 2, End: 1}},
		{index.CoreRangeFromSliceIdx(adapter).IntoSliceIdx(index.CoreRangeFrom[uint]{Start: 3}), index.TypedSliceIndexResult{Kind: index.SliceIndexCoreFrom, Start: 3}},
		{index.CoreRangeInclusiveSliceIdx(adapter).IntoSliceIdx(index.CoreRangeInclusive[uint]{Start: 4, Last: ^uint(0)}), index.TypedSliceIndexResult{Kind: index.SliceIndexCoreInclusive, Start: 4, Last: ^uint(0)}},
		{index.CoreRangeToInclusiveSliceIdx(adapter).IntoSliceIdx(index.CoreRangeToInclusive[uint]{Last: 5}), index.TypedSliceIndexResult{Kind: index.SliceIndexCoreToInclusive, Last: 5}},
	} {
		if tc.got != tc.want {
			t.Errorf("nightly conversion=%#v, want %#v", tc.got, tc.want)
		}
	}
}
