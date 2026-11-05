// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting Core::erase_indices and Vec::Drain paths used by indexmap2.14.0.
// Generic collect/fold and further inherited APIs remain outside this scope.
package rustindexmap

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func (inner *core[K, V]) eraseIndices(start, end uint, c Context) {
	erased := end - start
	shifted := uint(len(inner.entries)) - end
	half := inner.indices.Capacity() / 2
	switch {
	case erased == 0:
	case start+shifted < half && start < erased:
		inner.indices.Clear(c)
		for i := uint(0); i < start; i++ {
			inner.indices.InsertUnique(inner.entries[i].hash, i, c)
		}
		for i := uint(0); i < shifted; i++ {
			inner.indices.InsertUnique(inner.entries[end+i].hash, start+i, c)
		}
	case erased+shifted < half:
		for i := start; i < end; i++ {
			inner.eraseIndex(inner.entries[i].hash, i, c)
		}
		for i := uint(0); i < shifted; i++ {
			inner.updateIndex(inner.entries[end+i].hash, end+i, start+i, c)
		}
	default:
		// End the raw iterator and all cell borrows BEFORE mutations. Numeric
		// bucket identities preserve the source retain traversal, including
		// temporary duplicate shifted index values under colliding hashes.
		positions := []uint{}
		func() {
			it := inner.indices.Iter(c)
			for {
				position, _, ok := it.NextBucket(c)
				if !ok {
					break
				}
				positions = append(positions, position)
			}
		}()
		for _, position := range positions {
			index := inner.indices.At(position, c)
			if *index >= end {
				*index -= erased
			} else if *index >= start {
				inner.indices.RemoveAt(position, c)
			}
		}
	}
	if inner.config.DebugAssertions && inner.indices.Len() != start+shifted {
		c.Panic(fmt.Sprintf("assertion `left == right` failed\n  left: %d\n right: %d", inner.indices.Len(), start+shifted))
	}
}
func (m *Map[K, V]) Truncate(length uint, contexts ...Context) {
	c := current(contexts)
	m.check(c)
	if length < m.core.len(c) {
		m.core.eraseIndices(length, uint(len(m.core.entries)), c)
		removed := m.core.entries[length:]
		m.core.entries = m.core.entries[:length]
		defer clear(removed)
		dropBuckets(m.core.config, removed, c)
	}
}

// Drain holds the exclusive source map borrow until Close or Forget. Neither
// the map nor incompatible views may be used while this descriptor is active.
// Forget is the source mem::forget operation: it leaks unconsumed items/tail
// and preserves the source truncated Vec/adjusted-index state without repairs.
type Drain[K, V any] struct {
	inner      *IntoIter[K, V]
	owner      *core[K, V]
	original   []bucket[K, V]
	start, end uint
	live       bool
}

func (m *Map[K, V]) DrainOwned(rangeValue Range, contexts ...Context) *Drain[K, V] {
	c := current(contexts)
	m.check(c)
	limits, _ := simplifyRangeOwned(rangeValue, uint(len(m.core.entries)), true, c)
	m.core.eraseIndices(limits.Start, limits.End, c)
	original := m.core.entries
	m.core.entries = original[:limits.Start]
	inner := &IntoIter[K, V]{entries: original[limits.Start:limits.End], back: limits.End - limits.Start, config: m.core.config, live: true}
	return &Drain[K, V]{inner: inner, owner: m.core, original: original, start: limits.Start, end: limits.End, live: true}
}
func (d *Drain[K, V]) check(c Context) {
	if d == nil || !d.live {
		c.Panic("rustindexmap: consumed Drain")
	}
}
func (d *Drain[K, V]) Len(contexts ...Context) uint {
	c := current(contexts)
	d.check(c)
	return d.inner.Len(c)
}
func (d *Drain[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	c := current(contexts)
	d.check(c)
	return d.inner.SizeHint(c)
}
func (d *Drain[K, V]) AsSlice(contexts ...Context) Slice[K, V] {
	c := current(contexts)
	d.check(c)
	return d.inner.AsSlice(c)
}
func (d *Drain[K, V]) NextOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	d.check(c)
	return d.inner.NextOwned(c)
}
func (d *Drain[K, V]) NextBackOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	d.check(c)
	return d.inner.NextBackOwned(c)
}
func (d *Drain[K, V]) NthOwned(n uint, contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	d.check(c)
	for i := uint(0); i < n; i++ {
		value := d.NextOwned(c)
		if value == nil {
			return nil
		}
		value.DropWithContext(c)
	}
	return d.NextOwned(c)
}
func (d *Drain[K, V]) NthBackOwned(n uint, contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	d.check(c)
	for i := uint(0); i < n; i++ {
		value := d.NextBackOwned(c)
		if value == nil {
			return nil
		}
		value.DropWithContext(c)
	}
	return d.NextBackOwned(c)
}
func (d *Drain[K, V]) Close(contexts ...Context) {
	c := current(contexts)
	if d == nil || !d.live {
		return
	}
	d.live = false
	defer func() {
		p := recover()
		if p != nil && c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		tail := d.original[d.end:]
		copy(d.original[d.start:], tail)
		length := d.start + uint(len(tail))
		clear(d.original[length:])
		d.owner.entries = d.original[:length]
		d.original = nil
		d.owner = nil
		if p != nil {
			c.ResumeUnwind(p)
		}
	}()
	d.inner.Close(c)
}
func (d *Drain[K, V]) Forget(contexts ...Context) {
	c := current(contexts)
	d.check(c)
	d.live = false
	d.inner.live = false
	d.inner.entries = nil
	d.original = nil
	d.owner = nil
}
func (d *Drain[K, V]) Count(contexts ...Context) uint {
	c := current(contexts)
	n := d.Len(c)
	d.Close(c)
	return n
}
func (d *Drain[K, V]) LastOwned(contexts ...Context) *rustvec.Owned[Pair[K, V]] {
	c := current(contexts)
	value := d.NextBackOwned(c)
	d.Close(c)
	return value
}
func (d *Drain[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	d.check(f.Context)
	return d.inner.WriteDebugFormatter(f)
}
