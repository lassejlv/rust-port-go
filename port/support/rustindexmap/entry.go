// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting Entry/OccupiedEntry/VacantEntry paths from indexmap2.14.0.
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type EntryKind uint8

const (
	Occupied EntryKind = iota
	Vacant
)

type Entry[K, V any] struct {
	owner         *Map[K, V]
	kind          EntryKind
	index, bucket uint
	hash          uint64
	key           *rustvec.Owned[K]
	live          bool
}
type OccupiedEntry[K, V any] struct{ entry *Entry[K, V] }
type VacantEntry[K, V any] struct{ entry *Entry[K, V] }

func (e *Entry[K, V]) check(c Context) {
	if e == nil || !e.live {
		c.Panic("rustindexmap: use of consumed Entry")
	}
	e.owner.check(c)
}
func (e *Entry[K, V]) move(c Context) *Entry[K, V] {
	e.check(c)
	next := *e
	e.live = false
	if e.key != nil {
		next.key = e.key.MoveWithContext(c)
	}
	return &next
}
func (e *Entry[K, V]) Close(c Context) {
	if e == nil || !e.live {
		return
	}
	e.live = false
	if e.key != nil {
		e.key.DropWithContext(c)
	}
}
func consumeEntry[K, V, R any](e *Entry[K, V], c Context, call func(*Entry[K, V]) R) R {
	local := e.move(c)
	defer cleanupCall(c, local.Close)
	return call(local)
}

func (m *Map[K, V]) EntryOwned(key *rustvec.Owned[K], contexts ...Context) *Entry[K, V] {
	c := current(contexts)
	m.check(c)
	m.requireSelfQuery(c)
	return rustvec.ConsumeValue(key, c, func(key *rustvec.Owned[K]) *Entry[K, V] {
		hash := m.hash(key.Borrow(), c)
		index, position, found := m.core.find(hash, func(stored *K) bool { return m.core.config.Keys.Equal(key.Borrow(), stored, c) }, c)
		if found {
			return &Entry[K, V]{owner: m, kind: Occupied, index: index, bucket: position, live: true}
		}
		return &Entry[K, V]{owner: m, kind: Vacant, hash: hash, key: key.MoveWithContext(c), live: true}
	})
}
func (m *Map[K, V]) Entry(key K, contexts ...Context) *Entry[K, V] {
	return m.EntryOwned(rustvec.NewOwned(key, rustvec.ValueOps[K]{Drop: m.core.config.Keys.Drop}), contexts...)
}
func (e *Entry[K, V]) Kind() EntryKind { e.check(Context{}); return e.kind }
func (e *Entry[K, V]) Index(c Context) uint {
	e.check(c)
	if e.kind == Occupied {
		return e.index
	}
	return e.owner.core.indices.Len()
}
func (e *Entry[K, V]) Key(c Context) *K {
	e.check(c)
	if e.kind == Occupied {
		return &e.owner.core.entry(e.index, c).key
	}
	return e.key.Borrow()
}
func (e *Entry[K, V]) TakeOccupied(c Context) (*OccupiedEntry[K, V], bool) {
	e.check(c)
	if e.kind != Occupied {
		return nil, false
	}
	return &OccupiedEntry[K, V]{e.move(c)}, true
}
func (e *Entry[K, V]) TakeVacant(c Context) (*VacantEntry[K, V], bool) {
	e.check(c)
	if e.kind != Vacant {
		return nil, false
	}
	return &VacantEntry[K, V]{e.move(c)}, true
}

