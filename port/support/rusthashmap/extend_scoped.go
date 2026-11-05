// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

// ExtendScoped is the same concrete std/hashbrown extension strategy as
// Extend, with the primary borrowed insertion consumer retaining the context
// of each actual callback invocation. I::for_each may legally invoke it inside
// a destructor during a nested caught unwind. The consumer captures only the
// borrowed map and owns both complete key/value parameters at that call.
func (m *Map[K, V]) ExtendScoped(source rustvec.IntoIterator[Pair[K, V]], c Context) {
	m.check(c)
	defer cleanupInput(source, c)
	iterator := source.IntoIter(c)
	defer cleanupIterator(iterator, c)
	lower := iterator.SizeHint(c).Lower
	if !m.IsEmpty() {
		lower = lower/2 + lower%2
	}
	m.table.Reserve(lower, c)
	iterator.ForEachScopedWithContext(c, rustvec.NewBorrowedConsumer(func(pair Pair[K, V], current Context) {
		key := rustvec.NewOwned(pair.Key, rustvec.ValueOps[K]{Drop: m.keys.Drop})
		value := rustvec.NewOwned(pair.Value, rustvec.ValueOps[V]{Drop: m.values.Drop})
		old, present := m.InsertOwned(key, value, current)
		if present {
			old.DropWithContext(current)
		}
	}))
}

// ExtendScoped preserves the source Set's real map adapter and independent
// I::fold selection. Every insertion receives that invocation's current scope.
func (s *Set[T]) ExtendScoped(source rustvec.IntoIterator[T], c Context) {
	s.inner.check(c)
	defer cleanupInput(source, c)
	iterator := source.IntoIter(c)
	defer cleanupIterator(iterator, c)
	mapped := iterator.MapToUnitScopedWithContext(c, rustvec.ElementOps[T]{Drop: s.inner.keys.Drop})
	defer cleanupUnitMap(mapped, c)
	lower := mapped.SizeHint(c).Lower
	if !s.IsEmpty() {
		lower = lower/2 + lower%2
	}
	s.inner.table.Reserve(lower, c)
	mapped.ForEachScopedWithContext(c, rustvec.NewBorrowedConsumer(func(value T, current Context) {
		s.InsertOwned(rustvec.NewOwned(value, rustvec.ValueOps[T]{Drop: s.inner.keys.Drop}), current)
	}))
}

func MapFromIteratorScoped[K, V any](source rustvec.IntoIterator[Pair[K, V]], layout Layout, profile Profile, keys KeyOps[K], values ValueOps[V], defaultBuilder func(Context) Builder, c Context) *Map[K, V] {
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
	m.ExtendScoped(source, c)
	complete = true
	return m
}

func SetFromIteratorScoped[T any](source rustvec.IntoIterator[T], layout Layout, profile Profile, keys KeyOps[T], defaultBuilder func(Context) Builder, c Context) *Set[T] {
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
	s.ExtendScoped(source, c)
	complete = true
	return s
}
