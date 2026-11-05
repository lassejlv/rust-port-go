// SPDX-License-Identifier: MIT OR Apache-2.0
// std's Entry path uses hashbrown::rustc_entry, whose lookup/reserve order and
// equality receiver differ from hashbrown's ordinary Entry implementation.
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

type EntryKind uint8

const (
	Vacant EntryKind = iota
	Occupied
)

// Entry holds an exclusive borrow of its map and, when vacant, owns the input
// key. Close must be called when an entry is discarded. No map mutation or
// competing borrow is permitted until the entry has been consumed/closed.
type Entry[K, V any] struct {
	owner *Map[K, V]
	index uint
	hash  uint64
	key   *rustvec.Owned[K]
	kind  EntryKind
	live  bool
}
type OccupiedEntry[K, V any] struct{ entry *Entry[K, V] }
type VacantEntry[K, V any] struct{ entry *Entry[K, V] }

func (m *Map[K, V]) Entry(key K, c Context) *Entry[K, V] {
	return m.EntryOwned(rustvec.NewOwned(key, rustvec.ValueOps[K]{Drop: m.keys.Drop}), c)
}
func (m *Map[K, V]) EntryOwned(key *rustvec.Owned[K], c Context) *Entry[K, V] {
	m.check(c)
	k := key.MoveWithContext(c)
	defer cleanupState(k, c)
	hash := m.hash(k.Borrow(), c)
	index, ok := m.table.Find(hash, func(pair *Pair[K, V]) bool { return m.keys.Equal(&pair.Key, k.Borrow(), c) }, c)
	if ok {
		k.DropWithContext(c)
		return &Entry[K, V]{owner: m, index: index, hash: hash, kind: Occupied, live: true}
	}
	m.table.Reserve(1, c)
	return &Entry[K, V]{owner: m, hash: hash, key: k.MoveWithContext(c), kind: Vacant, live: true}
}
func (e *Entry[K, V]) check(c Context) {
	if e == nil || !e.live {
		c.Panic("rusthashmap: use of consumed Entry")
	}
	e.owner.check(c)
}
func (e *Entry[K, V]) Kind() EntryKind { e.check(Context{}); return e.kind }
func (e *Entry[K, V]) Key(c Context) *K {
	e.check(c)
	if e.kind == Vacant {
		return e.key.Borrow()
	}
	return &e.owner.table.At(e.index, c).Key
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
func (e *Entry[K, V]) TakeOccupied(c Context) (*OccupiedEntry[K, V], bool) {
	e.check(c)
	if e.kind != Occupied {
		return nil, false
	}
	e.live = false
	copy := *e
	copy.live = true
	return &OccupiedEntry[K, V]{&copy}, true
}
func (e *Entry[K, V]) TakeVacant(c Context) (*VacantEntry[K, V], bool) {
	e.check(c)
	if e.kind != Vacant {
		return nil, false
	}
	e.live = false
	copy := *e
	copy.live = true
	return &VacantEntry[K, V]{&copy}, true
}
func (e *Entry[K, V]) OrInsert(value V, c Context) *V {
	return e.OrInsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: e.owner.values.Drop}), c)
}
func (e *Entry[K, V]) OrInsertOwned(value *rustvec.Owned[V], c Context) *V {
	e.check(c)
	v := value.MoveWithContext(c)
	defer cleanupEntryValue(e, v, c)
	if e.kind == Occupied {
		result := &e.owner.table.At(e.index, c).Value
		e.live = false
		return result
	}
	return e.insertVacant(v, c)
}
func (e *Entry[K, V]) insertVacant(value *rustvec.Owned[V], c Context) *V {
	e.check(c)
	if e.kind != Vacant {
		c.Panic("rusthashmap: expected vacant Entry")
	}
	index := e.owner.table.findInsert(e.hash)
	pair := Pair[K, V]{Key: e.key.Take(), Value: value.Take()}
	e.key = nil
	e.live = false
	return &e.owner.table.InsertAt(e.hash, index, pair, c).Value
}
func cleanupEntryValue[K, V any](e *Entry[K, V], value *rustvec.Owned[V], c Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { value.DropWithContext(unwind) })
		cleanupOnPanic(unwind, func() { e.Close(unwind) })
		c.ResumeUnwind(p)
	}
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			cleanupOnPanic(unwind, func() { e.Close(unwind) })
			c.ResumeUnwind(p)
		}
	}()
	value.DropWithContext(c)
	e.Close(c)
}
func (e *Entry[K, V]) OrDefault(c Context) *V {
	if e.owner.values.Default == nil {
		c.Panic("rusthashmap: missing value Default implementation")
	}
	return e.OrInsertWithOwned(PlainOnce(func(_ struct{}, ctx Context) *rustvec.Owned[V] {
		return rustvec.NewOwned(e.owner.values.Default(ctx), rustvec.ValueOps[V]{Drop: e.owner.values.Drop})
	}), c)
}
func (e *Entry[K, V]) OrInsertWithOwned(makeValue Once[struct{}, *rustvec.Owned[V]], c Context) *V {
	return e.OrInsertWithKeyOwned(NewOnce(rustvec.NewOwned(makeValue, rustvec.ValueOps[Once[struct{}, *rustvec.Owned[V]]]{Drop: func(f *Once[struct{}, *rustvec.Owned[V]], ctx Context) { (*f).Close(ctx) }}), func(f *Once[struct{}, *rustvec.Owned[V]], _ *K, ctx Context) *rustvec.Owned[V] {
		return (*f).Call(struct{}{}, ctx)
	}), c)
}
func (e *Entry[K, V]) OrInsertWithKeyOwned(makeValue Once[*K, *rustvec.Owned[V]], c Context) *V {
	e.check(c)
	defer cleanupEntryOnce(e, makeValue, c)
	if e.kind == Occupied {
		result := &e.owner.table.At(e.index, c).Value
		e.live = false
		return result
	}
	value := makeValue.Call(e.key.Borrow(), c)
	if value == nil || !value.IsLive() {
		c.Panic("rusthashmap: FnOnce returned consumed value")
	}
	return e.insertVacant(value, c)
}
func cleanupEntryOnce[K, V, A, R any](e *Entry[K, V], call Once[A, R], c Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { call.Close(unwind) })
		cleanupOnPanic(unwind, func() { e.Close(unwind) })
		c.ResumeUnwind(p)
	}
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			cleanupOnPanic(unwind, func() { e.Close(unwind) })
			c.ResumeUnwind(p)
		}
	}()
	call.Close(c)
	e.Close(c)
}
func (e *Entry[K, V]) AndModifyOwned(modify Once[*V, struct{}], c Context) *Entry[K, V] {
	e.check(c)
	returned := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			cleanupOnPanic(unwind, func() { modify.Close(unwind) })
			cleanupOnPanic(unwind, func() { e.Close(unwind) })
			c.ResumeUnwind(p)
		}
		defer func() {
			if p := recover(); p != nil {
				if c.ProcessingPanic {
					c.ResumeUnwind(p)
				}
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { e.Close(unwind) })
				c.ResumeUnwind(p)
			}
		}()
		modify.Close(c)
		if !returned {
			e.Close(c)
		}
	}()
	if e.kind == Occupied {
		modify.Call(&e.owner.table.At(e.index, c).Value, c)
	}
	// Returning self moves the entire entry to the prepared result place.
	e.live = false
	result := *e
	result.live = true
	returned = true
	return &result
}
func (e *Entry[K, V]) AndModify(modify func(*V), c Context) *Entry[K, V] {
	return e.AndModifyOwned(PlainOnce(func(v *V, _ Context) struct{} { modify(v); return struct{}{} }), c)
}

