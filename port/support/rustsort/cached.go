// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

type Ordering int8

const (
	LessThan    Ordering = -1
	Equal       Ordering = 0
	GreaterThan Ordering = 1
)

// ChainingLess is the exact PartialOrd::__chaining_less result. Continue is
// equality, which delegates comparison to the following tuple index. Otherwise
// Less is the final result, including Break(false) for an unordered comparison.
type ChainingLess struct{ Continue, Less bool }
type CachedKeyOps[K any] struct {
	Drop           func(*K, Context)
	PartialCompare func(*K, *K, Context) (Ordering, bool)
	// Nil selects only the actual trait default, which invokes partial_cmp
	// once. Supply this witness when the source type overrides the method.
	ChainingLess func(*K, *K, Context) ChainingLess
}
type CachedConfig struct {
	KeyType     Type
	PointerBits uint
}
type cachedIndex[K any] struct {
	key   K
	index uint
}

func tupleIndexType(key Type, indexBytes uint, c Context) Type {
	align := max(key.Align, indexBytes)
	if key.Size > ^uint(0)-indexBytes {
		c.Panic("capacity overflow")
	}
	size := key.Size + indexBytes
	if size > ^uint(0)-(align-1) {
		c.Panic("capacity overflow")
	}
	size = (size + align - 1) &^ (align - 1)
	return Type{Size: size, Align: align, Freeze: key.Freeze, Copy: key.Copy}
}
func cachedIndexType(length int, config CachedConfig, c Context) Type {
	key := config.KeyType
	if key.Align == 0 || key.Align&(key.Align-1) != 0 || key.Size%key.Align != 0 {
		c.Panic("rustsort: invalid source key layout")
	}
	if config.PointerBits != 32 && config.PointerBits != 64 {
		c.Panic("rustsort: unsupported source pointer width")
	}
	wide := tupleIndexType(key, config.PointerBits/8, c)
	narrow := tupleIndexType(key, 4, c)
	if narrow.Size < wide.Size && uint64(length) <= uint64(^uint32(0)) {
		return narrow
	}
	return wide
}
func cleanupKeyFunction[T, K any](function KeyFunction[T, K], c Context) {
	p := recover()
	if p != nil && c.ProcessingPanic {
		c.ResumeUnwind(p)
	}
	if p != nil {
		unwind := c
		unwind.Unwinding = true
		func() {
			defer func() {
				if recover() != nil {
					unwind.AbortCleanup()
				}
			}()
			function.CloseWithContext(unwind)
		}()
		c.ResumeUnwind(p)
	}
	function.CloseWithContext(c)
}
func cleanupKeyVector[K any](vector *rustvec.VecIntoIter[cachedIndex[K]], c Context) {
	p := recover()
	if p != nil && c.ProcessingPanic {
		c.ResumeUnwind(p)
	}
	if p != nil {
		unwind := c
		unwind.Unwinding = true
		func() {
			defer func() {
				if recover() != nil {
					unwind.AbortCleanup()
				}
			}()
			vector.CloseWithContext(unwind)
		}()
		c.ResumeUnwind(p)
	}
	vector.CloseWithContext(c)
}

// Cached implements actual Rust 1.101 slice::sort_by_cached_key, including its
// owned FnMut parameter, trusted key collection, tuple PartialOrd dispatch,
// source u32/usize index choice, stable permutation and key destruction.
func Cached[T, K any](values []T, function KeyFunction[T, K], config CachedConfig, ops CachedKeyOps[K], contexts ...Context) {
	var c Context
	if len(contexts) > 1 {
		panic("rustsort: expected at most one context")
	}
	if len(contexts) == 1 {
		c = contexts[0]
	}
	function = function.MoveWithContext(c)
	defer cleanupKeyFunction(function, c)
	// The source does not construct keys or select index storage for len<2,
	// but it still destroys the uncalled owned function parameter.
	if len(values) < 2 {
		return
	}
	pairType := cachedIndexType(len(values), config, c)
	type state struct {
		values   []T
		function KeyFunction[T, K]
		next     int
	}
	iteratorOps := rustvec.IteratorOps[state, cachedIndex[K]]{
		TrustedLength: true,
		Next: func(s rustvec.Ref[state]) (cachedIndex[K], bool) {
			if s.Value.next == len(s.Value.values) {
				return cachedIndex[K]{}, false
			}
			index := s.Value.next
			// Map obtains the next shared slice item before evaluating its FnMut.
			s.Value.next++
			key := s.Value.function.Call(&s.Value.values[index], s.Context)
			return cachedIndex[K]{key: key.Take(), index: uint(index)}, true
		},
		SizeHint: func(s rustvec.Ref[state]) rustvec.Hint {
			n := uint(len(s.Value.values) - s.Value.next)
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Drop: func(s *state, c Context) { s.function.CloseWithContext(c) },
	}
	input := rustvec.NewIterator(state{values: values, function: function.MoveWithContext(c)}, iteratorOps)
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
	for i := range values {
		index := indices[i].index
		for index < uint(i) {
			index = indices[index].index
		}
		indices[i].index = index
		values[i], values[index] = values[index], values[i]
	}
}
