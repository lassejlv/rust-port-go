// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL supporting port of indexmap2.14.0's std/map operations. Exact source
// archive d466e9454f08e4a911e14806c24e16fba1b4c121d1ea474396f396069cf949d9.
// No whole dependency source credit is claimed. See tools/port-rust-indexmap.
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type Context = rustfmt.Context
type Profile = rusthashmap.Profile
type Layout = rusthashmap.Layout
type Builder = rusthashmap.Builder
type Hasher = rusthashmap.Hasher

// KeyOps and ValueOps describe the genuine source traits of K and V. All Ref
// pointers are temporary borrows; they cannot survive relocation, mutation,
// move or Drop. Static dictionary callbacks own no hidden destructible captures.
// Such data belongs in the concrete source values. Source panic sites use the
// explicit context.Panic; raw Go callback-local defer/recover keeps host timing.
// CloneFrom is independent of Clone, as Bucket::clone_from calls it directly.
// Nil selects Clone's actual provided default: clone, then drop-and-replace.
type KeyOps[K any] struct {
	rusthashmap.KeyOps[K]
	CloneFrom func(destination, source *K, context Context)
}
type ValueOps[V any] struct {
	rusthashmap.ValueOps[V]
	CloneFrom func(destination, source *V, context Context)
}
type Config[K, V any] struct {
	// BucketLayout is the actual Rust Bucket<K,V> layout including HashValue.
	// It is not computed from the Go representation, which has no Rust ABI.
	BucketLayout    Layout
	Profile         Profile
	DebugAssertions bool
	Keys            KeyOps[K]
	Values          ValueOps[V]
}
type Pair[K, V any] struct {
	Key   K
	Value V
}
type bucket[K, V any] struct {
	hash  uint64
	key   K
	value V
}
type core[K, V any] struct {
	indices *rusthashmap.RawTable[uint]
	entries []bucket[K, V]
	config  Config[K, V]
}
type Map[K, V any] struct {
	core    *core[K, V]
	builder Builder
	live    bool
}

func current(contexts []Context) Context {
	if len(contexts) > 1 {
		panic("rustindexmap: expected at most one current context")
	}
	if len(contexts) == 1 {
		return contexts[0]
	}
	return Context{}
}
func cleanupUnwind(context Context, call func()) {
	defer func() {
		if recover() != nil {
			context.AbortCleanup()
		}
	}()
	call()
}
func cleanupCall(c Context, call func(Context)) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupUnwind(unwind, func() { call(unwind) })
		c.ResumeUnwind(p)
	}
	call(c)
}
func dropPair[K, V any](config Config[K, V], key *K, value *V, c Context) {
	valuePending := true
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			if valuePending && config.Values.Drop != nil {
				cleanupUnwind(unwind, func() { config.Values.Drop(value, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	if config.Keys.Drop != nil {
		config.Keys.Drop(key, c)
	}
	valuePending = false
	if config.Values.Drop != nil {
		config.Values.Drop(value, c)
	}
}
func dropBuckets[K, V any](config Config[K, V], values []bucket[K, V], c Context) {
	n := 0
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			for ; n < len(values); n++ {
				v := &values[n]
				cleanupUnwind(unwind, func() { dropPair(config, &v.key, &v.value, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	for n < len(values) {
		v := &values[n]
		n++
		dropPair(config, &v.key, &v.value, c)
	}
}
func (m *Map[K, V]) check(c Context) {
	if m == nil || !m.live {
		c.Panic("rustindexmap: use of consumed IndexMap")
	}
}

// WithHasher consumes the actual builder object into the map. Do not reuse
// prior owning aliases. Values returned by Hasher are shared source borrows;
// its mutable Go methods cannot be used while that source borrow is retained.
// Constructors impose no Hash/Eq/BuildHasher/Clone bounds. Independent traits
// are required only by the source operations which declare those bounds.
func WithHasher[K, V any](config Config[K, V], builder Builder, contexts ...Context) *Map[K, V] {
	return WithCapacityAndHasher(0, config, builder, contexts...)
}
func WithCapacityAndHasher[K, V any](capacity uint, config Config[K, V], builder Builder, contexts ...Context) *Map[K, V] {
	c := current(contexts)
	if builder == nil {
		c.Panic("rustindexmap: incomplete source dictionary")
	}
	m := &Map[K, V]{builder: builder, live: true}
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { builder.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	m.core = newCore(capacity, config, c)
	complete = true
	return m
}
func (m *Map[K, V]) Hasher(contexts ...Context) Builder { m.check(current(contexts)); return m.builder }
func (m *Map[K, V]) Len(contexts ...Context) uint {
	c := current(contexts)
	m.check(c)
	return m.core.len(c)
}
func (m *Map[K, V]) IsEmpty(contexts ...Context) bool { return m.Len(contexts...) == 0 }
func (m *Map[K, V]) Capacity(contexts ...Context) uint {
	m.check(current(contexts))
	return m.core.capacity()
}
func (m *Map[K, V]) Close(contexts ...Context) {
	c := current(contexts)
	if m == nil || !m.live {
		return
	}
	m.live = false
	defer cleanupCall(c, m.builder.Close)
	m.core.close(c)
}

func (m *Map[K, V]) hash(key *K, c Context) uint64 {
	requireSourceBuildHasher(m.builder, c)
	if m.core.config.Keys.Hash == nil {
		c.Panic("rustindexmap: missing key Hash witness")
	}
	if m.core.config.Keys.HashOne != nil {
		return m.core.hashValue(m.core.config.Keys.HashOne(m.builder, key, c))
	}
	h := m.builder.Build(c)
	if h == nil {
		c.Panic("rustindexmap: BuildHasher returned nil")
	}
	defer cleanupCall(c, h.Close)
	actual := h.Borrow(c)
	m.core.config.Keys.Hash(key, actual, c)
	return m.core.hashValue(actual.Finish64())
}
