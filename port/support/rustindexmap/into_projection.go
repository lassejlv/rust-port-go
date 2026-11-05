// SPDX-License-Identifier: MIT OR Apache-2.0
// Source IntoKeys/IntoValues eagerly discard the opposite field using the
// concrete Vec in-place map specialization; MaybeUninit preserves slot layout.
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type IntoKeys[K, V any] struct{ inner *IntoIter[K, V] }
type IntoValues[K, V any] struct{ inner *IntoIter[K, V] }

func projectIterator[K, V any](inner *IntoIter[K, V], projection iteratorProjection, c Context) *IntoIter[K, V] {
	inner.check(c)
	prefix := inner.entries[:0]
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { dropIteratorBuckets(inner.config, projection, prefix, unwind) })
				clear(prefix)
				cleanupUnwind(unwind, func() { inner.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	for inner.front < inner.back {
		value, _ := inner.takeEntry(false, c)
		converted := bucket[K, V]{hash: value.hash}
		if projection == keyProjection {
			converted.key = value.key
			if inner.config.Values.Drop != nil {
				inner.config.Values.Drop(&value.value, c)
			}
		} else {
			converted.value = value.value
			if inner.config.Keys.Drop != nil {
				inner.config.Keys.Drop(&value.key, c)
			}
		}
		// The converted return place is initialized BEFORE parameter-field
		// cleanup. A throwing discarded field leaks this one retained field.
		prefix = append(prefix, converted)
	}
	inner.entries = prefix
	inner.front = 0
	inner.back = uint(len(prefix))
	inner.projection = projection
	complete = true
	return inner
}
func (m *Map[K, V]) IntoKeys(contexts ...Context) *IntoKeys[K, V] {
	c := current(contexts)
	return &IntoKeys[K, V]{projectIterator(m.IntoIter(c), keyProjection, c)}
}
func (m *Map[K, V]) IntoValues(contexts ...Context) *IntoValues[K, V] {
	c := current(contexts)
	return &IntoValues[K, V]{projectIterator(m.IntoIter(c), valueProjection, c)}
}
func DefaultIntoKeys[K, V any](config Config[K, V]) *IntoKeys[K, V] {
	inner := DefaultIntoIter(config)
	inner.projection = keyProjection
	return &IntoKeys[K, V]{inner}
}
func DefaultIntoValues[K, V any](config Config[K, V]) *IntoValues[K, V] {
	inner := DefaultIntoIter(config)
	inner.projection = valueProjection
	return &IntoValues[K, V]{inner}
}
func (it *IntoKeys[K, V]) take(n uint, skip, back bool, c Context) *rustvec.Owned[K] {
	if skip {
		it.inner.skip(n, back, c)
	}
	value, ok := it.inner.takeEntry(back, c)
	if !ok {
		return nil
	}
	return rustvec.NewOwned(value.key, rustvec.ValueOps[K]{Drop: it.inner.config.Keys.Drop}).MoveWithContext(c)
}
func (it *IntoValues[K, V]) take(n uint, skip, back bool, c Context) *rustvec.Owned[V] {
	if skip {
		it.inner.skip(n, back, c)
	}
	value, ok := it.inner.takeEntry(back, c)
	if !ok {
		return nil
	}
	return rustvec.NewOwned(value.value, rustvec.ValueOps[V]{Drop: it.inner.config.Values.Drop}).MoveWithContext(c)
}
func (it *IntoKeys[K, V]) Clone(contexts ...Context) *IntoKeys[K, V] {
	return &IntoKeys[K, V]{it.inner.Clone(contexts...)}
}
func (it *IntoValues[K, V]) Clone(contexts ...Context) *IntoValues[K, V] {
	return &IntoValues[K, V]{it.inner.Clone(contexts...)}
}
func (it *IntoKeys[K, V]) CloneFrom(source *IntoKeys[K, V], contexts ...Context) {
	it.inner.CloneFrom(source.inner, contexts...)
}
func (it *IntoValues[K, V]) CloneFrom(source *IntoValues[K, V], contexts ...Context) {
	it.inner.CloneFrom(source.inner, contexts...)
}
func (it *IntoKeys[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	cursor := newBorrowedCursor(it.inner.AsSlice(f.Context))
	return cursor.debug(f, 1)
}
func (it *IntoValues[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	cursor := newBorrowedCursor(it.inner.AsSlice(f.Context))
	return cursor.debug(f, 2)
}

func (it *IntoKeys[K, V]) Len(contexts ...Context) uint { return it.inner.Len(contexts...) }
func (it *IntoKeys[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.inner.SizeHint(contexts...)
}
func (it *IntoKeys[K, V]) Count(contexts ...Context) uint { return it.inner.Count(contexts...) }
func (it *IntoKeys[K, V]) Close(contexts ...Context)      { it.inner.Close(contexts...) }
func (it *IntoKeys[K, V]) LastOwned(contexts ...Context) *rustvec.Owned[K] {
	c := current(contexts)
	value := it.take(0, false, true, c)
	it.inner.Close(c)
	return value
}
func (it *IntoKeys[K, V]) NextOwned(contexts ...Context) *rustvec.Owned[K] {
	return it.take(0, false, false, current(contexts))
}
func (it *IntoKeys[K, V]) NextBackOwned(contexts ...Context) *rustvec.Owned[K] {
	return it.take(0, false, true, current(contexts))
}
func (it *IntoKeys[K, V]) NthOwned(n uint, contexts ...Context) *rustvec.Owned[K] {
	return it.take(n, true, false, current(contexts))
}
func (it *IntoKeys[K, V]) NthBackOwned(n uint, contexts ...Context) *rustvec.Owned[K] {
	return it.take(n, true, true, current(contexts))
}

func (it *IntoValues[K, V]) Len(contexts ...Context) uint { return it.inner.Len(contexts...) }
func (it *IntoValues[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.inner.SizeHint(contexts...)
}
func (it *IntoValues[K, V]) Count(contexts ...Context) uint { return it.inner.Count(contexts...) }
func (it *IntoValues[K, V]) Close(contexts ...Context)      { it.inner.Close(contexts...) }
func (it *IntoValues[K, V]) LastOwned(contexts ...Context) *rustvec.Owned[V] {
	c := current(contexts)
	value := it.take(0, false, true, c)
	it.inner.Close(c)
	return value
}
func (it *IntoValues[K, V]) NextOwned(contexts ...Context) *rustvec.Owned[V] {
	return it.take(0, false, false, current(contexts))
}
func (it *IntoValues[K, V]) NextBackOwned(contexts ...Context) *rustvec.Owned[V] {
	return it.take(0, false, true, current(contexts))
}
func (it *IntoValues[K, V]) NthOwned(n uint, contexts ...Context) *rustvec.Owned[V] {
	return it.take(n, true, false, current(contexts))
}
func (it *IntoValues[K, V]) NthBackOwned(n uint, contexts ...Context) *rustvec.Owned[V] {
	return it.take(n, true, true, current(contexts))
}
