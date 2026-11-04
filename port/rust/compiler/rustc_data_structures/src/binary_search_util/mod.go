// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_data_structures/src/binary_search_util/mod.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See licenses in port/rust.

// Package binarysearchutil searches a sorted slice as a key-to-many-values map.
package binarysearchutil

import "cmp"

// BinarySearchSlice finds the contiguous subslice whose extracted keys equal key.
// data must be sorted by keyFn using Go's cmp.Compare ordering. The returned
// slice aliases data, and a missing key returns nil. The search itself allocates
// no storage.
//
// BinarySearchSliceWithOperators supplies the full Rust trait adapter. This convenience
// adapter uses Go's ordered-key semantics, including cmp.Compare's total ordering
// of floating-point NaNs; Rust primitive floats do not implement Ord.
func BinarySearchSlice[E any, K cmp.Ordered](data []E, keyFn func(*E) K, key K) []E {
	return BinarySearchSliceBy(data, keyFn, key, cmp.Compare[K])
}

// KeyOperators adapts the three independent Rust trait methods used by the
// upstream function: PartialOrd::lt, PartialEq::ne and PartialOrd::le. Callbacks
// are invoked independently; one operator is never inferred from another.
// As in Rust, unused operators may panic without affecting a search.
type KeyOperators[K any] struct {
	Less        func(K, K) bool
	NotEqual    func(K, K) bool
	LessOrEqual func(K, K) bool
}

// BinarySearchSliceBy derives all three operators from one total-order
// comparator. compare returns negative, zero or positive for less, equal or
// greater. Use BinarySearchSliceWithOperators when Rust trait implementations
// independently override operator behavior, observations or panic timing.
func BinarySearchSliceBy[E, K any](data []E, keyFn func(*E) K, key K, compare func(K, K) int) []E {
	return BinarySearchSliceWithOperators(data, keyFn, key, KeyOperators[K]{
		Less:        func(a, b K) bool { return compare(a, b) < 0 },
		NotEqual:    func(a, b K) bool { return compare(a, b) != 0 },
		LessOrEqual: func(a, b K) bool { return compare(a, b) <= 0 },
	})
}

// BinarySearchSliceWithOperators is the full binary_search_slice port, including
// independent Rust operator overrides. data must be sorted consistently with the
// operators and keyFn. Results for inconsistent orders are unspecified. Callback
// panics propagate, and empty input calls neither the extractor nor operators.
//
// The result aliases data. Rust's shared-borrow immutability has no static Go
// counterpart, so callers must preserve the sorted data while searching it.
// Two partition-point searches use O(log n) key probes and O(1) auxiliary space.
func BinarySearchSliceWithOperators[E, K any](data []E, keyFn func(*E) K, key K, operators KeyOperators[K]) []E {
	size := len(data)
	start := partitionPoint(data, func(value *E) bool { return operators.Less(keyFn(value), key) })
	if start == size || operators.NotEqual(keyFn(&data[start]), key) {
		return nil
	}
	offset := start + 1
	end := partitionPoint(data[offset:], func(value *E) bool { return operators.LessOrEqual(keyFn(value), key) }) + offset
	return data[start:end]
}

// partitionPoint preserves the fixed-iteration probing used by Rust's slice
// partition_point/binary_search_by, including its final repeated probe. Using
// sort.Search here would change observable callback counts and panic timing.
func partitionPoint[E any](data []E, predicate func(*E) bool) int {
	size := len(data)
	if size == 0 {
		return 0
	}
	base := 0
	for size > 1 {
		half := size / 2
		middle := base + half
		if predicate(&data[middle]) {
			base = middle
		}
		size -= half
	}
	if predicate(&data[base]) {
		return base + 1
	}
	return base
}
