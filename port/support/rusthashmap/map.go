// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped std HashMap operations delegated to hashbrown 0.17.1. No completion
// credit is claimed for std HashMap or hashbrown by this backing adapter.
package rusthashmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// KeyOps and ValueOps are source trait implementations. Borrowed pointers must
// not outlive the source operation. Hash/Eq implementations must satisfy Rust's
// HashMap laws; changing a key's hash/equality while stored is a logic error.
// Hash/Equal/Clone/Debug borrow shared source values: callbacks may mutate only
// fields represented by source interior mutability, not ordinary stored fields.
type KeyOps[K any] struct {
	Hash func(*K, Hasher, Context)
	// HashOne is the monomorphized S::hash_one::<&K> override. Nil is lawful
	// only when that selected BuildHasher implementation uses std's default.
	// An override may skip Build, Hash, Finish and hasher Drop entirely.
	HashOne      func(Builder, *K, Context) uint64
	Equal        func(*K, *K, Context) bool
	Clone        func(*K, Context) K
	Drop         func(*K, Context)
	Debug        func(*K, *rustfmt.Formatter) error
	TrivialClone bool
}
type ValueOps[V any] struct {
	Clone        func(*V, Context) V
	Drop         func(*V, Context)
	Equal        func(*V, *V, Context) bool
	Debug        func(*V, *rustfmt.Formatter) error
	Default      func(Context) V
	TrivialClone bool
}

type Pair[K, V any] struct {
	Key   K
	Value V
}

// OwnedHasher is one live BuildHasher result. Its optional destructor runs
// after Finish64 and also during unwinding from the key's Hash implementation.
type OwnedHasher struct {
	value Hasher
	drop  func(Hasher, Context)
	live  bool
}

func NewOwnedHasher(value Hasher, drop func(Hasher, Context)) *OwnedHasher {
	if value == nil {
		panic("rusthashmap: nil Hasher result")
	}
	return &OwnedHasher{value: value, drop: drop, live: true}
}
func (h *OwnedHasher) Close(c Context) {
	if h == nil || !h.live {
		return
	}
	h.live = false
	if h.drop != nil {
		h.drop(h.value, c)
	}
}

// Builder owns the source BuildHasher state. A custom implementation must
// contain the complete owning state, and implement Clone/Close faithfully; Go
// closure captures with unrepresented Rust destructors are not permitted.
type Builder interface {
	Build(Context) *OwnedHasher
	Clone(Context) Builder
	CloneFrom(Builder, Context)
	Close(Context)
}
type FxBuilder struct{ PointerBits uint }

func (b FxBuilder) Build(c Context) *OwnedHasher {
	width := b.PointerBits
	if width == 0 {
		width = DefaultProfile().PointerBits
	}
	return NewOwnedHasher(NewFxHasherProfile(width, 0), nil)
}
func (b FxBuilder) Clone(c Context) Builder { return b }
func (b FxBuilder) CloneFrom(source Builder, c Context) {
	other, ok := source.(FxBuilder)
	if !ok || other.PointerBits != b.PointerBits {
		c.Panic("rusthashmap: FxBuilder CloneFrom requires matching target profiles")
	}
}
func (b FxBuilder) Close(c Context) {}

type Map[K, V any] struct {
	table   *RawTable[Pair[K, V]]
	keys    KeyOps[K]
	values  ValueOps[V]
	builder Builder
	live    bool
}