func (o *OccupiedEntry[K, V]) check(c Context) {
	if o == nil || o.entry == nil {
		c.Panic("rusthashmap: nil occupied entry")
	}
	o.entry.check(c)
	if o.entry.kind != Occupied {
		c.Panic("rusthashmap: expected occupied entry")
	}
}
func (o *OccupiedEntry[K, V]) Key(c Context) *K { o.check(c); return o.entry.Key(c) }
func (o *OccupiedEntry[K, V]) Get(c Context) *V {
	o.check(c)
	return &o.entry.owner.table.At(o.entry.index, c).Value
}
func (o *OccupiedEntry[K, V]) GetMut(c Context) *V { return o.Get(c) }
func (o *OccupiedEntry[K, V]) IntoMut(c Context) *V {
	value := o.Get(c)
	o.entry.live = false
	return value
}
func (o *OccupiedEntry[K, V]) Close(c Context) {
	if o != nil {
		o.entry.Close(c)
	}
}
func (o *OccupiedEntry[K, V]) InsertOwned(value *rustvec.Owned[V], c Context) *rustvec.Owned[V] {
	o.check(c)
	pair := o.entry.owner.table.At(o.entry.index, c)
	old := pair.Value
	pair.Value = value.Take()
	return rustvec.NewOwned(old, rustvec.ValueOps[V]{Drop: o.entry.owner.values.Drop})
}
func (o *OccupiedEntry[K, V]) Insert(value V, c Context) V {
	return o.InsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: o.entry.owner.values.Drop}), c).Take()
}
func (o *OccupiedEntry[K, V]) RemoveEntryOwned(c Context) *rustvec.Owned[Pair[K, V]] {
	o.check(c)
	e := o.entry
	e.live = false
	return rustvec.NewOwned(e.owner.table.RemoveAt(e.index, c), rustvec.ValueOps[Pair[K, V]]{Drop: e.owner.dropPair})
}
func (o *OccupiedEntry[K, V]) RemoveEntry(c Context) Pair[K, V] { return o.RemoveEntryOwned(c).Take() }
func (o *OccupiedEntry[K, V]) RemoveOwned(c Context) *rustvec.Owned[V] {
	pair := o.RemoveEntryOwned(c).Take()
	if o.entry.owner.keys.Drop != nil {
		o.entry.owner.keys.Drop(&pair.Key, c)
	}
	return rustvec.NewOwned(pair.Value, rustvec.ValueOps[V]{Drop: o.entry.owner.values.Drop})
}
func (o *OccupiedEntry[K, V]) Remove(c Context) V { return o.RemoveOwned(c).Take() }

