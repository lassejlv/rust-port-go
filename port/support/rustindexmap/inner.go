// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting source translation of indexmap2.14.0 src/inner.rs. The dense
// entries and actual hashbrown-style usize-index table remain distinct.
package rustindexmap

import (
	"fmt"

	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

func newCore[K, V any](capacity uint, config Config[K, V], c Context) *core[K, V] {
	if config.BucketLayout.Size == 0 || config.BucketLayout.Align == 0 || config.BucketLayout.Align&(config.BucketLayout.Align-1) != 0 || config.BucketLayout.Size%config.BucketLayout.Align != 0 {
		c.Panic("rustindexmap: invalid source Bucket layout")
	}
	inner := &core[K, V]{config: config}
	word := config.Profile.PointerBits / 8
	inner.indices = rusthashmap.NewRawTableWithCapacity(capacity, Layout{Size: word, Align: word}, config.Profile, rusthashmap.ElementOps[uint]{
		Hash:         func(index *uint, c Context) uint64 { return inner.entry(*index, c).hash },
		TrivialClone: true,
	}, c)
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { inner.indices.Close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	inner.allocateEntries(capacity, c)
	complete = true
	return inner
}
func (inner *core[K, V]) hashValue(value uint64) uint64 {
	if inner.config.Profile.PointerBits == 32 {
		return value & 0xffffffff
	}
	return value
}
func (inner *core[K, V]) len(c Context) uint {
	if inner.config.DebugAssertions && uint(len(inner.entries)) != inner.indices.Len() {
		c.Panic(fmt.Sprintf("assertion `left == right` failed\n  left: %d\n right: %d", len(inner.entries), inner.indices.Len()))
	}
	return inner.indices.Len()
}
func (inner *core[K, V]) capacity() uint {
	n := inner.indices.Capacity()
	if uint(cap(inner.entries)) < n {
		n = uint(cap(inner.entries))
	}
	return n
}
func (inner *core[K, V]) entry(index uint, c Context) *bucket[K, V] {
	if index >= uint(len(inner.entries)) {
		c.Panic(fmt.Sprintf("index out of bounds: the len is %d but the index is %d", len(inner.entries), index))
	}
	return &inner.entries[index]
}
func (inner *core[K, V]) maxEntries() uint {
	wordMax := uint64(^uint64(0) >> 1)
	if inner.config.Profile.PointerBits == 32 {
		wordMax = 1<<31 - 1
	}
	return uint(wordMax / uint64(inner.config.BucketLayout.Size))
}
func (inner *core[K, V]) allocateEntries(capacity uint, c Context) {
	if capacity > inner.maxEntries() || capacity > uint(^uint(0)>>1) {
		c.Panic("capacity overflow")
	}
	values := make([]bucket[K, V], len(inner.entries), int(capacity))
	copy(values, inner.entries)
	clear(inner.entries)
	inner.entries = values
}
func (inner *core[K, V]) reserveEntries(additional uint, c Context) {
	tryCapacity := inner.indices.Capacity()
	if maximum := inner.maxEntries(); maximum < tryCapacity {
		tryCapacity = maximum
	}
	length := uint(len(inner.entries))
	if tryCapacity >= length && tryCapacity-length > additional {
		if uint(cap(inner.entries)) < tryCapacity {
			inner.allocateEntries(tryCapacity, c)
		}
		return
	}
	if additional > inner.maxEntries()-length {
		c.Panic("capacity overflow")
	}
	required := length + additional
	if required > uint(cap(inner.entries)) {
		inner.allocateEntries(required, c)
	}
}
func (inner *core[K, V]) reserve(additional uint, exact bool, c Context) {
	inner.indices.Reserve(additional, c)
	length := uint(len(inner.entries))
	if exact {
		if additional > inner.maxEntries()-length {
			c.Panic("capacity overflow")
		}
		required := length + additional
		if required > uint(cap(inner.entries)) {
			inner.allocateEntries(required, c)
		}
	} else if additional > uint(cap(inner.entries))-length {
		inner.reserveEntries(additional, c)
	}
}
func (inner *core[K, V]) push(hash uint64, key K, value V, c Context) {
	if len(inner.entries) == cap(inner.entries) {
		inner.reserveEntries(1, c)
	}
	inner.entries = append(inner.entries, bucket[K, V]{hash: hash, key: key, value: value})
}
func (inner *core[K, V]) find(hash uint64, equal func(*K) bool, c Context) (uint, uint, bool) {
	bucketIndex, found := inner.indices.Find(hash, func(index *uint) bool { return equal(&inner.entry(*index, c).key) }, c)
	if !found {
		return 0, 0, false
	}
	return *inner.indices.At(bucketIndex, c), bucketIndex, true
}
func (inner *core[K, V]) updateIndex(hash uint64, old, next uint, c Context) {
	i, found := inner.indices.Find(hash, func(index *uint) bool { return *index == old }, c)
	if !found {
		c.Panic("index not found")
	}
	*inner.indices.At(i, c) = next
}
func (inner *core[K, V]) eraseIndex(hash uint64, index uint, c Context) {
	i, found := inner.indices.Find(hash, func(value *uint) bool { return *value == index }, c)
	if found {
		inner.indices.RemoveAt(i, c)
	} else if inner.config.DebugAssertions {
		c.Panic("index not found")
	}
}
func (inner *core[K, V]) decrement(start, end uint, c Context) {
	if end-start > inner.indices.Capacity()/2 {
		it := inner.indices.Iter(c)
		for {
			index, ok := it.Next(c)
			if !ok {
				break
			}
			if start <= *index && *index < end {
				*index--
			}
		}
	} else {
		for index := start; index < end; index++ {
			inner.updateIndex(inner.entry(index, c).hash, index, index-1, c)
		}
	}
}
func (inner *core[K, V]) increment(start, end uint, c Context) {
	if end-start > inner.indices.Capacity()/2 {
		it := inner.indices.Iter(c)
		for {
			index, ok := it.Next(c)
			if !ok {
				break
			}
			if start <= *index && *index < end {
				*index++
			}
		}
	} else {
		for index := end; index > start; {
			index--
			inner.updateIndex(inner.entry(index, c).hash, index, index+1, c)
		}
	}
}
func (inner *core[K, V]) swapRemoveFinish(index uint, c Context) Pair[K, V] {
	entry := *inner.entry(index, c)
	last := len(inner.entries) - 1
	inner.entries[index] = inner.entries[last]
	inner.entries[last] = bucket[K, V]{}
	inner.entries = inner.entries[:last]
	if index < uint(len(inner.entries)) {
		inner.updateIndex(inner.entries[index].hash, uint(last), index, c)
	}
	return Pair[K, V]{Key: entry.key, Value: entry.value}
}
func (inner *core[K, V]) shiftRemoveFinish(index uint, c Context) Pair[K, V] {
	entry := *inner.entry(index, c)
	inner.decrement(index+1, uint(len(inner.entries)), c)
	copy(inner.entries[index:], inner.entries[index+1:])
	inner.entries[len(inner.entries)-1] = bucket[K, V]{}
	inner.entries = inner.entries[:len(inner.entries)-1]
	return Pair[K, V]{Key: entry.key, Value: entry.value}
}
func (inner *core[K, V]) removeIndex(index uint, shift bool, c Context) (Pair[K, V], bool) {
	if index >= uint(len(inner.entries)) {
		return Pair[K, V]{}, false
	}
	inner.eraseIndex(inner.entries[index].hash, index, c)
	if shift {
		return inner.shiftRemoveFinish(index, c), true
	}
	return inner.swapRemoveFinish(index, c), true
}
func (inner *core[K, V]) pop(c Context) (Pair[K, V], bool) {
	if len(inner.entries) == 0 {
		return Pair[K, V]{}, false
	}
	last := len(inner.entries) - 1
	entry := inner.entries[last]
	inner.entries[last] = bucket[K, V]{}
	inner.entries = inner.entries[:last]
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { dropPair(inner.config, &entry.key, &entry.value, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	inner.eraseIndex(entry.hash, uint(last), c)
	complete = true
	return Pair[K, V]{Key: entry.key, Value: entry.value}, true
}
func (inner *core[K, V]) clear(c Context) {
	inner.indices.Clear(c)
	values := inner.entries
	inner.entries = inner.entries[:0]
	dropBuckets(inner.config, values, c)
	clear(values)
}
func (inner *core[K, V]) close(c Context) {
	inner.indices.Close(c)
	values := inner.entries
	inner.entries = nil
	dropBuckets(inner.config, values, c)
}
