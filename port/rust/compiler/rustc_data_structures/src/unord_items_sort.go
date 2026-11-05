// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordSortableSlice is C's actual exclusive BorrowMut<[T]> result. Nonzero
// source elements use their actual mutable Go storage; source ZSTs retain the
// full usize length and one actual shared value borrow without materializing an
// impossible []T. Type is the genuine source layout and Freeze/Copy metadata.
// No slice/element borrow may escape this operation or alias mutation through C.
type UnordSortableSlice[T any] struct {
	Length uint
	Values []T
	Zero   *T
	Type   rustsort.Type
}

func (slice UnordSortableSlice[T]) validate(c rustfmt.Context) {
	if slice.Type.Align == 0 || slice.Type.Align&(slice.Type.Align-1) != 0 || slice.Type.Size%slice.Type.Align != 0 {
		c.Panic("datastructures: invalid actual source slice layout")
	}
	if slice.Type.Size != 0 && slice.Length != uint(len(slice.Values)) {
		c.Panic("datastructures: source slice storage length differs")
	}
	if slice.Type.Size == 0 && slice.Length != 0 && slice.Zero == nil {
		c.Panic("datastructures: missing actual source ZST slice borrow")
	}
}

// UnordSortCollector is the actual selected I::collect::<C> operation (whose
// default calls C::from_iter) and C's BorrowMut implementation. Independent
// Iterator::collect overrides and concrete std Vec specializations must remain
// selected. Collect owns I and includes its complete local/source cleanup;
// a prepared C is leaked if subsequent iterator parameter destruction panics.
// The dictionaries are static operations, with no hidden owning captures.
type UnordSortCollector[S, T, C any] struct {
	Collect   func(*rustvec.Owned[S], rustfmt.Context) *rustvec.Owned[C]
	BorrowMut func(rustvec.Ref[C]) UnordSortableSlice[T]
}

// UnordStableKeyOps is the actual ToStableHashKey/KeyType trait selection.
// KeyType satisfies the source Ord+StableHash bounds. Project temporarily
// borrows T and mutable Hcx and returns an owned KeyType with complete Drop;
// Less is the actual PartialOrd::lt used by sort_by_key, independently from
// cached tuple PartialOrd dispatch. Nil selects its default partial_cmp-based
// implementation using Cached.PartialCompare; it does not call Ord::cmp.
// Cached.Drop must match the returned key owners' complete destructor.
type UnordStableKeyOps[T, K any] struct {
	Project     func(*T, StableHashCtxt, rustfmt.Context) *rustvec.Owned[K]
	Less        func(*K, *K, rustfmt.Context) bool
	KeyType     rustsort.Type
	PointerBits uint
	Cached      rustsort.CachedKeyOps[K]
}

func unordSortCollect[S, T, C any](items *UnordItems[T], selected UnordSortCollector[S, T, C], c rustfmt.Context) *rustvec.Owned[C] {
	if selected.Collect == nil || selected.BorrowMut == nil {
		c.Panic("datastructures: missing selected collect/BorrowMut implementation")
	}
	state := unordTakeConcrete[S](items, c)
	output := rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *rustvec.Owned[C] { return selected.Collect(state, c) })
	return unordFunctionResult(output, c)
}