func (v *VacantEntry[K, V]) check(c Context) {
	if v == nil || v.entry == nil {
		c.Panic("rusthashmap: nil vacant entry")
	}
	v.entry.check(c)
	if v.entry.kind != Vacant {
		c.Panic("rusthashmap: expected vacant entry")
	}
}
func (v *VacantEntry[K, V]) Key(c Context) *K { v.check(c); return v.entry.key.Borrow() }
func (v *VacantEntry[K, V]) IntoKeyOwned(c Context) *rustvec.Owned[K] {
	v.check(c)
	v.entry.live = false
	return v.entry.key.MoveWithContext(c)
}
func (v *VacantEntry[K, V]) IntoKey(c Context) K { return v.IntoKeyOwned(c).Take() }
func (v *VacantEntry[K, V]) InsertOwned(value *rustvec.Owned[V], c Context) *V {
	v.check(c)
	return v.entry.OrInsertOwned(value, c)
}
func (v *VacantEntry[K, V]) Insert(value V, c Context) *V {
	return v.InsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: v.entry.owner.values.Drop}), c)
}
func (v *VacantEntry[K, V]) Close(c Context) {
	if v != nil {
		v.entry.Close(c)
	}
}

// OccupiedError owns the uninserted key/value as separate source fields, along
// with the exclusive occupied entry borrow. This is std's error, which differs
// from hashbrown's ordinary OccupiedError by retaining the uninserted key.
type OccupiedError[K, V any] struct {
	Entry *OccupiedEntry[K, V]
	Key   *rustvec.Owned[K]
	Value *rustvec.Owned[V]
	live  bool
}

