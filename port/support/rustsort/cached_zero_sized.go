// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

// cachedZeroIndexType retains the logical source usize length. Converting an
// impossible physical Go slice length to int would corrupt the u32/usize choice.
func cachedZeroIndexType(length uint, config CachedConfig, c Context) Type {
	// Validate the actual source key layout/pointer profile using the existing
	// engine's checks. Both candidate tuples have a nonzero integer index.
	_ = cachedIndexType(0, config, c)
	wide := tupleIndexType(config.KeyType, config.PointerBits/8, c)
	narrow := tupleIndexType(config.KeyType, 4, c)
	if narrow.Size < wide.Size && uint64(length) <= uint64(^uint32(0)) {
		return narrow
	}
	return wide
}

// CachedZeroSized is sort_by_cached_key for a genuine logical slice of a
// zero-sized source T. The caller supplies its actual shared ZST borrow and
// full usize length, as provided by the existing IndexRaw storage. It does not
// materialize a fictitious []T. Key/Fn traits and borrowing laws are identical
// to Cached; all current runtime metadata is explicit for this operation.
//
// A source ZST swap moves no bytes. Keys, tuple indices, comparisons, iterator
// parameter destruction, permutation bookkeeping and key destruction still
// run normally. In particular, the nonzero (K,index) vector must allocate even
// when K is zero-sized; capacity failure precedes the first key callback.
func CachedZeroSized[T, K any](length uint, value *T, function KeyFunction[T, K], config CachedConfig, ops CachedKeyOps[K], contexts ...Context) {
	var c Context
	if len(contexts) > 1 {
		panic("rustsort: expected at most one context")
	}
	if len(contexts) == 1 {
		c = contexts[0]
	}
	function = function.MoveWithContext(c)
	defer cleanupKeyFunction(function, c)
	if length < 2 {
		return
	}
	if value == nil {
		c.Panic("rustsort: missing actual source ZST borrow")
	}
	pairType := cachedZeroIndexType(length, config, c)
	type state struct {
		length, next uint
		value        *T
		function     KeyFunction[T, K]
	}
	iteratorOps := rustvec.IteratorOps[state, cachedIndex[K]]{
		TrustedLength: true,
		Next: func(s rustvec.Ref[state]) (cachedIndex[K], bool) {
			if s.Value.next == s.Value.length {
				return cachedIndex[K]{}, false
			}
			index := s.Value.next
			s.Value.next++
			key := s.Value.function.Call(s.Value.value, s.Context)
			return cachedIndex[K]{key: key.Take(), index: index}, true
		},
		SizeHint: func(s rustvec.Ref[state]) rustvec.Hint {
			n := s.Value.length - s.Value.next
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Drop: func(s *state, c Context) { s.function.CloseWithContext(c) },
	}
	input := rustvec.NewIterator(state{length: length, value: value, function: function.MoveWithContext(c)}, iteratorOps)
	drop := rustvec.ElementOps[cachedIndex[K]]{Drop: func(pair *cachedIndex[K], c Context) {
		if ops.Drop != nil {
			ops.Drop(&pair.key, c)
		}
	}}
	collectConfig := rustvec.DefaultConfig(pairType.Size)
	collectConfig.Context = c
	indices := rustvec.Collect(rustvec.FromIterator(input), collectConfig, drop)
	defer cleanupKeyVector(rustvec.NewVecIntoIter(indices, drop), c)
	Unstable(indices, pairType, func(a, b *cachedIndex[K], c Context) bool {
		var result ChainingLess
		if ops.ChainingLess != nil {
			result = ops.ChainingLess(&a.key, &b.key, c)
		} else {
			if ops.PartialCompare == nil {
				c.Panic("rustsort: missing key PartialOrd implementation")
			}
			ordering, some := ops.PartialCompare(&a.key, &b.key, c)
			result.Continue = some && ordering == Equal
			result.Less = some && ordering == LessThan
		}
		if result.Continue {
			return a.index < b.index
		}
		return result.Less
	}, c)
	for i := range indices {
		index := indices[i].index
		for index < uint(i) {
			index = indices[index].index
		}
		indices[i].index = index
		// The original swap is a no-op for the source's zero-sized T.
	}
}
