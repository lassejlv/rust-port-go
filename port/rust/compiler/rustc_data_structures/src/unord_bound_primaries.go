// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"reflect"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// These borrowed dictionaries establish the complete source bounds, including
// required methods which the selected body never calls. They contain no extra
// owned Fn environment or destructor. Their methods describe the same lawful
// source type as the independently selected iterator/collection operations.
// Unused methods are never probed, invoked, or nil-validated by these primaries.
// The legacy selected-operation APIs remain conveniences with the same source
// type obligations on their callers.

// UnordDefaultValue witnesses Default::default's actual owned result, including
// that result's complete Drop. An empty derived Bag default does not call it.
type UnordDefaultValue[T any] interface {
	Default(rustfmt.Context) *rustvec.Owned[T]
}

// UnordCloneValue is the required Clone::clone method. Derived UnordItems Clone
// additionally requires this T:Clone bound but calls only its actual I::clone.
type UnordCloneValue[T any] interface {
	Clone(*T, rustfmt.Context) *rustvec.Owned[T]
}

// UnordCopyValue witnesses the actual unsafe Copy marker and its Clone
// supertrait. The marker is metadata and is never invoked. The source type must
// obey Copy's laws, including absence of a destructor; copying invokes no Clone.
type UnordCopyValue[T any] interface {
	UnordCloneValue[T]
	UnordCopyMarker()
}

// UnordOrdValue retains required PartialEq::eq, PartialOrd::partial_cmp, and
// Ord::cmp, plus Eq's marker laws. Independently overridden lt, chaining_lt,
// max/min, reduce and fold remain in the existing used-operation dictionaries.
type UnordOrdValue[T any] interface {
	Equal(*T, *T, rustfmt.Context) bool
	PartialCompare(*T, *T, rustfmt.Context) (rustsort.Ordering, bool)
	Compare(*T, *T, rustfmt.Context) rustsort.Ordering
}

// UnordStableHashKeyValue is ToStableHashKey::KeyType's complete Ord+StableHash
// bound. Sized is witnessed by its actual finite source KeyType layout in Used;
// zero-sized inhabited types are lawful. A Rust unsized type is not a lawful
// source layout for this dictionary. StableHash is unused by key sorting.
type UnordStableHashKeyValue[K any] interface {
	UnordOrdValue[K]
	StableHash(*K, StableHashCtxt, *StableHasher, rustfmt.Context)
}

// UnordStableKeyBounds retains every used projection/order/Drop slot unchanged
// and the complete associated-value bound. Value is borrowed static metadata;
// its required methods are not substituted for independent used overrides.
type UnordStableKeyBounds[T, K any] struct {
	Used  UnordStableKeyOps[T, K]
	Value UnordStableHashKeyValue[K]
}

// UnordSumValue and UnordProductValue are the required R trait methods
// monomorphized at actual concrete S: Iterator<Item=T>. They are independent
// from I::sum/product overrides. Static coherence with all trait invocations
// inside that selected override remains a lawful source-type obligation, as
// for the other monomorphized dictionaries. No owned capture is introduced.
type UnordSumValue[S, T, R any] interface {
	Sum(*rustvec.Owned[S], rustfmt.Context) *rustvec.Owned[R]
}
type UnordProductValue[S, T, R any] interface {
	Product(*rustvec.Owned[S], rustfmt.Context) *rustvec.Owned[R]
}

func DefaultUnordBagWithBounds[T any](config UnordBagConfig[T], _ UnordDefaultValue[T], contexts ...rustfmt.Context) *UnordBag[T] {
	return NewUnordBag(config, contexts...)
}
func CloneUnordItemsWithBounds[T any](items *UnordItems[T], _ UnordCloneValue[T], contexts ...rustfmt.Context) *UnordItems[T] {
	return items.Clone(contexts...)
}
func CloneFromUnordItemsWithBounds[T any](items, source *UnordItems[T], _ UnordCloneValue[T], contexts ...rustfmt.Context) {
	items.CloneFrom(source, contexts...)
}

// Both concrete Self and destination Clone exposure are checked before cloning
// the source or destroying/replacing the destination. reflect.TypeOf observes
// the actual instantiated private cell[S,T], not comparable value data. Source
// aliases share S; distinct named/embedding iterator states have distinct S.
func unordRequireCloneFromSelf[T any](items, source *UnordItems[T], c rustfmt.Context) {
	if !items.clonePublic {
		c.Panic("datastructures: source UnordItems bounds do not expose Clone")
	}
	if reflect.TypeOf(items.owner) != reflect.TypeOf(source.owner) {
		c.Panic("datastructures: CloneFrom requires the same concrete Iterator Self")
	}
}

