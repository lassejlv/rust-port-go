// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting std/map operations translated from indexmap2.14.0 src/map.rs.
package rustindexmap

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

type EquivalentOps[K, Q any] struct {
	Hash       func(*Q, Hasher, Context)
	HashOne    func(Builder, *Q, Context) uint64
	Equivalent func(query *Q, key *K, context Context) bool
}

func (m *Map[K, V]) GetIndex(index uint, contexts ...Context) (*K, *V, bool) {
	c := current(contexts)
	m.check(c)
	if index >= uint(len(m.core.entries)) {
		return nil, nil, false
	}
	v := &m.core.entries[index]
	return &v.key, &v.value, true
}
func (m *Map[K, V]) GetIndexMut(index uint, contexts ...Context) (*K, *V, bool) {
	return m.GetIndex(index, contexts...)
}
func (m *Map[K, V]) GetFull(key *K, contexts ...Context) (uint, *K, *V, bool) {
	c := current(contexts)
	m.check(c)
	m.requireSelfQuery(c)
	if len(m.core.entries) == 0 {
		return 0, nil, nil, false
	}
	if len(m.core.entries) == 1 {
		entry := &m.core.entries[0]
		if m.core.config.Keys.Equal(key, &entry.key, c) {
			return 0, &entry.key, &entry.value, true
		}
		return 0, nil, nil, false
	}
	hash := m.hash(key, c)
	index, _, found := m.core.find(hash, func(stored *K) bool { return m.core.config.Keys.Equal(key, stored, c) }, c)
	if !found {
		return 0, nil, nil, false
	}
	v := m.core.entry(index, c)
	return index, &v.key, &v.value, true
}
func (m *Map[K, V]) Get(key *K, contexts ...Context) (*V, bool) {
	_, _, v, ok := m.GetFull(key, contexts...)
	return v, ok
}
func (m *Map[K, V]) GetMut(key *K, contexts ...Context) (*V, bool) { return m.Get(key, contexts...) }
func (m *Map[K, V]) GetKeyValue(key *K, contexts ...Context) (*K, *V, bool) {
	_, k, v, ok := m.GetFull(key, contexts...)
	return k, v, ok
}
func (m *Map[K, V]) GetIndexOf(key *K, contexts ...Context) (uint, bool) {
	i, _, _, ok := m.GetFull(key, contexts...)
	return i, ok
}
func (m *Map[K, V]) ContainsKey(key *K, contexts ...Context) bool {
	_, ok := m.Get(key, contexts...)
	return ok
}
func (m *Map[K, V]) Index(key *K, contexts ...Context) *V {
	c := current(contexts)
	v, ok := m.Get(key, c)
	if !ok {
		c.Panic("no entry found for key")
	}
	return v
}
func (m *Map[K, V]) IndexPosition(index uint, contexts ...Context) *V {
	c := current(contexts)
	m.check(c)
	return &m.core.entry(index, c).value
}

func GetEquivalent[K, V, Q any](m *Map[K, V], query *Q, ops EquivalentOps[K, Q], contexts ...Context) (uint, *K, *V, bool) {
	c := current(contexts)
	m.check(c)
	requireSourceBuildHasher(m.builder, c)
	if query == nil || ops.Hash == nil || ops.Equivalent == nil {
		c.Panic("rustindexmap: incomplete Equivalent dictionary")
	}
	if len(m.core.entries) == 0 {
		return 0, nil, nil, false
	}
	if len(m.core.entries) == 1 {
		entry := &m.core.entries[0]
		if ops.Equivalent(query, &entry.key, c) {
			return 0, &entry.key, &entry.value, true
		}
		return 0, nil, nil, false
	}
	var hash uint64
	if ops.HashOne != nil {
		hash = ops.HashOne(m.builder, query, c)
	} else {
		hash = func() uint64 {
			h := m.builder.Build(c)
			if h == nil {
				c.Panic("rustindexmap: nil BuildHasher result")
			}
			defer cleanupCall(c, h.Close)
			actual := h.Borrow(c)
			ops.Hash(query, actual, c)
			return actual.Finish64()
		}()
	}
	index, _, ok := m.core.find(m.core.hashValue(hash), func(k *K) bool { return ops.Equivalent(query, k, c) }, c)
	if !ok {
		return 0, nil, nil, false
	}
	v := m.core.entry(index, c)
	return index, &v.key, &v.value, true
}
func (m *Map[K, V]) Reserve(additional uint, contexts ...Context) {
	c := current(contexts)
	m.check(c)
	m.core.reserve(additional, false, c)
}
func (m *Map[K, V]) ReserveExact(additional uint, contexts ...Context) {
	c := current(contexts)
	m.check(c)
	m.core.reserve(additional, true, c)
}

type InsertResult[V any] struct {
	Index uint
	Old   *rustvec.Owned[V]
}

