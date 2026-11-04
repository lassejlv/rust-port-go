// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_index/src/idx.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.

// Package rustcindex contains source-ported typed-index operations. This file
// provides Idx and IntoSliceIdx adapters; it does not implement IndexVec,
// IndexSlice, or the standard library's SliceIndex operations.
package rustcindex

// IdxConfig selects the Rust usize addition behavior in the default Plus
// implementation. Checked overflow panics before FromIndex; unchecked addition
// wraps at the current host's uint width, corresponding to Rust usize.
type IdxConfig struct{ OverflowChecks bool }

// Idx adapts the complete Idx trait's required and default methods. Keys must
// have Rust Copy-like value semantics and lawful Eq/Hash behavior when used by
// consumers that require those supertraits. Key any permits keys containing
// reference-like slice metadata; Go comparability is not an extra restriction.
// The optional method overrides independently model concrete Rust impl methods.
type Idx[Key any] struct {
	FromIndex           func(uint) Key
	ToIndex             func(Key) uint
	PlusOverride        func(Key, uint) Key
	IncrementByOverride func(*Key, uint)
	Config              IdxConfig
}

// NewIdx enables overflow checks, matching a checked Rust build.
func NewIdx[Key any](from func(uint) Key, to func(Key) uint) Idx[Key] {
	return NewIdxWithConfig(from, to, IdxConfig{OverflowChecks: true})
}

func NewIdxWithConfig[Key any](from func(uint) Key, to func(Key) uint, config IdxConfig) Idx[Key] {
	return Idx[Key]{FromIndex: from, ToIndex: to, Config: config}
}

func (index Idx[Key]) New(value uint) Key { return index.FromIndex(value) }
func (index Idx[Key]) Index(key Key) uint { return index.ToIndex(key) }

// Plus preserves the source order Index, usize addition, then New. Its returned
// key should be used; IncrementBy is the explicit in-place operation.
func (index Idx[Key]) Plus(key Key, amount uint) Key {
	if index.PlusOverride != nil {
		return index.PlusOverride(key, amount)
	}
	value := index.Index(key)
	if index.Config.OverflowChecks && amount > ^uint(0)-value {
		panic("rustcindex: usize addition overflow")
	}
	return index.New(value + amount)
}

// IncrementBy calls the actual Plus implementation, including its override.
// Default assignment happens only after Plus succeeds. A concrete increment
// override receives the original mutable key and can mutate before panicking.
func (index Idx[Key]) IncrementBy(key *Key, amount uint) {
	if index.IncrementByOverride != nil {
		index.IncrementByOverride(key, amount)
		return
	}
	*key = index.Plus(*key, amount)
}

func UsizeIdx() Idx[uint] { return UsizeIdxWithConfig(IdxConfig{OverflowChecks: true}) }
func UsizeIdxWithConfig(config IdxConfig) Idx[uint] {
	return NewIdxWithConfig(func(i uint) uint { return i }, func(i uint) uint { return i }, config)
}

func U32Idx() Idx[uint32] { return U32IdxWithConfig(IdxConfig{OverflowChecks: true}) }
func U32IdxWithConfig(config IdxConfig) Idx[uint32] {
	return NewIdxWithConfig(func(i uint) uint32 {
		if i > uint(^uint32(0)) {
			panic("rustcindex: Idx::new index exceeds u32::MAX")
		}
		return uint32(i)
	}, func(i uint32) uint { return uint(i) }, config)
}

// IntoSliceIdx adapts arbitrary implementations of the public conversion trait.
// Input is the implementing type and Output its associated SliceIndex output.
// The callback's output must meet the downstream slice-index contract; no
// slicing, normalization or bounds check is performed by this trait adapter.
type IntoSliceIdx[Input, Output any] struct{ Convert func(Input) Output }

func (adapter IntoSliceIdx[Input, Output]) IntoSliceIdx(input Input) Output {
	return adapter.Convert(input)
}