func unordSortByStableKey[T, K any](slice UnordSortableSlice[T], hcx StableHashCtxt, cache bool, ops UnordStableKeyOps[T, K], c rustfmt.Context) {
	if slice.Length < 2 {
		return
	}
	if ops.Project == nil {
		c.Panic("datastructures: missing ToStableHashKey implementation")
	}
	if cache {
		// The source closure owns only a borrowed mutable Hcx, with trivial
		// destruction. The actual key owner and caller's current scope remain
		// explicit at each invocation; no runtime scope is stored in Hcx.
		function := rustsort.NewKeyFunction(rustvec.Own(hcx), func(hcx *StableHashCtxt, value *T, c rustfmt.Context) *rustvec.Owned[K] {
			return ops.Project(value, *hcx, c)
		})
		config := rustsort.CachedConfig{KeyType: ops.KeyType, PointerBits: ops.PointerBits}
		if slice.Type.Size == 0 {
			rustsort.CachedZeroSized(slice.Length, slice.Zero, function, config, ops.Cached, c)
		} else {
			rustsort.Cached(slice.Values, function, config, ops.Cached, c)
		}
		return
	}
	// sort_by_key's two temporary keys are destroyed right, then left, both
	// on success and when the comparison/right-key construction unwinds.
	less := func(a, b *T, c rustfmt.Context) bool {
		left := ops.Project(a, hcx, c)
		return rustvec.ConsumeValue(left, c, func(left *rustvec.Owned[K]) bool {
			right := ops.Project(b, hcx, c)
			return rustvec.ConsumeValue(right, c, func(right *rustvec.Owned[K]) bool {
				if ops.Less != nil {
					return ops.Less(left.Borrow(), right.Borrow(), c)
				}
				if ops.Cached.PartialCompare == nil {
					c.Panic("datastructures: missing key PartialOrd implementation")
				}
				ordering, some := ops.Cached.PartialCompare(left.Borrow(), right.Borrow(), c)
				return some && ordering == rustsort.LessThan
			})
		})
	}
	if slice.Type.Size != 0 {
		// The generic UnordItems false branch is STABLE sort_by_key. The
		// private Map/Set helper deliberately has a different unstable branch.
		rustsort.Stable(slice.Values, slice.Type, less, c)
	}
}

func CollectUnordItemsSorted[S, T, C, K any](items *UnordItems[T], hcx StableHashCtxt, cache bool, selected UnordSortCollector[S, T, C], ops UnordStableKeyOps[T, K], contexts ...rustfmt.Context) *rustvec.Owned[C] {
	c := unordContext(contexts)
	output := unordSortCollect(items, selected, c)
	return rustvec.ConsumeValue(output, c, func(output *rustvec.Owned[C]) *rustvec.Owned[C] {
		// BorrowMut is observable and must run even for an empty/single C.
		slice := selected.BorrowMut(rustvec.Ref[C]{Value: output.Borrow(), Context: c})
		slice.validate(c)
		unordSortByStableKey(slice, hcx, cache, ops, c)
		return output.MoveWithContext(rustfmt.Context{})
	})
}

type UnordStableCompare[K any] struct {
	CanUseUnstableSort bool
	Compare            func(*K, *K, rustfmt.Context) rustsort.Ordering
}

func CollectUnordItemsStableOrdByKey[S, T, C, K any](items *UnordItems[T], project UnordFunction[rustvec.Ref[T], rustvec.Ref[K]], selected UnordSortCollector[S, T, C], key UnordStableCompare[K], contexts ...rustfmt.Context) *rustvec.Owned[C] {
	c := unordContext(contexts)
	project = project.Move(c)
	defer unordFunctionParameterCleanup(project, c)
	output := unordSortCollect(items, selected, c)
	return rustvec.ConsumeValue(output, c, func(output *rustvec.Owned[C]) *rustvec.Owned[C] {
		slice := selected.BorrowMut(rustvec.Ref[C]{Value: output.Borrow(), Context: c})
		slice.validate(c)
		if slice.Length > 1 && slice.Type.Size != 0 {
			less := func(a, b *T, c rustfmt.Context) bool {
				// P:Fn is shared in the source comparison closure, including
				// when the sorting engine invokes that closure via FnMut.
				aKey := project.Call(rustvec.Own(rustvec.Ref[T]{Value: a, Context: c}), c).Take()
				bKey := project.Call(rustvec.Own(rustvec.Ref[T]{Value: b, Context: c}), c).Take()
				if key.Compare == nil {
					c.Panic("datastructures: missing StableCompare implementation")
				}
				return key.Compare(aKey.Value, bKey.Value, c) == rustsort.LessThan
			}
			if key.CanUseUnstableSort {
				rustsort.Unstable(slice.Values, slice.Type, less, c)
			} else {
				rustsort.Stable(slice.Values, slice.Type, less, c)
			}
		}
		// P's later parameter Drop may panic. At this point C is already
		// in the return place and must not be destroyed during that unwind.
		return output.MoveWithContext(rustfmt.Context{})
	})
}
