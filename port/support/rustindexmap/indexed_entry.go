// SPDX-License-Identifier: MIT OR Apache-2.0
// IndexedEntry from indexmap2.14.0 src/map/entry.rs. Existing core methods stay unchanged.
package rustindexmap

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

// IndexedEntry holds the actual exclusive map borrow. Returned Key/Get borrows
// are shared; GetMut is exclusive. IntoMut transfers the value borrow to the
// map lifetime. No handle/view may overlap an incompatible map operation.
// Descriptor copies are not source Clone and cannot duplicate its live borrow.
type IndexedEntry[K, V any] struct {
	owner *Map[K, V]
	index uint
	live  bool
}

func (m *Map[K, V]) GetIndexEntry(index uint, contexts ...Context) (*IndexedEntry[K, V], bool) {
	c := current(contexts)
	m.check(c)
	if index >= m.core.len(c) {
		return nil, false
	}
	return &IndexedEntry[K, V]{m, index, true}, true
}
func (m *Map[K, V]) FirstEntry(contexts ...Context) (*IndexedEntry[K, V], bool) {
	return m.GetIndexEntry(0, contexts...)
}
func (m *Map[K, V]) LastEntry(contexts ...Context) (*IndexedEntry[K, V], bool) {
	c := current(contexts)
	length := m.Len(c)
	if length == 0 {
		return nil, false
	}
	return m.GetIndexEntry(length-1, c)
}
func (e *IndexedEntry[K, V]) check(c Context) {
	if e == nil || !e.live {
		c.Panic("rustindexmap: use of consumed IndexedEntry")
	}
	e.owner.check(c)
}
func (e *IndexedEntry[K, V]) Close(contexts ...Context) {
	if e == nil || !e.live {
		return
	}
	e.check(current(contexts))
	e.live = false
}
func (e *IndexedEntry[K, V]) Index(contexts ...Context) uint {
	e.check(current(contexts))
	return e.index
}
func (e *IndexedEntry[K, V]) Key(contexts ...Context) *K {
	c := current(contexts)
	e.check(c)
	return &e.owner.core.entry(e.index, c).key
}
func (e *IndexedEntry[K, V]) Get(contexts ...Context) *V {
	c := current(contexts)
	e.check(c)
	return &e.owner.core.entry(e.index, c).value
}
func (e *IndexedEntry[K, V]) GetMut(contexts ...Context) *V { return e.Get(contexts...) }
func (e *IndexedEntry[K, V]) keyMut(c Context) *K           { return e.Key(c) }
func (e *IndexedEntry[K, V]) IntoMut(contexts ...Context) *V {
	value := e.GetMut(contexts...)
	e.live = false
	return value
}
func (e *IndexedEntry[K, V]) InsertOwned(value *rustvec.Owned[V], contexts ...Context) *rustvec.Owned[V] {
	c := current(contexts)
	e.check(c)
	return rustvec.ConsumeValue(value, c, func(value *rustvec.Owned[V]) *rustvec.Owned[V] {
		slot := e.GetMut(c)
		old := *slot
		*slot = value.Take()
		return rustvec.NewOwned(old, rustvec.ValueOps[V]{Drop: e.owner.core.config.Values.Drop}).MoveWithContext(c)
	})
}
func (e *IndexedEntry[K, V]) Insert(value V, contexts ...Context) V {
	c := current(contexts)
	return e.InsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: e.owner.core.config.Values.Drop}), c).Take()
}
func (e *IndexedEntry[K, V]) removeEntry(shift bool, c Context) *rustvec.Owned[Pair[K, V]] {
	e.check(c)
	e.live = false
	pair, ok := e.owner.core.removeIndex(e.index, shift, c)
	if !ok {
		c.Panic("rustindexmap: indexed entry invariant violated")
	}
	return e.owner.ownPair(pair).MoveWithContext(c)
}
func (e *IndexedEntry[K, V]) SwapRemoveEntryOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	return e.removeEntry(false, current(contexts))
}
func (e *IndexedEntry[K, V]) ShiftRemoveEntryOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	return e.removeEntry(true, current(contexts))
}
func (e *IndexedEntry[K, V]) SwapRemoveOwned(contexts ...Context) *rustvec.Owned[V] {
	c := current(contexts)
	return builderInvoke(c, func() *rustvec.Owned[V] { return e.owner.valueFromPair(e.removeEntry(false, c), c) }).MoveWithContext(c)
}
func (e *IndexedEntry[K, V]) ShiftRemoveOwned(contexts ...Context) *rustvec.Owned[V] {
	c := current(contexts)
	return builderInvoke(c, func() *rustvec.Owned[V] { return e.owner.valueFromPair(e.removeEntry(true, c), c) }).MoveWithContext(c)
}
func (e *IndexedEntry[K, V]) MoveIndex(to uint, contexts ...Context) {
	c := current(contexts)
	e.check(c)
	e.live = false
	e.owner.MoveIndex(e.index, to, c)
}
func (e *IndexedEntry[K, V]) SwapIndices(other uint, contexts ...Context) {
	c := current(contexts)
	e.check(c)
	e.live = false
	e.owner.SwapIndices(e.index, other, c)
}
func (o *OccupiedEntry[K, V]) IntoIndexed(contexts ...Context) *IndexedEntry[K, V] {
	c := current(contexts)
	o.entry.check(c)
	result := &IndexedEntry[K, V]{o.entry.owner, o.entry.index, true}
	o.entry.live = false
	return result
}
