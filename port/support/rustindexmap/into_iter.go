// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting owning iterator paths from indexmap2.14.0 src/map/iter.rs and the
// selected Vec::IntoIter behavior. Generic collect/fold and additional APIs remain pending; this does not claim a whole std Vec implementation.
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type IntoIter[K, V any] struct {
	entries     []bucket[K, V]
	front, back uint
	config      Config[K, V]
	live        bool
	projection  iteratorProjection
}
type iteratorProjection uint8

const (
	pairProjection iteratorProjection = iota
	keyProjection
	valueProjection
)

// Projected storage corresponds to Bucket<K,MaybeUninit<V>> or its converse.
// Private zero slots stand for logically uninitialized fields and are never
// exposed, cloned, read or destroyed as live values.
func dropIteratorBuckets[K, V any](config Config[K, V], projection iteratorProjection, values []bucket[K, V], c Context) {
	if projection == pairProjection {
		dropBuckets(config, values, c)
		return
	}
	n := 0
	drop := func(v *bucket[K, V], context Context) {
		if projection == keyProjection {
			if config.Keys.Drop != nil {
				config.Keys.Drop(&v.key, context)
			}
		} else if config.Values.Drop != nil {
			config.Values.Drop(&v.value, context)
		}
	}
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			for ; n < len(values); n++ {
				v := &values[n]
				cleanupUnwind(unwind, func() { drop(v, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	for n < len(values) {
		v := &values[n]
		n++
		drop(v, c)
	}
}

func DefaultIntoIter[K, V any](config Config[K, V]) *IntoIter[K, V] {
	return &IntoIter[K, V]{config: config, live: true}
}
func (m *Map[K, V]) IntoIter(contexts ...Context) *IntoIter[K, V] {
	c := current(contexts)
	m.check(c)
	m.live = false
	entries := m.core.entries
	m.core.entries = nil
	m.core.indices.Close(c)
	result := &IntoIter[K, V]{entries: entries, back: uint(len(entries)), config: m.core.config, live: true}
	// into_entries has already initialized its Vec return place. A throwing
	// remaining builder destructor leaks that prepared value, as in Rust.
	m.builder.Close(c)
	return result
}
func (it *IntoIter[K, V]) check(context Context) {
	if it == nil || !it.live {
		context.Panic("rustindexmap: use of consumed owning iterator")
	}
}
func (it *IntoIter[K, V]) Len(contexts ...Context) uint {
	c := current(contexts)
	it.check(c)
	return it.back - it.front
}
func (it *IntoIter[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	n := it.Len(contexts...)
	return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
}
func (it *IntoIter[K, V]) AsSlice(contexts ...Context) Slice[K, V] {
	it.check(current(contexts))
	return Slice[K, V]{entries: it.entries[it.front:it.back], config: it.config}
}
func (it *IntoIter[K, V]) AsMutSlice(contexts ...Context) Slice[K, V] {
	view := it.AsSlice(contexts...)
	view.mutable = true
	return view
}
func (it *IntoIter[K, V]) ownPair(value bucket[K, V], context Context) *rustvec.Owned[Pair[K, V]] {
	config := it.config
	return rustvec.NewOwned(Pair[K, V]{value.key, value.value}, rustvec.ValueOps[Pair[K, V]]{Drop: func(value *Pair[K, V], c Context) { dropPair(config, &value.Key, &value.Value, c) }}).MoveWithContext(context)
}
func (it *IntoIter[K, V]) NextOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	value, ok := it.takeEntry(false, c)
	if !ok {
		return nil
	}
	return it.ownPair(value, c)
}
func (it *IntoIter[K, V]) NextBackOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	value, ok := it.takeEntry(true, c)
	if !ok {
		return nil
	}
	return it.ownPair(value, c)
}
func (it *IntoIter[K, V]) takeEntry(back bool, c Context) (bucket[K, V], bool) {
	it.check(c)
	if it.front == it.back {
		return bucket[K, V]{}, false
	}
	var index uint
	if back {
		it.back--
		index = it.back
	} else {
		index = it.front
		it.front++
	}
	value := it.entries[index]
	it.entries[index] = bucket[K, V]{}
	return value, true
}
func (it *IntoIter[K, V]) skip(n uint, back bool, c Context) {
	it.check(c)
	if it.config.Profile.PointerBits == 32 && uint64(n) > 0xffffffff {
		c.Panic("rustindexmap: iterator argument exceeds source usize domain")
	}
	if available := it.back - it.front; n > available {
		n = available
	}
	var skipped []bucket[K, V]
	if back {
		it.back -= n
		skipped = it.entries[it.back : it.back+n]
	} else {
		start := it.front
		it.front += n
		skipped = it.entries[start:it.front]
	}
	// Vec's optimized backward advance drops the skipped tail in FORWARD
	// memory order. Update its end pointer before calling any destructors.
	defer clear(skipped)
	dropIteratorBuckets(it.config, it.projection, skipped, c)
}
func (it *IntoIter[K, V]) NthOwned(n uint, contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	it.skip(n, false, c)
	return it.NextOwned(c)
}
func (it *IntoIter[K, V]) NthBackOwned(n uint, contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	it.skip(n, true, c)
	return it.NextBackOwned(c)
}
func (it *IntoIter[K, V]) Close(contexts ...Context) {
	c := current(contexts)
	if it == nil || !it.live {
		return
	}
	it.live = false
	remaining := it.entries[it.front:it.back]
	it.front = it.back
	defer func() { clear(remaining); it.entries = nil }()
	dropIteratorBuckets(it.config, it.projection, remaining, c)
}
func (it *IntoIter[K, V]) Count(contexts ...Context) uint {
	c := current(contexts)
	n := it.Len(c)
	it.Close(c)
	return n
}
func (it *IntoIter[K, V]) LastOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	value := it.NextBackOwned(c)
	it.Close(c)
	return value
}
func (it *IntoIter[K, V]) Clone(contexts ...Context) *IntoIter[K, V] {
	c := current(contexts)
	it.check(c)
	if it.projection != valueProjection && !it.config.Keys.TrivialClone && it.config.Keys.Clone == nil || it.projection != keyProjection && !it.config.Values.TrivialClone && it.config.Values.Clone == nil {
		c.Panic("rustindexmap: unavailable source owning iterator Clone traits")
	}
	result := DefaultIntoIter(it.config)
	result.projection = it.projection
	result.entries = make([]bucket[K, V], 0, int(it.back-it.front))
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { result.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	for i := it.front; i < it.back; i++ {
		var value bucket[K, V]
		switch it.projection {
		case keyProjection:
			value.key = cloneKeyValue(it.config.Keys, &it.entries[i].key, c)
		case valueProjection:
			value.value = cloneValueValue(it.config.Values, &it.entries[i].value, c)
		default:
			value = cloneBucketValue(it.config, &it.entries[i], c)
		}
		result.entries = append(result.entries, value)
		result.back++
	}
	complete = true
	return result
}
func (it *IntoIter[K, V]) CloneFrom(source *IntoIter[K, V], contexts ...Context) {
	c := current(contexts)
	it.check(c)
	source.check(c)
	if it == source {
		c.Panic("rustindexmap: aliased iterator CloneFrom source")
	}
	if it.projection != source.projection || it.config.Profile != source.config.Profile || it.config.BucketLayout != source.config.BucketLayout {
		c.Panic("rustindexmap: different source owning iterator profiles")
	}
	replacement := source.Clone(c)
	defer func() {
		p := recover()
		if p != nil && c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		*it = *replacement
		replacement.live = false
		replacement.entries = nil
		if p != nil {
			c.ResumeUnwind(p)
		}
	}()
	it.Close(c)
}
func (it *IntoIter[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	cursor := newBorrowedCursor(it.AsSlice(f.Context))
	return cursor.debug(f, 0)
}
