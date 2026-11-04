// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL foundation from rustc_data_structures/src/unord.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. These concrete map/read-items operations
// use the real scoped Fx/SwissTable backing. Generic iterator algebra, sorting,
// codecs and unordered stable hashing remain pending; no whole-file credit.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type UnordMapConfig[K, V any] struct {
	Layout  rusthashmap.Layout
	Profile rusthashmap.Profile
	Keys    rusthashmap.KeyOps[K]
	Values  rusthashmap.ValueOps[V]
}
type UnordMap[K, V any] struct {
	inner  *rusthashmap.Map[K, V]
	config UnordMapConfig[K, V]
	live   bool
}

func unordContext(contexts []rusthashmap.Context) rusthashmap.Context {
	if len(contexts) > 1 {
		panic("datastructures: expected at most one source runtime context")
	}
	if len(contexts) == 1 {
		return contexts[0]
	}
	return rusthashmap.Context{}
}
func NewUnordMap[K, V any](config UnordMapConfig[K, V], contexts ...rusthashmap.Context) *UnordMap[K, V] {
	return NewUnordMapWithCapacity(0, config, contexts...)
}
func NewUnordMapWithCapacity[K, V any](capacity uint, config UnordMapConfig[K, V], contexts ...rusthashmap.Context) *UnordMap[K, V] {
	c := unordContext(contexts)
	if config.Keys.HashOne != nil {
		c.Panic("datastructures: UnordMap's fixed FxBuildHasher uses the default hash_one")
	}
	return &UnordMap[K, V]{inner: rusthashmap.NewMapWithCapacity(capacity, config.Layout, config.Profile, config.Keys, config.Values, rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}, c), config: config, live: true}
}
func (m *UnordMap[K, V]) check(c rusthashmap.Context) {
	if m == nil || !m.live {
		c.Panic("datastructures: use of consumed UnordMap")
	}
}
func (m *UnordMap[K, V]) Len() uint     { m.check(rusthashmap.Context{}); return m.inner.Len() }
func (m *UnordMap[K, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *UnordMap[K, V]) Insert(key K, value V, contexts ...rusthashmap.Context) (V, bool) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.Insert(key, value, c)
}
func (m *UnordMap[K, V]) InsertOwned(key *rustvec.Owned[K], value *rustvec.Owned[V], contexts ...rusthashmap.Context) (*rustvec.Owned[V], bool) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.InsertOwned(key, value, c)
}
func (m *UnordMap[K, V]) TryInsert(key K, value V, contexts ...rusthashmap.Context) (*V, *rusthashmap.OccupiedError[K, V]) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.TryInsert(key, value, c)
}
func (m *UnordMap[K, V]) TryInsertOwned(key *rustvec.Owned[K], value *rustvec.Owned[V], contexts ...rusthashmap.Context) (*V, *rusthashmap.OccupiedError[K, V]) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.TryInsertOwned(key, value, c)
}
func (m *UnordMap[K, V]) Entry(key K, contexts ...rusthashmap.Context) *rusthashmap.Entry[K, V] {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.Entry(key, c)
}
func (m *UnordMap[K, V]) EntryOwned(key *rustvec.Owned[K], contexts ...rusthashmap.Context) *rusthashmap.Entry[K, V] {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.EntryOwned(key, c)
}

// Get returns a shared source borrow. GetMut returns an exclusive source borrow.
// Go's *V does not encode that distinction: callers must preserve it, and may
// retain either pointer only until an incompatible borrow/mutation/move/drop.
func (m *UnordMap[K, V]) Get(key *K, contexts ...rusthashmap.Context) (*V, bool) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.Get(key, c)
}
func (m *UnordMap[K, V]) GetMut(key *K, contexts ...rusthashmap.Context) (*V, bool) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.GetMut(key, c)
}
func (m *UnordMap[K, V]) ContainsKey(key *K, contexts ...rusthashmap.Context) bool {
	_, ok := m.Get(key, contexts...)
	return ok
}
func (m *UnordMap[K, V]) Remove(key *K, contexts ...rusthashmap.Context) (V, bool) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.Remove(key, c)
}
func (m *UnordMap[K, V]) RemoveOwned(key *K, contexts ...rusthashmap.Context) (*rustvec.Owned[V], bool) {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.RemoveOwned(key, c)
}
func (m *UnordMap[K, V]) Index(key *K, contexts ...rusthashmap.Context) *V {
	c := unordContext(contexts)
	m.check(c)
	return m.inner.Index(key, c)
}
func (m *UnordMap[K, V]) Clear(contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	m.check(c)
	m.inner.Clear(c)
}
func (m *UnordMap[K, V]) Close(contexts ...rusthashmap.Context) {
	if m != nil && m.live {
		m.live = false
		m.inner.Close(unordContext(contexts))
	}
}
func (m *UnordMap[K, V]) Clone(contexts ...rusthashmap.Context) *UnordMap[K, V] {
	c := unordContext(contexts)
	m.check(c)
	return &UnordMap[K, V]{inner: m.inner.Clone(c), config: m.config, live: true}
}
func (m *UnordMap[K, V]) CloneFrom(source *UnordMap[K, V], contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	m.check(c)
	source.check(c)
	if m == source {
		c.Panic("datastructures: CloneFrom requires disjoint source/destination borrows")
	}
	replacement := source.Clone(c)
	// Derived Clone inherits the whole-value assignment default. Rust installs
	// the replacement even if dropping the old destination starts unwinding.
	defer func() { *m = *replacement; replacement.live = false }()
	m.inner.Close(c)
}
func (m *UnordMap[K, V]) Equal(other *UnordMap[K, V], contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	m.check(c)
	other.check(c)
	return m.inner.Equal(other.inner, c)
}
func (m *UnordMap[K, V]) NotEqual(other *UnordMap[K, V], contexts ...rusthashmap.Context) bool {
	return !m.Equal(other, contexts...)
}