func (e *OccupiedError[K, V]) Close(c Context) {
	if e == nil || !e.live {
		return
	}
	e.live = false
	e.Entry.Close(c)
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			cleanupOnPanic(unwind, func() { e.Value.DropWithContext(unwind) })
			c.ResumeUnwind(p)
		}
	}()
	e.Key.DropWithContext(c)
	e.Value.DropWithContext(c)
}
func (m *Map[K, V]) TryInsert(key K, value V, c Context) (*V, *OccupiedError[K, V]) {
	return m.TryInsertOwned(rustvec.NewOwned(key, rustvec.ValueOps[K]{Drop: m.keys.Drop}), rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: m.values.Drop}), c)
}
func (m *Map[K, V]) TryInsertOwned(key *rustvec.Owned[K], value *rustvec.Owned[V], c Context) (*V, *OccupiedError[K, V]) {
	m.check(c)
	k, v := key.MoveWithContext(c), value.MoveWithContext(c)
	defer cleanupTwo(k, v, c)
	hash := m.hash(k.Borrow(), c)
	index, found := m.table.Find(hash, func(pair *Pair[K, V]) bool { return m.keys.Equal(&pair.Key, k.Borrow(), c) }, c)
	if found {
		entry := &OccupiedEntry[K, V]{&Entry[K, V]{owner: m, index: index, hash: hash, kind: Occupied, live: true}}
		return nil, &OccupiedError[K, V]{Entry: entry, Key: k.MoveWithContext(c), Value: v.MoveWithContext(c), live: true}
	}
	_, pair := m.table.insertUnique(hash, Pair[K, V]{Key: k.Take(), Value: v.Take()}, c)
	return &pair.Value, nil
}

func (v *VacantEntry[K, V]) InsertEntryOwned(value *rustvec.Owned[V], c Context) *OccupiedEntry[K, V] {
	v.check(c)
	e := v.entry
	val := value.MoveWithContext(c)
	defer cleanupEntryValue(e, val, c)
	index := e.owner.table.findInsert(e.hash)
	e.owner.table.InsertAt(e.hash, index, Pair[K, V]{Key: e.key.Take(), Value: val.Take()}, c)
	e.key = nil
	e.live = false
	return &OccupiedEntry[K, V]{&Entry[K, V]{owner: e.owner, index: index, hash: e.hash, kind: Occupied, live: true}}
}
func (v *VacantEntry[K, V]) InsertEntry(value V, c Context) *OccupiedEntry[K, V] {
	return v.InsertEntryOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: v.entry.owner.values.Drop}), c)
}
func (e *Entry[K, V]) InsertEntryOwned(value *rustvec.Owned[V], c Context) *OccupiedEntry[K, V] {
	e.check(c)
	if e.kind == Vacant {
		v, _ := e.TakeVacant(c)
		return v.InsertEntryOwned(value, c)
	}
	o, _ := e.TakeOccupied(c)
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			o.Close(c)
			c.ResumeUnwind(p)
		}
	}()
	old := o.InsertOwned(value, c)
	old.DropWithContext(c)
	return o
}
func (e *Entry[K, V]) InsertEntry(value V, c Context) *OccupiedEntry[K, V] {
	return e.InsertEntryOwned(rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: e.owner.values.Drop}), c)
}

type TryValue[V, E any] struct {
	Value *rustvec.Owned[V]
	Error *rustvec.Owned[E]
	OK    bool
}

func OrTryInsertWithKey[K, V, E any](entry *Entry[K, V], makeValue Once[*K, TryValue[V, E]], c Context) (*V, *rustvec.Owned[E]) {
	entry.check(c)
	defer cleanupEntryOnce(entry, makeValue, c)
	if entry.kind == Occupied {
		value := &entry.owner.table.At(entry.index, c).Value
		entry.live = false
		return value, nil
	}
	result := makeValue.Call(entry.key.Borrow(), c)
	if !result.OK {
		if result.Error == nil || !result.Error.IsLive() || result.Value != nil {
			c.Panic("rusthashmap: invalid fallible FnOnce error result")
		}
		return nil, result.Error
	}
	if result.Value == nil || !result.Value.IsLive() || result.Error != nil {
		c.Panic("rusthashmap: invalid fallible FnOnce success result")
	}
	return entry.insertVacant(result.Value, c), nil
}
func OrTryInsertWith[K, V, E any](entry *Entry[K, V], makeValue Once[struct{}, TryValue[V, E]], c Context) (*V, *rustvec.Owned[E]) {
	adapter := NewOnce(rustvec.NewOwned(makeValue, rustvec.ValueOps[Once[struct{}, TryValue[V, E]]]{Drop: func(f *Once[struct{}, TryValue[V, E]], ctx Context) { (*f).Close(ctx) }}), func(f *Once[struct{}, TryValue[V, E]], _ *K, ctx Context) TryValue[V, E] {
		return (*f).Call(struct{}{}, ctx)
	})
	return OrTryInsertWithKey(entry, adapter, c)
}
