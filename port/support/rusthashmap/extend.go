// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

func cleanupInput[T any](input rustvec.IntoIterator[T], c Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { input.CloseWithContext(unwind) })
		c.ResumeUnwind(p)
	}
	input.CloseWithContext(c)
}
func cleanupIterator[T any](iterator rustvec.Iterator[T], c Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { iterator.CloseWithContext(unwind) })
		c.ResumeUnwind(p)
	}
	iterator.CloseWithContext(c)
}
func cleanupUnitMap[T any](iterator rustvec.UnitMap[T], c Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { iterator.CloseWithContext(unwind) })
		c.ResumeUnwind(p)
	}
	iterator.CloseWithContext(c)
}

// Extend follows std/hashbrown's IntoIter, lower SizeHint, reserve, for_each
// sequence. A nonempty map reserves ceil(lower/2). The source consumer captures
// only the borrowed map; it consumes and cleans up both key/value parameters.
func (m *Map[K, V]) Extend(source rustvec.IntoIterator[Pair[K, V]], c Context) {
	m.check(c)
	defer cleanupInput(source, c)
	iterator := source.IntoIter(c)
	defer cleanupIterator(iterator, c)
	lower := iterator.SizeHint(c).Lower
	if !m.IsEmpty() {
		lower = lower/2 + lower%2
	}
	m.table.Reserve(lower, c)
	iterator.ForEachWithContext(c, func(pair Pair[K, V]) {
		key := rustvec.NewOwned(pair.Key, rustvec.ValueOps[K]{Drop: m.keys.Drop})
		value := rustvec.NewOwned(pair.Value, rustvec.ValueOps[V]{Drop: m.values.Drop})
		old, present := m.InsertOwned(key, value, c)
		if present {
			old.DropWithContext(c)
		}
	})
}

// MapFromIterator implements the actual std FromIterator body. defaultBuilder
// is the monomorphized S::default trait method, with no hidden owned captures.
// The input is owned from entry, even if constructing the hasher throws.
func MapFromIterator[K, V any](source rustvec.IntoIterator[Pair[K, V]], layout Layout, profile Profile, keys KeyOps[K], values ValueOps[V], defaultBuilder func(Context) Builder, c Context) *Map[K, V] {
	defer cleanupInput(source, c)
	if defaultBuilder == nil {
		c.Panic("rusthashmap: missing source BuildHasher Default implementation")
	}
	m := NewMapWithCapacity(0, layout, profile, keys, values, defaultBuilder(c), c)
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { m.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	m.Extend(source, c)
	complete = true
	return m
}

// Extend preserves Set's internal Map adapter: its for_each delegates to the
// original input's fold, not the original input's for_each override.
func (s *Set[T]) Extend(source rustvec.IntoIterator[T], c Context) {
	s.inner.check(c)
	defer cleanupInput(source, c)
	iterator := source.IntoIter(c)
	defer cleanupIterator(iterator, c)
	mapped := iterator.MapToUnitWithContext(c, rustvec.ElementOps[T]{Drop: s.inner.keys.Drop})
	defer cleanupUnitMap(mapped, c)
	lower := mapped.SizeHint(c).Lower
	if !s.IsEmpty() {
		lower = lower/2 + lower%2
	}
	s.inner.table.Reserve(lower, c)
	mapped.ForEachWithContext(c, func(value T) { s.InsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[T]{Drop: s.inner.keys.Drop}), c) })
}

func SetFromIterator[T any](source rustvec.IntoIterator[T], layout Layout, profile Profile, keys KeyOps[T], defaultBuilder func(Context) Builder, c Context) *Set[T] {
	defer cleanupInput(source, c)
	if defaultBuilder == nil {
		c.Panic("rusthashmap: missing source BuildHasher Default implementation")
	}
	s := NewSetWithCapacity(0, layout, profile, keys, defaultBuilder(c), c)
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { s.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	s.Extend(source, c)
	complete = true
	return s
}
