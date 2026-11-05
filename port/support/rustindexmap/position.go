// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting stored-index movement and split_off from indexmap2.14.0 inner.rs/map.rs.
package rustindexmap

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const (
	IndexMapSplitPanicSite = "indexmap.split_off.caller"
	IndexMapMovePanicSite  = "indexmap.move_index.caller"
	IndexMapSwapPanicSite  = "indexmap.swap_indices.caller"
)

func positionIndexEntry[K, V any](inner *core[K, V], index uint, site string, c Context) *bucket[K, V] {
	if index >= uint(len(inner.entries)) {
		rangePanic(c, site, func(Context) any {
			return fmt.Sprintf("index out of bounds: the len is %d but the index is %d", len(inner.entries), index)
		})
	}
	return &inner.entries[index]
}
func (m *Map[K, V]) SplitOff(at uint, contexts ...Context) *Map[K, V] {
	c := current(contexts)
	m.check(c)
	requireSourceBuilderClone(m.builder, c)
	length := uint(len(m.core.entries))
	if at > length {
		rangePanic(c, IndexMapSplitPanicSite, func(Context) any {
			return fmt.Sprintf("index out of bounds: the len is %d but the index is %d. Expected index <= len", length, at)
		})
	}
	m.core.eraseIndices(at, length, c)
	next := newCore(length-at, m.core.config, c)
	next.entries = next.entries[:length-at]
	copy(next.entries, m.core.entries[at:])
	clear(m.core.entries[at:])
	m.core.entries = m.core.entries[:at]
	for i := range next.entries {
		next.indices.InsertUnique(next.entries[i].hash, uint(i), c)
	}
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { next.close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	// Source initializer moves Core first, then clones only S. A throwing S::clone
	// destroys the prepared right Core while the original prefix remains split.
	builder := builderInvoke(c, func() Builder { return m.builder.Clone(c) })
	if builder == nil {
		c.Panic("rustindexmap: builder Clone returned nil source state")
	}
	result := &Map[K, V]{core: next, builder: builder, live: true}
	complete = true
	return result
}
func (m *Map[K, V]) MoveIndex(from, to uint, contexts ...Context) {
	c := current(contexts)
	m.check(c)
	inner := m.core
	hash := positionIndexEntry(inner, from, IndexMapMovePanicSite, c).hash
	if from == to {
		return
	}
	positionIndexEntry(inner, to, IndexMapMovePanicSite, c)
	position, found := inner.indices.Find(hash, func(value *uint) bool { return *value == from }, c)
	if !found {
		rangePanic(c, IndexMapMovePanicSite, func(Context) any { return "index not found" })
	}
	value := inner.entries[from]
	if from < to {
		inner.decrement(from+1, to+1, c)
		copy(inner.entries[from:to], inner.entries[from+1:to+1])
	} else {
		inner.increment(to, from, c)
		copy(inner.entries[to+1:from+1], inner.entries[to:from])
	}
	inner.entries[to] = value
	*inner.indices.At(position, c) = to
}
func (m *Map[K, V]) SwapIndices(a, b uint, contexts ...Context) {
	c := current(contexts)
	m.check(c)
	inner := m.core
	if a == b && a < uint(len(inner.entries)) {
		return
	}
	hashA := positionIndexEntry(inner, a, IndexMapSwapPanicSite, c).hash
	hashB := positionIndexEntry(inner, b, IndexMapSwapPanicSite, c).hash
	positionA, foundA := inner.indices.Find(hashA, func(value *uint) bool { return *value == a }, c)
	positionB, foundB := inner.indices.Find(hashB, func(value *uint) bool { return *value == b }, c)
	if !foundA || !foundB || positionA == positionB {
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "indices not found", Kind: rustfmt.PanicStaticStr, Location: c.SourcePanicLocation(IndexMapSwapPanicSite), CanUnwind: true})
	}
	valueA, valueB := inner.indices.At(positionA, c), inner.indices.At(positionB, c)
	*valueA, *valueB = *valueB, *valueA
	inner.entries[a], inner.entries[b] = inner.entries[b], inner.entries[a]
}