type RangeFull struct{}
type Range[Key any] struct{ Start, End Key }
type RangeFrom[Key any] struct{ Start Key }
type RangeTo[Key any] struct{ End Key }

// RangeInclusive records the endpoints as returned by Rust start()/end(), plus
// exhaustion state. Conversion deliberately discards exhaustion by constructing
// a fresh inclusive range, even if the original range was exhausted.
type RangeInclusive[Key any] struct {
	Start, End Key
	Exhausted  bool
}
type RangeToInclusive[Key any] struct{ End Key }

// Core range values are the source nightly-feature input types. Their four
// conversion adapters are available with Go build tag ferrugo_rustc_nightly.
type CoreRange[Key any] struct{ Start, End Key }
type CoreRangeFrom[Key any] struct{ Start Key }
type CoreRangeInclusive[Key any] struct{ Start, Last Key }
type CoreRangeToInclusive[Key any] struct{ Last Key }

type SliceIndexKind uint8

const (
	SliceIndexSingle SliceIndexKind = iota
	SliceIndexFull
	SliceIndexRange
	SliceIndexFrom
	SliceIndexTo
	SliceIndexInclusive
	SliceIndexToInclusive
	SliceIndexCoreRange
	SliceIndexCoreFrom
	SliceIndexCoreInclusive
	SliceIndexCoreToInclusive
)

// TypedSliceIndexResult is the associated output's discriminated representation.
// Single uses Index; Range uses Start/End; From uses Start; To uses End; inclusive
// ops ranges use End, and inclusive core ranges use Last. Full has no bounds.
// Built-in conversions always produce an unexhausted inclusive result. Keeping
// inclusive endpoints avoids introducing end+1 overflow or bounds validation
// that the original conversion does not perform.
type TypedSliceIndexResult struct {
	Kind                    SliceIndexKind
	Index, Start, End, Last uint
	Exhausted               bool
}

func SingleSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[Key, TypedSliceIndexResult] {
	return IntoSliceIdx[Key, TypedSliceIndexResult]{Convert: func(key Key) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexSingle, Index: index.Index(key)}
	}}
}
func FullSliceIdx() IntoSliceIdx[RangeFull, TypedSliceIndexResult] {
	return IntoSliceIdx[RangeFull, TypedSliceIndexResult]{Convert: func(RangeFull) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexFull}
	}}
}
func RangeSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[Range[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[Range[Key], TypedSliceIndexResult]{Convert: func(r Range[Key]) TypedSliceIndexResult {
		start := index.Index(r.Start)
		end := index.Index(r.End)
		return TypedSliceIndexResult{Kind: SliceIndexRange, Start: start, End: end}
	}}
}
func RangeFromSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[RangeFrom[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[RangeFrom[Key], TypedSliceIndexResult]{Convert: func(r RangeFrom[Key]) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexFrom, Start: index.Index(r.Start)}
	}}
}
func RangeToSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[RangeTo[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[RangeTo[Key], TypedSliceIndexResult]{Convert: func(r RangeTo[Key]) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexTo, End: index.Index(r.End)}
	}}
}
func RangeInclusiveSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[RangeInclusive[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[RangeInclusive[Key], TypedSliceIndexResult]{Convert: func(r RangeInclusive[Key]) TypedSliceIndexResult {
		start := index.Index(r.Start)
		end := index.Index(r.End)
		return TypedSliceIndexResult{Kind: SliceIndexInclusive, Start: start, End: end}
	}}
}
func RangeToInclusiveSliceIdx[Key any](index Idx[Key]) IntoSliceIdx[RangeToInclusive[Key], TypedSliceIndexResult] {
	return IntoSliceIdx[RangeToInclusive[Key], TypedSliceIndexResult]{Convert: func(r RangeToInclusive[Key]) TypedSliceIndexResult {
		return TypedSliceIndexResult{Kind: SliceIndexToInclusive, End: index.Index(r.End)}
	}}
}
