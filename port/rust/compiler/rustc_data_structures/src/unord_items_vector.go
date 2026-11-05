// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordVectorCollector retains the actual selected I::collect::<Vec<T>>
// operation and the source element layout/Freeze/Copy traits. Existing genuine
// IndexVec storage represents the source Vec, including ZST lengths/capacities.
// Collect is a static monomorphized operation, including its selected std
// specialization and complete local cleanup, not an arbitrary user closure.
type UnordVectorCollector[S, T any] struct {
	Collect func(*rustvec.Owned[S], rustfmt.Context) *rustcindex.IndexVec[uint, T]
	Type    rustsort.Type
}

func (selected UnordVectorCollector[S, T]) forSorting() UnordSortCollector[S, T, *rustcindex.IndexVec[uint, T]] {
	return UnordSortCollector[S, T, *rustcindex.IndexVec[uint, T]]{
		Collect: func(state *rustvec.Owned[S], c rustfmt.Context) *rustvec.Owned[*rustcindex.IndexVec[uint, T]] {
			if selected.Collect == nil {
				c.Panic("datastructures: missing selected Vec collection implementation")
			}
			vector := selected.Collect(state, c)
			if vector == nil {
				c.Panic("datastructures: selected Vec collector returned nil")
			}
			return rustvec.NewOwned(vector, rustvec.ValueOps[*rustcindex.IndexVec[uint, T]]{
				Drop: func(vector **rustcindex.IndexVec[uint, T], c rustfmt.Context) { (*vector).CloseWithRuntimeContext(c) },
			})
		},
		BorrowMut: func(ref rustvec.Ref[*rustcindex.IndexVec[uint, T]]) UnordSortableSlice[T] {
			vector := *ref.Value
			slice := UnordSortableSlice[T]{Length: vector.Len(), Type: selected.Type}
			if selected.Type.Size == 0 {
				if vector.Len() != 0 {
					slice.Zero = vector.Raw.At(0)
				}
			} else {
				slice.Values = vector.Raw.AsGoSlice()
			}
			return slice
		},
	}
}

func IntoSortedUnordItems[S, T, K any](items *UnordItems[T], hcx StableHashCtxt, selected UnordVectorCollector[S, T], ops UnordStableKeyOps[T, K], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	return CollectUnordItemsSorted(items, hcx, true, selected.forSorting(), ops, contexts...).Take()
}

func IntoSortedUnordItemsStableOrdByKey[S, T, K any](items *UnordItems[T], project UnordFunction[rustvec.Ref[T], rustvec.Ref[K]], selected UnordVectorCollector[S, T], key UnordStableCompare[K], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	return CollectUnordItemsStableOrdByKey(items, project, selected.forSorting(), key, contexts...).Take()
}

func unordIdentityProjection[T any]() UnordFunction[rustvec.Ref[T], rustvec.Ref[T]] {
	invoke := func(_ rustvec.Ref[struct{}], argument *rustvec.Owned[rustvec.Ref[T]]) *rustvec.Owned[rustvec.Ref[T]] {
		return rustvec.Own(*argument.Borrow())
	}
	return NewUnordFunction(rustvec.Own(struct{}{}), UnordFunctionOps[struct{}, rustvec.Ref[T], rustvec.Ref[T]]{
		Call: invoke, CallMut: invoke,
		CallOnce: func(state *rustvec.Owned[struct{}], argument *rustvec.Owned[rustvec.Ref[T]], c rustfmt.Context) *rustvec.Owned[rustvec.Ref[T]] {
			return invoke(rustvec.Ref[struct{}]{Value: state.Borrow(), Context: c}, argument)
		},
	})
}

func IntoSortedUnordItemsStableOrd[S, T any](items *UnordItems[T], selected UnordVectorCollector[S, T], key UnordStableCompare[T], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	return IntoSortedUnordItemsStableOrdByKey(items, unordIdentityProjection[T](), selected, key, contexts...)
}

// UnordOwnedVectorCollector is the actual Vec::IntoIter<T> collection
// specialization. It composes existing Index storage and preserves allocation
// reuse and capacity decisions; it does not manufacture next/hint callbacks.
func UnordOwnedVectorCollector[T any](config UnordBagConfig[T], typ rustsort.Type) UnordVectorCollector[*rustcindex.OwnedIterator[T], T] {
	return UnordVectorCollector[*rustcindex.OwnedIterator[T], T]{Type: typ,
		Collect: func(state *rustvec.Owned[*rustcindex.OwnedIterator[T]], c rustfmt.Context) *rustcindex.IndexVec[uint, T] {
			input := state.Take().IntoIterator()
			return rustcindex.IndexVecFromIteratorWithConfigAndContext(input, rustcindex.UsizeIdx(), config.indexOps(), config.Profile, c)
		},
	}
}
