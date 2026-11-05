// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// The source's concrete std HashMap/HashSet iterators use Iterator's default
// Vec collect operation. Their actual next/hint/drop operations remain those
// of the real hash-table iterators; these are not replacement dictionaries for
// arbitrary I::collect overrides. The concrete iterator is moved exactly once.
func unordHashVectorCollector[S, T any](typ rustsort.Type, next func(*S, rustfmt.Context) (T, bool), length func(*S) uint, close func(*S, rustfmt.Context), drop func(*T, rustfmt.Context)) UnordVectorCollector[S, T] {
	return UnordVectorCollector[S, T]{Type: typ, Collect: func(owner *rustvec.Owned[S], c rustfmt.Context) *rustcindex.IndexVec[uint, T] {
		state := owner.Take()
		input := rustcindex.IndexIntoIterator[T]{IntoIterWithContext: func(rustfmt.Context) rustcindex.IndexValueIterator[T] {
			return rustcindex.IndexValueIterator[T]{
				NextWithContext:     func(c rustfmt.Context) (T, bool) { return next(&state, c) },
				SizeHintWithContext: func(rustfmt.Context) (uint, uint, bool) { n := length(&state); return n, n, true },
				CloseWithRuntimeContext: func(c rustfmt.Context) {
					if close != nil {
						close(&state, c)
					}
				},
			}
		}}
		return rustcindex.IndexVecFromIteratorWithConfigAndContext(input, rustcindex.UsizeIdx(), rustcindex.ValueOps[T]{RustSize: typ.Size, RustZeroSized: typ.Size == 0, DropWithRuntimeContext: drop}, rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, c)
	}}
}

func unordBorrowedMapCollector[K, V any](pointerBits uint) UnordVectorCollector[*rusthashmap.Iter[K, V], rusthashmap.BorrowedPair[K, V]] {
	return unordHashVectorCollector(rustsort.Type{Size: 2 * pointerBits / 8, Align: pointerBits / 8, Copy: true, Freeze: true},
		func(i **rusthashmap.Iter[K, V], c rustfmt.Context) (rusthashmap.BorrowedPair[K, V], bool) {
			return (*i).Next(c)
		},
		func(i **rusthashmap.Iter[K, V]) uint { return (*i).Len() }, nil, nil)
}
func unordOwnedMapCollector[K, V any](config UnordMapConfig[K, V], typ rustsort.Type) UnordVectorCollector[*rusthashmap.IntoIter[K, V], rusthashmap.Pair[K, V]] {
	return unordHashVectorCollector(typ,
		func(i **rusthashmap.IntoIter[K, V], c rustfmt.Context) (rusthashmap.Pair[K, V], bool) {
			return (*i).Next(c)
		},
		func(i **rusthashmap.IntoIter[K, V]) uint { return (*i).Len() },
		func(i **rusthashmap.IntoIter[K, V], c rustfmt.Context) { (*i).Close(c) },
		func(pair *rusthashmap.Pair[K, V], c rustfmt.Context) { dropUnordPair(pair, config, c) })
}
func unordBorrowedSetCollector[T any](pointerBits uint) UnordVectorCollector[*rusthashmap.SetIter[T], *T] {
	return unordHashVectorCollector(rustsort.Type{Size: pointerBits / 8, Align: pointerBits / 8, Copy: true, Freeze: true},
		func(i **rusthashmap.SetIter[T], c rustfmt.Context) (*T, bool) { return (*i).Next(c) },
		func(i **rusthashmap.SetIter[T]) uint { return (*i).Len() }, nil, nil)
}
func unordOwnedSetCollector[T any](config UnordSetConfig[T], typ rustsort.Type) UnordVectorCollector[*rusthashmap.SetIntoIter[T], T] {
	return unordHashVectorCollector(typ,
		func(i **rusthashmap.SetIntoIter[T], c rustfmt.Context) (T, bool) { return (*i).Next(c) },
		func(i **rusthashmap.SetIntoIter[T]) uint { return (*i).Len() },
		func(i **rusthashmap.SetIntoIter[T], c rustfmt.Context) { (*i).Close(c) }, config.Values.Drop)
}

func unordCollectionSort[S, T any](items *UnordItems[T], selected UnordVectorCollector[S, T], sort func(UnordSortableSlice[T], rustfmt.Context), c rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	collector := selected.forSorting()
	output := unordSortCollect(items, collector, c)
	return rustvec.ConsumeValue(output, c, func(output *rustvec.Owned[*rustcindex.IndexVec[uint, T]]) *rustcindex.IndexVec[uint, T] {
		slice := collector.BorrowMut(rustvec.Ref[*rustcindex.IndexVec[uint, T]]{Value: output.Borrow(), Context: c})
		slice.validate(c)
		sort(slice, c)
		return output.Take()
	})
}

