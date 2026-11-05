// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordSetFromFxSet moves the actual FxHashSet allocation, without rebuilding,
// reserving, hashing or comparing its elements. Config must describe that exact
// source set's layout, target profile and traits, including FxBuildHasher.
// Source owns the complete Set destructor; all other aliases remain invalid.
func UnordSetFromFxSet[T any](source *rustvec.Owned[*rusthashmap.Set[T]], config UnordSetConfig[T], contexts ...rustfmt.Context) *UnordSet[T] {
	c := unordContext(contexts)
	return rustvec.ConsumeValue(source, c, func(source *rustvec.Owned[*rusthashmap.Set[T]]) *UnordSet[T] {
		return &UnordSet[T]{inner: source.Take(), config: config, live: true}
	})
}

// UnordCollectionIteratorOps retains I's actual next/hint and independently
// selected for_each/fold operations. Scoped slots are primary when the input
// may call its borrowed consumer under a nested current context. Legacy slots
// select only the actual default or a same-context concrete implementation.
type UnordCollectionIteratorOps[S, T any] struct {
	Iterator rustvec.IteratorOps[S, T]
	Scoped   rustvec.ScopedIteratorOps[S, T]
}

// The iterator dictionary in these conversions is I's actual selected
// Next/SizeHint/ForEach/Fold implementation. ForEach and FoldUnit are separate:
// HashMap's Extend calls I::for_each, whereas HashSet's map adapter calls
// I::fold. Nil slots select only those methods' real source trait defaults.
// State already owns I's complete destructor; no operation calls I::collect.
func UnordMapFromItems[S, K, V any](items *UnordItems[rusthashmap.Pair[K, V]], iterator UnordCollectionIteratorOps[S, rusthashmap.Pair[K, V]], config UnordMapConfig[K, V], contexts ...rustfmt.Context) *UnordMap[K, V] {
	c := unordContext(contexts)
	state := unordTakeConcrete[S](items, c)
	return rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *UnordMap[K, V] {
		input := rustvec.FromIterator(rustvec.NewScopedIteratorFromOwned(state, iterator.Iterator, iterator.Scoped))
		inner := rusthashmap.MapFromIteratorScoped(input, config.Layout, config.Profile, config.Keys, config.Values, func(rustfmt.Context) rusthashmap.Builder {
			return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
		}, c)
		return &UnordMap[K, V]{inner: inner, config: config, live: true}
	})
}
func UnordSetFromItems[S, T any](items *UnordItems[T], iterator UnordCollectionIteratorOps[S, T], config UnordSetConfig[T], contexts ...rustfmt.Context) *UnordSet[T] {
	c := unordContext(contexts)
	state := unordTakeConcrete[S](items, c)
	return rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *UnordSet[T] {
		input := rustvec.FromIterator(rustvec.NewScopedIteratorFromOwned(state, iterator.Iterator, iterator.Scoped))
		inner := rusthashmap.SetFromIteratorScoped(input, config.Layout, config.Profile, config.Values, func(rustfmt.Context) rusthashmap.Builder {
			return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
		}, c)
		return &UnordSet[T]{inner: inner, config: config, live: true}
	})
}
func UnordMapExtendItems[S, K, V any](collection *UnordMap[K, V], items *UnordItems[rusthashmap.Pair[K, V]], iterator UnordCollectionIteratorOps[S, rusthashmap.Pair[K, V]], contexts ...rustfmt.Context) {
	ExtendUnordItems(collection, items, func(collection *UnordMap[K, V], state *rustvec.Owned[S], c rustfmt.Context) {
		collection.check(c)
		collection.inner.ExtendScoped(rustvec.FromIterator(rustvec.NewScopedIteratorFromOwned(state, iterator.Iterator, iterator.Scoped)), c)
	}, contexts...)
}
func UnordSetExtendItems[S, T any](collection *UnordSet[T], items *UnordItems[T], iterator UnordCollectionIteratorOps[S, T], contexts ...rustfmt.Context) {
	ExtendUnordItems(collection, items, func(collection *UnordSet[T], state *rustvec.Owned[S], c rustfmt.Context) {
		collection.check(c)
		collection.inner.ExtendScoped(rustvec.FromIterator(rustvec.NewScopedIteratorFromOwned(state, iterator.Iterator, iterator.Scoped)), c)
	}, contexts...)
}

// UnordBagFromIteratorOps selects Vec<T>::from_iter::<I>, independently from
// an override of I::collect::<Vec<T>>. It includes the actual concrete std
// specialization, including Vec::IntoIter allocation reuse or an in-place
// adapter's collection. It must not replace that strategy with Next callbacks.
// The selected static operation owns I and includes complete local cleanup.
type UnordBagFromIteratorOps[S, T any] struct {
	FromIterator func(*rustvec.Owned[S], rustfmt.Context) *rustcindex.IndexVec[uint, T]
}

func UnordBagFromItems[S, T any](items *UnordItems[T], selected UnordBagFromIteratorOps[S, T], config UnordBagConfig[T], contexts ...rustfmt.Context) *UnordBag[T] {
	c := unordContext(contexts)
	state := unordTakeConcrete[S](items, c)
	inner := rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *rustcindex.IndexVec[uint, T] {
		if selected.FromIterator == nil {
			c.Panic("datastructures: missing selected Vec::from_iter implementation")
		}
		return selected.FromIterator(state, c)
	})
	return &UnordBag[T]{inner: inner, config: config, live: true}
}

// The concrete Vec::IntoIter strategy is the existing genuine Index storage
// specialization. Its identical Iterator::collect default is not overridden.
func UnordBagOwnedFromIterator[T any](config UnordBagConfig[T]) UnordBagFromIteratorOps[*rustcindex.OwnedIterator[T], T] {
	return UnordBagFromIteratorOps[*rustcindex.OwnedIterator[T], T]{FromIterator: func(state *rustvec.Owned[*rustcindex.OwnedIterator[T]], c rustfmt.Context) *rustcindex.IndexVec[uint, T] {
		return rustcindex.IndexVecFromIteratorWithConfigAndContext(state.Take().IntoIterator(), rustcindex.UsizeIdx(), config.indexOps(), config.Profile, c)
	}}
}

// UnordBagExtendIteratorOps is the corresponding selected Vec::extend::<I>
// specialization. It receives the actual Vec borrow and the owned actual I.
type UnordBagExtendIteratorOps[S, T any] struct {
	Extend func(*rustcindex.IndexVec[uint, T], *rustvec.Owned[S], rustfmt.Context)
}

func UnordBagExtendItems[S, T any](collection *UnordBag[T], items *UnordItems[T], selected UnordBagExtendIteratorOps[S, T], contexts ...rustfmt.Context) {
	ExtendUnordItems(collection, items, func(collection *UnordBag[T], state *rustvec.Owned[S], c rustfmt.Context) {
		collection.check(c)
		if selected.Extend == nil {
			c.Panic("datastructures: missing selected Vec::extend implementation")
		}
		selected.Extend(collection.inner, state, c)
	}, contexts...)
}
func UnordBagOwnedExtendIterator[T any]() UnordBagExtendIteratorOps[*rustcindex.OwnedIterator[T], T] {
	return UnordBagExtendIteratorOps[*rustcindex.OwnedIterator[T], T]{Extend: func(vector *rustcindex.IndexVec[uint, T], state *rustvec.Owned[*rustcindex.OwnedIterator[T]], c rustfmt.Context) {
		vector.ExtendWithContext(state.Take().IntoIterator(), c)
	}}
}
