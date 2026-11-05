// SPDX-License-Identifier: MIT OR Apache-2.0
// Stable-hashing operations from unord.rs. General collection operations remain
// partial; this adds no credit for the surrounding stable_hash.rs dependency.
package datastructures

import "github.com/lassejlv/rust-port-go/port/support/rusthashmap"

// UnordStableHash is a monomorphized StableHash trait method. It temporarily
// borrows the actual value, mutable hcx and concrete source hasher. The method
// dictionary is borrowed metadata, not an additional owned Fn environment.
// Translated panic sites must use the supplied context; raw Go callback-local
// defers retain Go timing before an escaping panic reaches the boundary.
type UnordStableHash[T any] func(*T, StableHashCtxt, *StableHasher, rusthashmap.Context)

func unordHashBoundary(context rusthashmap.Context) {
	if p := recover(); p != nil {
		context.ResumeUnwind(p)
	}
}

// next is a private static borrow of a concrete ExactSizeIterator. These used
// map/set iterators contain no destructible captures or element ownership.
func hashUnordBorrowed(length uint, next func() bool, hashCurrent func(*StableHasher), hasher *StableHasher) {
	// usize StableHash delegates to Hash; StableHasher normalizes it to u64.
	hasher.WriteUsize(length)
	switch length {
	case 0:
		return
	case 1:
		next()
		hashCurrent(hasher)
	default:
		accumulator := ZeroFingerprint()
		for next() {
			itemHasher := NewStableHasher()
			hashCurrent(itemHasher)
			fingerprint := itemHasher.FinishHash()
			accumulator = accumulator.CombineCommutative(FingerprintFromU64(fingerprint[0], fingerprint[1]))
		}
		accumulator.StableHash(nil, hasher)
	}
}

func (m *UnordMap[K, V]) StableHash(hcx StableHashCtxt, hasher *StableHasher, keyHash UnordStableHash[K], valueHash UnordStableHash[V], contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	m.check(c)
	if keyHash == nil || valueHash == nil {
		c.Panic("datastructures: missing UnordMap StableHash implementation")
	}
	i := m.inner.Iter(c)
	var current rusthashmap.BorrowedPair[K, V]
	hashUnordBorrowed(i.Len(), func() bool {
		var ok bool
		current, ok = i.Next(c)
		return ok
	}, func(target *StableHasher) {
		// Tuple StableHash borrows and hashes its first field before its second.
		keyHash(current.Key, hcx, target, c)
		valueHash(current.Value, hcx, target, c)
	}, hasher)
}

func (s *UnordSet[T]) StableHash(hcx StableHashCtxt, hasher *StableHasher, valueHash UnordStableHash[T], contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	s.check(c)
	if valueHash == nil {
		c.Panic("datastructures: missing UnordSet StableHash implementation")
	}
	i := s.inner.Iter(c)
	var current *T
	hashUnordBorrowed(i.Len(), func() bool {
		var ok bool
		current, ok = i.Next(c)
		return ok
	}, func(target *StableHasher) { valueHash(current, hcx, target, c) }, hasher)
}