func unordCollectionKeySort[T, K, U any](slice UnordSortableSlice[T], extract func(*T) *K, hcx StableHashCtxt, cache bool, ops UnordStableKeyOps[K, U], c rustfmt.Context) {
	projected := UnordStableKeyOps[T, U]{Project: func(value *T, hcx StableHashCtxt, c rustfmt.Context) *rustvec.Owned[U] {
		return ops.Project(extract(value), hcx, c)
	}, Less: ops.Less, KeyType: ops.KeyType, PointerBits: ops.PointerBits, Cached: ops.Cached}
	if cache {
		unordSortByStableKey(slice, hcx, true, projected, c)
		return
	}
	if slice.Length < 2 || slice.Type.Size == 0 {
		return
	}
	// to_sorted_vec's false branch is sort_unstable_by_key. Its two owned
	// temporary keys have the same right-then-left cleanup as sort_by_key.
	rustsort.Unstable(slice.Values, slice.Type, func(a, b *T, c rustfmt.Context) bool {
		return rustvec.ConsumeValue(projected.Project(a, hcx, c), c, func(left *rustvec.Owned[U]) bool {
			return rustvec.ConsumeValue(projected.Project(b, hcx, c), c, func(right *rustvec.Owned[U]) bool {
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
	}, c)
}

func unordCollectionStableOrd[T, K any](slice UnordSortableSlice[T], extract func(*T) *K, key UnordStableCompare[K], c rustfmt.Context) {
	if slice.Length < 2 || slice.Type.Size == 0 {
		return
	}
	// All four Map/Set StableCompare methods explicitly use unstable sort;
	// they do not branch on K::CAN_USE_UNSTABLE_SORT.
	rustsort.Unstable(slice.Values, slice.Type, func(a, b *T, c rustfmt.Context) bool {
		return key.Compare(extract(a), extract(b), c) == rustsort.LessThan
	}, c)
}

func UnordMapToSorted[K, V, U any](collection *UnordMap[K, V], hcx StableHashCtxt, cache bool, key UnordStableKeyOps[K, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, rusthashmap.BorrowedPair[K, V]] {
	c := unordContext(contexts)
	collection.check(c)
	return unordCollectionSort(collection.Items(c).General(c), unordBorrowedMapCollector[K, V](collection.config.Profile.PointerBits), func(slice UnordSortableSlice[rusthashmap.BorrowedPair[K, V]], c rustfmt.Context) {
		unordCollectionKeySort(slice, func(pair *rusthashmap.BorrowedPair[K, V]) *K { return pair.Key }, hcx, cache, key, c)
	}, c)
}
func UnordMapToSortedStableOrd[K, V any](collection *UnordMap[K, V], key UnordStableCompare[K], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, rusthashmap.BorrowedPair[K, V]] {
	c := unordContext(contexts)
	collection.check(c)
	return unordCollectionSort(collection.Items(c).General(c), unordBorrowedMapCollector[K, V](collection.config.Profile.PointerBits), func(slice UnordSortableSlice[rusthashmap.BorrowedPair[K, V]], c rustfmt.Context) {
		unordCollectionStableOrd(slice, func(pair *rusthashmap.BorrowedPair[K, V]) *K { return pair.Key }, key, c)
	}, c)
}
func UnordMapIntoSorted[K, V, U any](collection *UnordMap[K, V], hcx StableHashCtxt, cache bool, typ rustsort.Type, key UnordStableKeyOps[K, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, rusthashmap.Pair[K, V]] {
	c := unordContext(contexts)
	collection.check(c)
	selected := unordOwnedMapCollector(collection.config, typ)
	return unordCollectionSort(collection.IntoItems(c), selected, func(slice UnordSortableSlice[rusthashmap.Pair[K, V]], c rustfmt.Context) {
		unordCollectionKeySort(slice, func(pair *rusthashmap.Pair[K, V]) *K { return &pair.Key }, hcx, cache, key, c)
	}, c)
}
func UnordMapIntoSortedStableOrd[K, V any](collection *UnordMap[K, V], typ rustsort.Type, key UnordStableCompare[K], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, rusthashmap.Pair[K, V]] {
	c := unordContext(contexts)
	collection.check(c)
	selected := unordOwnedMapCollector(collection.config, typ)
	return unordCollectionSort(collection.IntoItems(c), selected, func(slice UnordSortableSlice[rusthashmap.Pair[K, V]], c rustfmt.Context) {
		unordCollectionStableOrd(slice, func(pair *rusthashmap.Pair[K, V]) *K { return &pair.Key }, key, c)
	}, c)
}
func UnordSetToSorted[T, U any](collection *UnordSet[T], hcx StableHashCtxt, cache bool, key UnordStableKeyOps[T, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, *T] {
	c := unordContext(contexts)
	collection.check(c)
	return unordCollectionSort(collection.Items(c).General(c), unordBorrowedSetCollector[T](collection.config.Profile.PointerBits), func(slice UnordSortableSlice[*T], c rustfmt.Context) {
		unordCollectionKeySort(slice, func(value **T) *T { return *value }, hcx, cache, key, c)
	}, c)
}
func UnordSetToSortedStableOrd[T any](collection *UnordSet[T], key UnordStableCompare[T], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, *T] {
	c := unordContext(contexts)
	collection.check(c)
	return unordCollectionSort(collection.Items(c).General(c), unordBorrowedSetCollector[T](collection.config.Profile.PointerBits), func(slice UnordSortableSlice[*T], c rustfmt.Context) {
		unordCollectionStableOrd(slice, func(value **T) *T { return *value }, key, c)
	}, c)
}
func UnordSetIntoSorted[T, U any](collection *UnordSet[T], hcx StableHashCtxt, cache bool, typ rustsort.Type, key UnordStableKeyOps[T, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	c := unordContext(contexts)
	collection.check(c)
	selected := unordOwnedSetCollector(collection.config, typ)
	return unordCollectionSort(collection.IntoItems(c), selected, func(slice UnordSortableSlice[T], c rustfmt.Context) {
		unordCollectionKeySort(slice, func(value *T) *T { return value }, hcx, cache, key, c)
	}, c)
}
func UnordSetIntoSortedStableOrd[T any](collection *UnordSet[T], typ rustsort.Type, key UnordStableCompare[T], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	c := unordContext(contexts)
	collection.check(c)
	selected := unordOwnedSetCollector(collection.config, typ)
	return unordCollectionSort(collection.IntoItems(c), selected, func(slice UnordSortableSlice[T], c rustfmt.Context) {
		unordCollectionStableOrd(slice, func(value *T) *T { return value }, key, c)
	}, c)
}