func MaxUnordItemsWithBounds[S, T any](items *UnordItems[T], selected UnordIteratorReduction[S, T, T], _ UnordOrdValue[T], contexts ...rustfmt.Context) *rustvec.Owned[T] {
	return MaxUnordItems(items, selected, contexts...)
}
func MinUnordItemsWithBounds[S, T any](items *UnordItems[T], selected UnordIteratorReduction[S, T, T], _ UnordOrdValue[T], contexts ...rustfmt.Context) *rustvec.Owned[T] {
	return MinUnordItems(items, selected, contexts...)
}
func SumUnordItemsWithBounds[S, T, R any](items *UnordItems[T], selected UnordIteratorReduction[S, T, R], _ UnordSumValue[S, T, R], contexts ...rustfmt.Context) *rustvec.Owned[R] {
	return SumUnordItems(items, selected, contexts...)
}
func ProductUnordItemsWithBounds[S, T, R any](items *UnordItems[T], selected UnordIteratorReduction[S, T, R], _ UnordProductValue[S, T, R], contexts ...rustfmt.Context) *rustvec.Owned[R] {
	return ProductUnordItems(items, selected, contexts...)
}
func ClonedUnordItemsWithBounds[S, T, R any](items *UnordItems[rustvec.Ref[T]], selected UnordUnaryIteratorConstructor[S, rustvec.Ref[T], R, T], _ UnordCloneValue[T], contexts ...rustfmt.Context) *UnordItems[T] {
	return ClonedUnordItems(items, selected, contexts...)
}
func CopiedUnordItemsWithBounds[S, T, R any](items *UnordItems[rustvec.Ref[T]], selected UnordUnaryIteratorConstructor[S, rustvec.Ref[T], R, T], _ UnordCopyValue[T], contexts ...rustfmt.Context) *UnordItems[T] {
	return CopiedUnordItems(items, selected, contexts...)
}
func ClonedBorrowedUnordItemsWithBounds[S, T, R any](items *UnordItems[*T], selected UnordUnaryIteratorConstructor[S, *T, R, T], _ UnordCloneValue[T], contexts ...rustfmt.Context) *UnordItems[T] {
	return ClonedBorrowedUnordItems(items, selected, contexts...)
}
func CopiedBorrowedUnordItemsWithBounds[S, T, R any](items *UnordItems[*T], selected UnordUnaryIteratorConstructor[S, *T, R, T], _ UnordCopyValue[T], contexts ...rustfmt.Context) *UnordItems[T] {
	return CopiedBorrowedUnordItems(items, selected, contexts...)
}
func CollectUnordItemsSortedWithBounds[S, T, C, K any](items *UnordItems[T], hcx StableHashCtxt, cache bool, selected UnordSortCollector[S, T, C], key UnordStableKeyBounds[T, K], contexts ...rustfmt.Context) *rustvec.Owned[C] {
	return CollectUnordItemsSorted(items, hcx, cache, selected, key.Used, contexts...)
}
func IntoSortedUnordItemsWithBounds[S, T, K any](items *UnordItems[T], hcx StableHashCtxt, selected UnordVectorCollector[S, T], key UnordStableKeyBounds[T, K], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	return IntoSortedUnordItems(items, hcx, selected, key.Used, contexts...)
}
func UnordMapToSortedWithBounds[K, V, U any](collection *UnordMap[K, V], hcx StableHashCtxt, cache bool, key UnordStableKeyBounds[K, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, rusthashmap.BorrowedPair[K, V]] {
	return UnordMapToSorted(collection, hcx, cache, key.Used, contexts...)
}
func UnordMapIntoSortedWithBounds[K, V, U any](collection *UnordMap[K, V], hcx StableHashCtxt, cache bool, typ rustsort.Type, key UnordStableKeyBounds[K, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, rusthashmap.Pair[K, V]] {
	return UnordMapIntoSorted(collection, hcx, cache, typ, key.Used, contexts...)
}
func UnordSetToSortedWithBounds[T, U any](collection *UnordSet[T], hcx StableHashCtxt, cache bool, key UnordStableKeyBounds[T, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, *T] {
	return UnordSetToSorted(collection, hcx, cache, key.Used, contexts...)
}
func UnordSetIntoSortedWithBounds[T, U any](collection *UnordSet[T], hcx StableHashCtxt, cache bool, typ rustsort.Type, key UnordStableKeyBounds[T, U], contexts ...rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	return UnordSetIntoSorted(collection, hcx, cache, typ, key.Used, contexts...)
}
func UnordMapValuesSortedWithBounds[K, V, U any](collection *UnordMap[K, V], hcx StableHashCtxt, cache bool, key UnordStableKeyBounds[K, U], contexts ...rustfmt.Context) rustvec.Iterator[*V] {
	return UnordMapValuesSorted(collection, hcx, cache, key.Used, contexts...)
}
