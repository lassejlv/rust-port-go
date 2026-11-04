// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_data_structures/src/union_find.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.

package datastructures

import (
	"strconv"
	"strings"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

// UnionFindIndex adapts Rust's Idx::new and Idx::index for a typed key. The
// callbacks must obey the Idx laws: FromIndex(i) represents i, ToIndex reverses
// that conversion, and equal keys represent the same index. FromIndex may panic
// when an index exceeds the key's representable domain, as Rust index types do.
// This adapter implements the conversions needed here, not the rustc_index API.
type UnionFindIndex[Key any] struct {
	FromIndex func(uint) Key
	ToIndex   func(Key) uint
	// Equal and NotEqual independently model Rust PartialEq::eq and ::ne.
	// Nil Equal uses Go equality for comparable keys; other key types require
	// Equal. Nil NotEqual calls !Equal, matching PartialEq::ne's default.
	Equal    func(Key, Key) bool
	NotEqual func(Key, Key) bool
}

// UnionFindConfig models the source's cfg(debug_assertions). The default
// constructor enables debug assertions; use the explicit constructor for a
// release build's lookup/callback behavior.
type UnionFindConfig struct {
	DebugAssertions bool
}

// UnionFindUintIndex corresponds to Rust's Idx implementation for usize.
func UnionFindUintIndex() UnionFindIndex[uint] {
	index := rustcindex.UsizeIdx()
	return UnionFindIndex[uint]{
		FromIndex: index.New,
		ToIndex:   index.Index,
	}
}

// UnionFindUint32Index corresponds to Rust's checked Idx implementation for u32.
func UnionFindUint32Index() UnionFindIndex[uint32] {
	index := rustcindex.U32Idx()
	return UnionFindIndex[uint32]{
		FromIndex: index.New,
		ToIndex:   index.Index,
	}
}

// UnionFind is a disjoint-set forest whose parent/rank table has fixed length.
// Obtain a forest through NewUnionFind; a zero value has no index adapter.
// As with Rust's exclusive &mut self, concurrent operations require exclusion.
type UnionFind[Key any] struct {
	table  []unionFindEntry[Key]
	index  UnionFindIndex[Key]
	config UnionFindConfig
}

type unionFindEntry[Key any] struct {
	// A root key is its own parent.
	parent Key
	rank   uint32
}

// NewUnionFind creates singleton sets for keys 0..numKeys. It first validates
// FromIndex(numKeys), even for an empty forest, preserving IndexVec::from_fn_n's
// representability check before allocation and per-element index construction.
func NewUnionFind[Key any](numKeys uint, index UnionFindIndex[Key]) *UnionFind[Key] {
	return NewUnionFindWithConfig(numKeys, index, UnionFindConfig{DebugAssertions: true})
}

// NewUnionFindWithConfig selects source debug/release assertion behavior while
// retaining the same index validation and forest algorithm.
func NewUnionFindWithConfig[Key any](numKeys uint, index UnionFindIndex[Key], config UnionFindConfig) *UnionFind[Key] {
	_ = index.FromIndex(numKeys)
	table := make([]unionFindEntry[Key], numKeys)
	for i := uint(0); i < numKeys; i++ {
		table[i] = unionFindEntry[Key]{parent: index.FromIndex(i), rank: 0}
	}
	return &UnionFind[Key]{table: table, index: index, config: config}
}

func (sets *UnionFind[Key]) equal(a, b Key) bool {
	if sets.index.Equal != nil {
		return sets.index.Equal(a, b)
	}
	return any(a) == any(b)
}

func (sets *UnionFind[Key]) notEqual(a, b Key) bool {
	if sets.index.NotEqual != nil {
		return sets.index.NotEqual(a, b)
	}
	return !sets.equal(a, b)
}

// Find returns the root of key's set, performing upstream's one-layer-ahead
// path compression while advancing by one original parent at each iteration.
// An out-of-range key panics at the corresponding typed table lookup.
func (sets *UnionFind[Key]) Find(key Key) Key {
	curr := key
	for {
		parent := sets.table[sets.index.ToIndex(curr)].parent
		if !sets.notEqual(curr, parent) {
			return curr
		}
		parentParent := sets.table[sets.index.ToIndex(parent)].parent
		sets.table[sets.index.ToIndex(curr)].parent = parentParent
		curr = parent
	}
}

// Unify joins a's and b's sets and returns their common root. Greater rank wins;
// equal ranks prefer a's root and increment its rank, exactly as upstream.
func (sets *UnionFind[Key]) Unify(a, b Key) Key {
	a = sets.Find(a)
	b = sets.Find(b)
	if sets.equal(a, b) {
		return a
	}
	aRank := sets.table[sets.index.ToIndex(a)].rank
	bRank := sets.table[sets.index.ToIndex(b)].rank
	switch {
	case aRank < bRank:
		a, b = b, a
	case aRank == bRank:
		sets.table[sets.index.ToIndex(a)].rank++
	case aRank > bRank:
	}
	if sets.config.DebugAssertions {
		if !(sets.table[sets.index.ToIndex(a)].rank > sets.table[sets.index.ToIndex(b)].rank) {
			panic("UnionFind: assertion failed: a.rank > b.rank")
		}
		if !sets.equal(sets.table[sets.index.ToIndex(b)].parent, b) {
			panic("UnionFind: assertion failed: b.parent == b")
		}
	}
	sets.table[sets.index.ToIndex(b)].parent = a
	return a
}

// UnionFindSnapshot is the independent root vector returned by Snapshot. Raw
// corresponds to IndexVec::raw and may be mutated without changing the forest.
// At and Set retain typed-key indexing. This is a scoped result adapter, not a
// port of the general IndexVec container.
type UnionFindSnapshot[Key any] struct {
	Raw   []Key
	index UnionFindIndex[Key]
}

func (snapshot UnionFindSnapshot[Key]) At(key Key) Key {
	return snapshot.Raw[snapshot.index.ToIndex(key)]
}

func (snapshot UnionFindSnapshot[Key]) Set(key, root Key) {
	snapshot.Raw[snapshot.index.ToIndex(key)] = root
}

func (snapshot UnionFindSnapshot[Key]) Len() uint { return uint(len(snapshot.Raw)) }

// Snapshot maps each key to its current root and compresses paths through Find.
// The upfront FromIndex(len) call preserves IndexSlice::indices validation and
// precedes the per-key conversions in ascending index order.
func (sets *UnionFind[Key]) Snapshot() UnionFindSnapshot[Key] {
	_ = sets.index.FromIndex(uint(len(sets.table)))
	roots := make([]Key, len(sets.table))
	for i := range roots {
		roots[i] = sets.Find(sets.index.FromIndex(uint(i)))
	}
	return UnionFindSnapshot[Key]{Raw: roots, index: sets.index}
}

// Debug mirrors the derived Rust Debug representation, using formatKey for the
// key type's own Debug implementation (for example, decimal for u32/usize).
func (sets *UnionFind[Key]) Debug(formatKey func(Key) string) string {
	var result strings.Builder
	result.WriteString("UnionFind { table: [")
	for i, entry := range sets.table {
		if i != 0 {
			result.WriteString(", ")
		}
		result.WriteString("UnionFindEntry { parent: ")
		result.WriteString(formatKey(entry.parent))
		result.WriteString(", rank: ")
		result.WriteString(strconv.FormatUint(uint64(entry.rank), 10))
		result.WriteString(" }")
	}
	result.WriteString("] }")
	return result.String()
}