func (e *Entry[K, V]) insertVacant(value *rustvec.Owned[V], c Context) *OccupiedEntry[K, V] {
	index := uint(len(e.owner.core.entries))
	position, _ := e.owner.core.indices.InsertUnique(e.hash, index, c)
	if len(e.owner.core.entries) == cap(e.owner.core.entries) {
		e.owner.core.reserveEntries(1, c)
	}
	e.owner.core.push(e.hash, e.key.Take(), value.Take(), c)
	return &OccupiedEntry[K, V]{&Entry[K, V]{owner: e.owner, kind: Occupied, index: index, bucket: position, live: true}}
}
func (e *Entry[K, V]) OrInsertOwned(value *rustvec.Owned[V], c Context) *V {
	return consumeEntry(e, c, func(e *Entry[K, V]) *V {
		return rustvec.ConsumeValue(value, c, func(v *rustvec.Owned[V]) *V {
			if e.kind == Occupied {
				return &e.owner.core.entry(e.index, c).value
			}
			occupied := e.insertVacant(v, c)
			return occupied.IntoMut(c)
		})
	})
}
func (e *Entry[K, V]) OrInsert(value V, c Context) *V {
	return e.OrInsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: e.owner.core.config.Values.Drop}), c)
}
func (e *Entry[K, V]) OrDefault(c Context) *V {
	return consumeEntry(e, c, func(e *Entry[K, V]) *V {
		if e.kind == Occupied {
			return &e.owner.core.entry(e.index, c).value
		}
		if e.owner.core.config.Values.Default == nil {
			c.Panic("rustindexmap: missing value Default witness")
		}
		return e.OrInsertOwned(rustvec.NewOwned(e.owner.core.config.Values.Default(c), rustvec.ValueOps[V]{Drop: e.owner.core.config.Values.Drop}), c)
	})
}
func (e *Entry[K, V]) OrInsertWithOwned(call rusthashmap.Once[struct{}, *rustvec.Owned[V]], c Context) *V {
	return consumeEntry(e, c, func(e *Entry[K, V]) *V {
		defer cleanupCall(c, call.Close)
		if e.kind == Occupied {
			return &e.owner.core.entry(e.index, c).value
		}
		value := call.Call(struct{}{}, c)
		return e.OrInsertOwned(value, c)
	})
}
func (e *Entry[K, V]) OrInsertWithKeyOwned(call rusthashmap.Once[*K, *rustvec.Owned[V]], c Context) *V {
	return consumeEntry(e, c, func(e *Entry[K, V]) *V {
		defer cleanupCall(c, call.Close)
		if e.kind == Occupied {
			return &e.owner.core.entry(e.index, c).value
		}
		value := call.Call(e.Key(c), c)
		return e.OrInsertOwned(value, c)
	})
}
func (e *Entry[K, V]) AndModifyOwned(call rusthashmap.Once[*V, struct{}], c Context) *Entry[K, V] {
	return consumeEntry(e, c, func(e *Entry[K, V]) *Entry[K, V] {
		defer cleanupCall(c, call.Close)
		if e.kind == Occupied {
			call.Call(&e.owner.core.entry(e.index, c).value, c)
		}
		return e.move(c)
	})
}
func (e *Entry[K, V]) AndModify(call func(*V), c Context) *Entry[K, V] {
	return e.AndModifyOwned(rusthashmap.PlainOnce(func(v *V, _ Context) struct{} { call(v); return struct{}{} }), c)
}
func (e *Entry[K, V]) InsertEntryOwned(value *rustvec.Owned[V], c Context) *OccupiedEntry[K, V] {
	return consumeEntry(e, c, func(e *Entry[K, V]) *OccupiedEntry[K, V] {
		return rustvec.ConsumeValue(value, c, func(v *rustvec.Owned[V]) *OccupiedEntry[K, V] {
			if e.kind == Vacant {
				return e.insertVacant(v, c)
			}
			slot := e.owner.core.entry(e.index, c)
			old := slot.value
			slot.value = v.Take()
			if e.owner.core.config.Values.Drop != nil {
				e.owner.core.config.Values.Drop(&old, c)
			}
			return &OccupiedEntry[K, V]{e.move(c)}
		})
	})
}
func (e *Entry[K, V]) InsertEntry(value V, c Context) *OccupiedEntry[K, V] {
	return e.InsertEntryOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: e.owner.core.config.Values.Drop}), c)
}

