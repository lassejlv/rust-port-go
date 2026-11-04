//go:build ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_index/src/idx.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179: cfg(feature = "nightly") impls.
package rustcindex

func CoreRangeSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[CoreRange[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[CoreRange[Key], TypedSliceIndexResult]{Convert: func(r CoreRange[Key]) TypedSliceIndexResult {
		start := index.Index(r.Start)
		end := index.Index(r.End)
		return TypedSliceIndexResult{Kind: SliceIndexCoreRange, Start: start, End: end}
	}}
}
func CoreRangeFromSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[CoreRangeFrom[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[CoreRangeFrom[Key], TypedSliceIndexResult]{Convert: func(r CoreRangeFrom[Key]) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexCoreFrom, Start: index.Index(r.Start)}
	}}
}
func CoreRangeInclusiveSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[CoreRangeInclusive[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[CoreRangeInclusive[Key], TypedSliceIndexResult]{Convert: func(r CoreRangeInclusive[Key]) TypedSliceIndexResult {
		start := index.Index(r.Start)
		last := index.Index(r.Last)
		return TypedSliceIndexResult{Kind: SliceIndexCoreInclusive, Start: start, Last: last}
	}}
}
func CoreRangeToInclusiveSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[CoreRangeToInclusive[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[CoreRangeToInclusive[Key], TypedSliceIndexResult]{Convert: func(r CoreRangeToInclusive[Key]) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexCoreToInclusive, Last: index.Index(r.Last)}
	}}
}
