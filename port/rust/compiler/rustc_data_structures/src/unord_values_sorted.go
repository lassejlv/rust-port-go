// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"unsafe"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordMapValuesSorted returns the source's ordered, owned
// Vec<(&K,&V)>::IntoIter.map(|(_,v)|v). Each result is a shared collection
// borrow, invalidated by incompatible mutation/move/drop. Rust 2024 also keeps
// the source Hcx borrow in the opaque return's lifetime; callers preserve that
// restriction even though no Hcx value is stored in the iterator.
func UnordMapValuesSorted[K, V, U any](collection *UnordMap[K, V], hcx StableHashCtxt, cache bool, key UnordStableKeyOps[K, U], contexts ...rustfmt.Context) rustvec.Iterator[*V] {
	vector := UnordMapToSorted(collection, hcx, cache, key, contexts...)
	return unordValuesSortedIterator(vector, unordContext(contexts))
}

// The vector is exclusively consumed here. No slice or iterator alias of its
// backing storage may survive this move; only the shared K/V pointees outlive it.
func unordValuesSortedIterator[K, V any](vector *rustcindex.IndexVec[uint, rusthashmap.BorrowedPair[K, V]], c rustfmt.Context) rustvec.Iterator[*V] {
	var pair rusthashmap.BorrowedPair[K, V]
	var pointer *V
	if unsafe.Sizeof(pair) != 2*unsafe.Sizeof(pointer) || unsafe.Alignof(pair) != unsafe.Alignof(pointer) || unsafe.Offsetof(pair.Key) != 0 || unsafe.Offsetof(pair.Value) != unsafe.Sizeof(pointer) {
		c.Panic("datastructures: invalid Go pointer-pair layout")
	}
	storage := vector.Raw.AsGoSlice()
	if cap(storage) > int(^uint(0)>>1)/2 {
		c.Panic("datastructures: pointer-pair capacity overflow")
	}
	var destination []*V
	if cap(storage) != 0 {
		// This is the concrete in-place Map<Vec::IntoIter<(&K,&V)>, _>
		// specialization: compact the second pointer of each source pair into
		// the same allocation. Both Go layouts contain only pointer slots, with
		// identical GC pointer maps. The bounded slice covers exactly the old
		// allocation, and each write is strictly behind the next unread pair.
		// This preserves actual reuse and the 2*source-capacity decision without
		// claiming a Go allocator or target ABI correspondence.
		destination = unsafe.Slice((**V)(unsafe.Pointer(unsafe.SliceData(storage))), 2*cap(storage))
	}
	type state struct {
		input       *rustcindex.OwnedIterator[rusthashmap.BorrowedPair[K, V]]
		destination []*V
	}
	owner := rustvec.NewOwned(state{input: vector.IntoIter(), destination: destination}, rustvec.ValueOps[state]{Drop: func(s *state, c rustfmt.Context) {
		if s.input != nil {
			s.input.CloseWithRuntimeContext(c)
		}
	}})
	next := func(ref rustvec.Ref[state]) (*V, bool) {
		pair, ok := ref.Value.input.Next()
		return pair.Value, ok
	}
	return rustvec.NewIteratorFromOwned(owner, rustvec.IteratorOps[state, *V]{
		Next: next,
		SizeHint: func(ref rustvec.Ref[state]) rustvec.Hint {
			n := ref.Value.input.Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		TrustedLength: true,
		// Map::fold delegates to IntoIter::fold while applying the zero-sized
		// field projection. There is no owning or observable projection Drop.
		FoldUnit: func(owner *rustvec.Owned[state], consume func(*V)) {
			for {
				value, ok := next(rustvec.Ref[state]{Value: owner.Borrow(), Context: owner.Context()})
				if !ok {
					return
				}
				consume(value)
			}
		},
		VecFromIterator: func(owner *rustvec.Owned[state], _ rustvec.Config, _ rustvec.ElementOps[*V]) []*V {
			s := owner.Borrow()
			result := s.destination[:0]
			for {
				value, ok := next(rustvec.Ref[state]{Value: s, Context: owner.Context()})
				if !ok {
					break
				}
				result = append(result, value)
			}
			// Drop the unused reference slots without touching the compacted
			// output. IntoIter has already consumed every logical source pair.
			clear(s.destination[len(result):])
			// The exhausted source iterator is deliberately forgotten: ownership
			// of its allocation has moved into result. Ordinary iterator Close
			// would clear that allocation, including the prepared output.
			s.input = nil
			s.destination = nil
			return result
		},
	})
}

// BorrowUnordSortedValues is the actual &mut Iterator view used by by_ref().
// It borrows the original iterator exclusively for its lifetime. Collection
// through this view has the TrustedLen strategy; the owning iterator alone has
// the Vec::IntoIter map's in-place strategy and allocation ownership.
func BorrowUnordSortedValues[V any](source rustvec.Iterator[*V]) rustvec.Iterator[*V] {
	return rustvec.NewIterator(source, rustvec.IteratorOps[rustvec.Iterator[*V], *V]{
		Next:          func(ref rustvec.Ref[rustvec.Iterator[*V]]) (*V, bool) { return ref.Value.Next(ref.Context) },
		SizeHint:      func(ref rustvec.Ref[rustvec.Iterator[*V]]) rustvec.Hint { return ref.Value.SizeHint(ref.Context) },
		TrustedLength: true,
	})
}