func (o *OccupiedEntry[K, V]) Index(c Context) uint { o.entry.check(c); return o.entry.index }
func (o *OccupiedEntry[K, V]) Key(c Context) *K     { return o.entry.Key(c) }
func (o *OccupiedEntry[K, V]) Get(c Context) *V {
	o.entry.check(c)
	return &o.entry.owner.core.entry(o.entry.index, c).value
}
func (o *OccupiedEntry[K, V]) GetMut(c Context) *V { return o.Get(c) }
func (o *OccupiedEntry[K, V]) IntoMut(c Context) *V {
	value := o.Get(c)
	o.entry.live = false
	return value
}
func (o *OccupiedEntry[K, V]) Close(c Context) { o.entry.Close(c) }
func (o *OccupiedEntry[K, V]) InsertOwned(value *rustvec.Owned[V], c Context) *rustvec.Owned[V] {
	o.entry.check(c)
	return rustvec.ConsumeValue(value, c, func(v *rustvec.Owned[V]) *rustvec.Owned[V] {
		slot := o.entry.owner.core.entry(o.entry.index, c)
		old := slot.value
		slot.value = v.Take()
		return rustvec.NewOwned(old, rustvec.ValueOps[V]{Drop: o.entry.owner.core.config.Values.Drop})
	})
}
func (o *OccupiedEntry[K, V]) Insert(value V, c Context) V {
	return o.InsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: o.entry.owner.core.config.Values.Drop}), c).Take()
}
func (o *OccupiedEntry[K, V]) removeEntry(shift bool, c Context) *rustvec.Owned[Pair[K, V]] {
	return consumeEntry(o.entry, c, func(e *Entry[K, V]) *rustvec.Owned[Pair[K, V]] {
		e.owner.core.indices.RemoveAt(e.bucket, c)
		var pair Pair[K, V]
		if shift {
			pair = e.owner.core.shiftRemoveFinish(e.index, c)
		} else {
			pair = e.owner.core.swapRemoveFinish(e.index, c)
		}
		return e.owner.ownPair(pair)
	})
}
func (o *OccupiedEntry[K, V]) SwapRemoveEntryOwned(c Context) *rustvec.Owned[Pair[K, V]] {
	return o.removeEntry(false, c)
}
func (o *OccupiedEntry[K, V]) ShiftRemoveEntryOwned(c Context) *rustvec.Owned[Pair[K, V]] {
	return o.removeEntry(true, c)
}
func (o *OccupiedEntry[K, V]) SwapRemoveOwned(c Context) *rustvec.Owned[V] {
	return o.entry.owner.valueFromPair(o.removeEntry(false, c), c)
}
func (o *OccupiedEntry[K, V]) ShiftRemoveOwned(c Context) *rustvec.Owned[V] {
	return o.entry.owner.valueFromPair(o.removeEntry(true, c), c)
}

func (v *VacantEntry[K, V]) Index(c Context) uint { return v.entry.Index(c) }
func (v *VacantEntry[K, V]) Key(c Context) *K     { return v.entry.Key(c) }
func (v *VacantEntry[K, V]) Close(c Context)      { v.entry.Close(c) }
func (v *VacantEntry[K, V]) IntoKey(c Context) K {
	return consumeEntry(v.entry, c, func(e *Entry[K, V]) K { return e.key.Take() })
}
func (v *VacantEntry[K, V]) InsertOwned(value *rustvec.Owned[V], c Context) *V {
	return v.entry.OrInsertOwned(value, c)
}
func (v *VacantEntry[K, V]) Insert(value V, c Context) *V { return v.entry.OrInsert(value, c) }
func (v *VacantEntry[K, V]) InsertEntryOwned(value *rustvec.Owned[V], c Context) *OccupiedEntry[K, V] {
	return v.entry.InsertEntryOwned(value, c)
}
func (v *VacantEntry[K, V]) InsertEntry(value V, c Context) *OccupiedEntry[K, V] {
	return v.entry.InsertEntry(value, c)
}
