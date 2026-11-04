//go:build ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
package indexoracle

import index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"

const NightlyEnabled = true

func nightlyConvert[K any](adapter index.Idx[K], a, b K, operation string) index.TypedSliceIndexResult {
	switch operation {
	case "core_range":
		return index.CoreRangeSliceIdx(adapter).IntoSliceIdx(index.CoreRange[K]{Start: a, End: b})
	case "core_from":
		return index.CoreRangeFromSliceIdx(adapter).IntoSliceIdx(index.CoreRangeFrom[K]{Start: a})
	case "core_inclusive":
		return index.CoreRangeInclusiveSliceIdx(adapter).IntoSliceIdx(index.CoreRangeInclusive[K]{Start: a, Last: b})
	case "core_to_inclusive":
		return index.CoreRangeToInclusiveSliceIdx(adapter).IntoSliceIdx(index.CoreRangeToInclusive[K]{Last: b})
	}
	panic("unknown nightly operation")
}