func (m *Map[K, V]) InsertFullOwned(key *rustvec.Owned[K], value *rustvec.Owned[V], contexts ...Context) InsertResult[V] {
	c := current(contexts)
	m.check(c)
	m.requireSelfQuery(c)
	return rustvec.ConsumeValue(key, c, func(k *rustvec.Owned[K]) InsertResult[V] {
		return rustvec.ConsumeValue(value, c, func(v *rustvec.Owned[V]) InsertResult[V] {
			hash := m.hash(k.Borrow(), c)
			position, occupied := m.core.indices.FindOrInsert(hash, func(index *uint) bool { return m.core.config.Keys.Equal(k.Borrow(), &m.core.entry(*index, c).key, c) }, c)
			if occupied {
				index := *m.core.indices.At(position, c)
				slot := m.core.entry(index, c)
				old := slot.value
				slot.value = v.Take()
				return InsertResult[V]{Index: index, Old: rustvec.NewOwned(old, rustvec.ValueOps[V]{Drop: m.core.config.Values.Drop})}
			}
			index := uint(len(m.core.entries))
			m.core.indices.InsertAt(hash, position, index, c)
			if len(m.core.entries) == cap(m.core.entries) {
				m.core.reserveEntries(1, c)
			}
			m.core.push(hash, k.Take(), v.Take(), c)
			if m.core.config.DebugAssertions {
				m.core.len(c)
			}
			return InsertResult[V]{Index: index}
		})
	})
}
func (m *Map[K, V]) InsertOwned(key *rustvec.Owned[K], value *rustvec.Owned[V], contexts ...Context) *rustvec.Owned[V] {
	return m.InsertFullOwned(key, value, contexts...).Old
}
func (m *Map[K, V]) InsertFull(key K, value V, contexts ...Context) (uint, V, bool) {
	result := m.InsertFullOwned(rustvec.NewOwned(key, rustvec.ValueOps[K]{Drop: m.core.config.Keys.Drop}), rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: m.core.config.Values.Drop}), contexts...)
	if result.Old != nil {
		return result.Index, result.Old.Take(), true
	}
	var zero V
	return result.Index, zero, false
}
func (m *Map[K, V]) Insert(key K, value V, contexts ...Context) (V, bool) {
	_, old, ok := m.InsertFull(key, value, contexts...)
	return old, ok
}

func (m *Map[K, V]) ownPair(value Pair[K, V]) *rustvec.Owned[Pair[K, V]] {
	return rustvec.NewOwned(value, rustvec.ValueOps[Pair[K, V]]{Drop: func(v *Pair[K, V], c Context) { dropPair(m.core.config, &v.Key, &v.Value, c) }})
}
func (m *Map[K, V]) removeEntry(key *K, shift bool, c Context) *rustvec.Owned[Pair[K, V]] {
	m.check(c)
	m.requireSelfQuery(c)
	if len(m.core.entries) == 0 {
		return nil
	}
	if len(m.core.entries) == 1 {
		if !m.core.config.Keys.Equal(key, &m.core.entries[0].key, c) {
			return nil
		}
		v, ok := m.core.pop(c)
		if ok {
			return m.ownPair(v)
		}
		return nil
	}
	hash := m.hash(key, c)
	index, bucketIndex, found := m.core.find(hash, func(stored *K) bool { return m.core.config.Keys.Equal(key, stored, c) }, c)
	if !found {
		return nil
	}
	m.core.indices.RemoveAt(bucketIndex, c)
	if shift {
		return m.ownPair(m.core.shiftRemoveFinish(index, c))
	}
	return m.ownPair(m.core.swapRemoveFinish(index, c))
}
func (m *Map[K, V]) SwapRemoveEntryOwned(key *K, contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	return m.removeEntry(key, false, current(contexts))
}
func (m *Map[K, V]) ShiftRemoveEntryOwned(key *K, contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	return m.removeEntry(key, true, current(contexts))
}
func (m *Map[K, V]) valueFromPair(pair *rustvec.Owned[Pair[K, V]], c Context) *rustvec.Owned[V] {
	if pair == nil {
		return nil
	}
	v := pair.Take()
	result := rustvec.NewOwned(v.Value, rustvec.ValueOps[V]{Drop: m.core.config.Values.Drop})
	if m.core.config.Keys.Drop != nil {
		m.core.config.Keys.Drop(&v.Key, c)
	}
	return result
}
func (m *Map[K, V]) SwapRemoveOwned(key *K, contexts ...Context) *rustvec.Owned[V] {
	c := current(contexts)
	return m.valueFromPair(m.removeEntry(key, false, c), c)
}
func (m *Map[K, V]) ShiftRemoveOwned(key *K, contexts ...Context) *rustvec.Owned[V] {
	c := current(contexts)
	return m.valueFromPair(m.removeEntry(key, true, c), c)
}
func (m *Map[K, V]) SwapRemove(key *K, contexts ...Context) (V, bool) {
	v := m.SwapRemoveOwned(key, contexts...)
	if v != nil {
		return v.Take(), true
	}
	var zero V
	return zero, false
}
func (m *Map[K, V]) ShiftRemove(key *K, contexts ...Context) (V, bool) {
	v := m.ShiftRemoveOwned(key, contexts...)
	if v != nil {
		return v.Take(), true
	}
	var zero V
	return zero, false
}
func (m *Map[K, V]) SwapRemoveIndex(index uint, contexts ...Context) (Pair[K, V], bool) {
	c := current(contexts)
	m.check(c)
	return m.core.removeIndex(index, false, c)
}
func (m *Map[K, V]) ShiftRemoveIndex(index uint, contexts ...Context) (Pair[K, V], bool) {
	c := current(contexts)
	m.check(c)
	return m.core.removeIndex(index, true, c)
}
func (m *Map[K, V]) Pop(contexts ...Context) (Pair[K, V], bool) {
	c := current(contexts)
	m.check(c)
	if len(m.core.entries) == 0 {
		return Pair[K, V]{}, false
	}
	return m.core.pop(c)
}
func (m *Map[K, V]) Clear(contexts ...Context) { c := current(contexts); m.check(c); m.core.clear(c) }

func (m *Map[K, V]) Reverse(contexts ...Context) {
	c := current(contexts)
	m.check(c)
	n := len(m.core.entries)
	for a, b := 0, n-1; a < b; a, b = a+1, b-1 {
		m.core.entries[a], m.core.entries[b] = m.core.entries[b], m.core.entries[a]
	}
	it := m.core.indices.Iter(c)
	for {
		index, ok := it.Next(c)
		if !ok {
			break
		}
		*index = uint(n) - *index - 1
	}
}