func NewMap[K, V any](layout Layout, profile Profile, keys KeyOps[K], values ValueOps[V], builder Builder) *Map[K, V] {
	return NewMapWithCapacity(0, layout, profile, keys, values, builder, Context{})
}
func NewMapWithCapacity[K, V any](capacity uint, layout Layout, profile Profile, keys KeyOps[K], values ValueOps[V], builder Builder, c Context) *Map[K, V] {
	if keys.Hash == nil || keys.Equal == nil || builder == nil {
		c.Panic("rusthashmap: incomplete key or builder dictionary")
	}
	m := &Map[K, V]{keys: keys, values: values, builder: builder, live: true}
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { builder.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	m.table = NewRawTableWithCapacity(capacity, layout, profile, m.elementOps(), c)
	complete = true
	return m
}
func (m *Map[K, V]) check(c Context) {
	if m == nil || !m.live {
		c.Panic("rusthashmap: use of consumed map")
	}
}
func (m *Map[K, V]) elementOps() ElementOps[Pair[K, V]] {
	return ElementOps[Pair[K, V]]{
		Hash:         func(pair *Pair[K, V], c Context) uint64 { return m.hash(&pair.Key, c) },
		Clone:        m.clonePair,
		Drop:         m.dropPair,
		TrivialClone: m.keys.TrivialClone && m.values.TrivialClone,
	}
}
func (m *Map[K, V]) hash(key *K, c Context) uint64 {
	if m.keys.HashOne != nil {
		return m.keys.HashOne(m.builder, key, c)
	}
	h := m.builder.Build(c)
	if h == nil || !h.live {
		c.Panic("rusthashmap: BuildHasher returned a consumed result")
	}
	defer cleanupHasher(h, c)
	m.keys.Hash(key, h.value, c)
	return h.value.Finish64()
}
func cleanupHasher(h *OwnedHasher, c Context) {
	p := recover()
	if p != nil && c.ProcessingPanic {
		c.ResumeUnwind(p)
	}
	if p != nil {
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { h.Close(unwind) })
		c.ResumeUnwind(p)
	}
	h.Close(c)
}
func (m *Map[K, V]) clonePair(pair *Pair[K, V], c Context) (result Pair[K, V]) {
	if m.keys.TrivialClone {
		result.Key = pair.Key
	} else {
		if m.keys.Clone == nil {
			c.Panic("rusthashmap: missing key Clone implementation")
		}
		result.Key = m.keys.Clone(&pair.Key, c)
	}
	keyLive := true
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if keyLive && m.keys.Drop != nil {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { m.keys.Drop(&result.Key, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	if m.values.TrivialClone {
		result.Value = pair.Value
	} else {
		if m.values.Clone == nil {
			c.Panic("rusthashmap: missing value Clone implementation")
		}
		result.Value = m.values.Clone(&pair.Value, c)
	}
	keyLive = false
	return result
}
func (m *Map[K, V]) dropPair(pair *Pair[K, V], c Context) {
	// Tuple drop glue visits both fields even when the first field throws.
	valueStarted := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !valueStarted && m.values.Drop != nil {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { m.values.Drop(&pair.Value, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	if m.keys.Drop != nil {
		m.keys.Drop(&pair.Key, c)
	}
	valueStarted = true
	if m.values.Drop != nil {
		m.values.Drop(&pair.Value, c)
	}
}
func (m *Map[K, V]) Len() uint       { m.check(Context{}); return m.table.Len() }
func (m *Map[K, V]) IsEmpty() bool   { return m.Len() == 0 }
func (m *Map[K, V]) Capacity() uint  { m.check(Context{}); return m.table.Capacity() }
func (m *Map[K, V]) Clear(c Context) { m.check(c); m.table.Clear(c) }

// Insert is the trivial-drop value adapter. For nontrivial values use
// InsertOwned so the caller's actual key/value cells are consumed once.
func (m *Map[K, V]) Insert(key K, value V, c Context) (V, bool) {
	old, ok := m.InsertOwned(rustvec.NewOwned(key, rustvec.ValueOps[K]{Drop: m.keys.Drop}), rustvec.NewOwned(value, rustvec.ValueOps[V]{Drop: m.values.Drop}), c)
	if ok {
		return old.Take(), true
	}
	var zero V
	return zero, false
}
func (m *Map[K, V]) InsertOwned(key *rustvec.Owned[K], value *rustvec.Owned[V], c Context) (*rustvec.Owned[V], bool) {
	m.check(c)
	k, v := key.MoveWithContext(c), value.MoveWithContext(c)
	defer cleanupTwo(k, v, c)
	hash := m.hash(k.Borrow(), c)
	index, occupied := m.table.FindOrInsert(hash, func(pair *Pair[K, V]) bool { return m.keys.Equal(k.Borrow(), &pair.Key, c) }, c)
	if occupied {
		pair := m.table.At(index, c)
		old := pair.Value
		pair.Value = v.Take()
		// The unused input key is destroyed before the owned old value returns.
		k.DropWithContext(c)
		return rustvec.NewOwned(old, rustvec.ValueOps[V]{Drop: m.values.Drop}), true
	}
	m.table.InsertAt(hash, index, Pair[K, V]{k.Take(), v.Take()}, c)
	return nil, false
}
func cleanupTwo[K, V any](key *rustvec.Owned[K], value *rustvec.Owned[V], c Context) {
	p := recover()
	if p != nil && c.ProcessingPanic {
		c.ResumeUnwind(p)
	}
	if p != nil {
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { value.DropWithContext(unwind) })
		cleanupOnPanic(unwind, func() { key.DropWithContext(unwind) })
		c.ResumeUnwind(p)
	}
	// Function parameters are destroyed in reverse declaration order.
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			cleanupOnPanic(unwind, func() { key.DropWithContext(unwind) })
			c.ResumeUnwind(p)
		}
	}()
	value.DropWithContext(c)
	key.DropWithContext(c)
}

func (m *Map[K, V]) Get(key *K, c Context) (*V, bool) {
	m.check(c)
	hash := m.hash(key, c)
	index, ok := m.table.Find(hash, func(pair *Pair[K, V]) bool { return m.keys.Equal(key, &pair.Key, c) }, c)
	if !ok {
		return nil, false
	}
	return &m.table.At(index, c).Value, true
}
func (m *Map[K, V]) GetMut(key *K, c Context) (*V, bool) { return m.Get(key, c) }
func (m *Map[K, V]) ContainsKey(key *K, c Context) bool  { _, ok := m.Get(key, c); return ok }
func (m *Map[K, V]) Index(key *K, c Context) *V {
	value, ok := m.Get(key, c)
	if !ok {
		c.Panic("no entry found for key")
	}
	return value
}

// BorrowedLookup is the exact Borrow<Q>/Q:Hash+Eq dictionary for a query. Hash
// and Equal must agree with the stored key's implementations; Equal receives
// the query first and stored key second, as std's equivalent_key does.
type BorrowedLookup[K, Q any] struct {
	Hash func(*Q, Hasher, Context)
	// Monomorphized S::hash_one::<&Q>, with the KeyOps default/override rule.
	HashOne func(Builder, *Q, Context) uint64
	Equal   func(*Q, *K, Context) bool
}

func GetBorrowed[K, V, Q any](m *Map[K, V], query *Q, ops BorrowedLookup[K, Q], c Context) (*V, bool) {
	index, ok := findBorrowed(m, query, ops, c)
	if !ok {
		return nil, false
	}
	return &m.table.At(index, c).Value, true
}
func findBorrowed[K, V, Q any](m *Map[K, V], query *Q, ops BorrowedLookup[K, Q], c Context) (uint, bool) {
	m.check(c)
	if ops.Hash == nil || ops.Equal == nil {
		c.Panic("rusthashmap: incomplete borrowed lookup dictionary")
	}
	var hash uint64
	if ops.HashOne != nil {
		hash = ops.HashOne(m.builder, query, c)
	} else {
		h := m.builder.Build(c)
		if h == nil || !h.live {
			c.Panic("rusthashmap: BuildHasher returned consumed result")
		}
		func() { defer cleanupHasher(h, c); ops.Hash(query, h.value, c); hash = h.value.Finish64() }()
	}
	index, ok := m.table.Find(hash, func(pair *Pair[K, V]) bool { return ops.Equal(query, &pair.Key, c) }, c)
	return index, ok
}

func RemoveBorrowedOwned[K, V, Q any](m *Map[K, V], query *Q, ops BorrowedLookup[K, Q], c Context) (*rustvec.Owned[V], bool) {
	index, ok := findBorrowed(m, query, ops, c)
	if !ok {
		return nil, false
	}
	pair := m.table.RemoveAt(index, c)
	if m.keys.Drop != nil {
		m.keys.Drop(&pair.Key, c)
	}
	return rustvec.NewOwned(pair.Value, rustvec.ValueOps[V]{Drop: m.values.Drop}), true
}
func RemoveBorrowed[K, V, Q any](m *Map[K, V], query *Q, ops BorrowedLookup[K, Q], c Context) (V, bool) {
	value, ok := RemoveBorrowedOwned(m, query, ops, c)
	if ok {
		return value.Take(), true
	}
	var zero V
	return zero, false
}

func (m *Map[K, V]) Remove(key *K, c Context) (V, bool) {
	value, ok := m.RemoveOwned(key, c)
	if ok {
		return value.Take(), true
	}
	var zero V
	return zero, false
}
func (m *Map[K, V]) RemoveOwned(key *K, c Context) (*rustvec.Owned[V], bool) {
	m.check(c)
	hash := m.hash(key, c)
	index, ok := m.table.Find(hash, func(pair *Pair[K, V]) bool { return m.keys.Equal(key, &pair.Key, c) }, c)
	if !ok {
		return nil, false
	}
	pair := m.table.RemoveAt(index, c)
	if m.keys.Drop != nil {
		// The value has already moved into Rust's return place. If the removed
		// key's destructor throws, that prepared result is leaked, not dropped.
		m.keys.Drop(&pair.Key, c)
	}
	return rustvec.NewOwned(pair.Value, rustvec.ValueOps[V]{Drop: m.values.Drop}), true
}
func (m *Map[K, V]) Close(c Context) {
	if m == nil || !m.live {
		return
	}
	m.live = false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			cleanupOnPanic(unwind, func() { m.table.Close(unwind) })
			c.ResumeUnwind(p)
		}
	}()
	m.builder.Close(c)
	m.table.Close(c)
}