// UnordMapItems is an opaque owned iterator view borrowing its source map. It
// has no public Iterator/IntoIterator operation: source order stays behind the
// scoped collection operation below. Source mutation/Drop is forbidden while
// this view or any of its shared key/value borrows remains alive.
type UnordMapItems[K, V any] struct {
	iterator *rusthashmap.Iter[K, V]
	live     bool
}

func (m *UnordMap[K, V]) Items(contexts ...rusthashmap.Context) *UnordMapItems[K, V] {
	c := unordContext(contexts)
	m.check(c)
	return &UnordMapItems[K, V]{m.inner.Iter(c), true}
}
func (i *UnordMapItems[K, V]) Clone(contexts ...rusthashmap.Context) *UnordMapItems[K, V] {
	c := unordContext(contexts)
	if i == nil || !i.live {
		c.Panic("datastructures: use of consumed UnordMapItems")
	}
	return &UnordMapItems[K, V]{i.iterator.Clone(), true}
}
func (i *UnordMapItems[K, V]) Close(contexts ...rusthashmap.Context) {
	if i != nil {
		i.live = false
	}
}

// CollectUnordMapFilterMap is a scoped static-operation adapter for the actual
// .items().filter(predicate).map(copy_fields).collect() pipeline used by Span.
// Its lower hint is zero; source bucket order drives insertion/growth. combine
// performs the predicate before constructing any mapped pair. It may capture
// only trivial or borrowed state, and owns all cleanup of its constructed
// result/temporaries if it panics. Hidden destructible Fn captures are forbidden.
func CollectUnordMapFilterMap[K, V, KO, VO any](items *UnordMapItems[K, V], combine func(*K, *V, rusthashmap.Context) (rusthashmap.Pair[KO, VO], bool), config UnordMapConfig[KO, VO], contexts ...rusthashmap.Context) *UnordMap[KO, VO] {
	c := unordContext(contexts)
	if config.Keys.HashOne != nil {
		c.Panic("datastructures: UnordMap's fixed FxBuildHasher uses the default hash_one")
	}
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordMapItems")
	}
	items.live = false
	if combine == nil {
		c.Panic("datastructures: nil static filter/map operation")
	}
	type state struct{ iterator *rusthashmap.Iter[K, V] }
	ops := rustvec.IteratorOps[state, rusthashmap.Pair[KO, VO]]{
		Next: func(ref rustvec.Ref[state]) (rusthashmap.Pair[KO, VO], bool) {
			for {
				pair, ok := ref.Value.iterator.Next(ref.Context)
				if !ok {
					return rusthashmap.Pair[KO, VO]{}, false
				}
				if mapped, keep := combine(pair.Key, pair.Value, ref.Context); keep {
					return mapped, true
				}
			}
		},
		SizeHint: func(ref rustvec.Ref[state]) rustvec.Hint {
			return rustvec.Hint{Upper: ref.Value.iterator.Len(), HasUpper: true}
		},
		FoldUnit: func(arg *rustvec.Owned[state], consume func(rusthashmap.Pair[KO, VO])) {
			s := arg.Borrow()
			for {
				pair, ok := s.iterator.Next(arg.Context())
				if !ok {
					return
				}
				if mapped, keep := combine(pair.Key, pair.Value, arg.Context()); keep {
					consume(mapped)
				}
			}
		},
	}
	input := rustvec.FromIterator(rustvec.NewIterator(state{items.iterator}, ops))
	inner := rusthashmap.MapFromIterator(input, config.Layout, config.Profile, config.Keys, config.Values, func(rusthashmap.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}, c)
	return &UnordMap[KO, VO]{inner: inner, config: config, live: true}
}
