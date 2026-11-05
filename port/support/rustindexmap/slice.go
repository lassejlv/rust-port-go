// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting borrowed Slice operations from indexmap2.14.0 src/map/slice.rs.
package rustindexmap

// Slice is a temporary borrowed view of the actual dense source entries. Its
// copies do not own K/V. A shared view permits reads only; a mutable view permits
// disjoint mutable V borrows. No view or returned pointer may outlive its owner
// or overlap a map mutation, relocation, Move or Drop forbidden by Rust borrows.
// The Go descriptor/layout is not the source unsized Slice ABI.
type Slice[K, V any] struct {
	entries []bucket[K, V]
	config  Config[K, V]
	mutable bool
}

func (m *Map[K, V]) AsSlice(contexts ...Context) Slice[K, V] {
	m.check(current(contexts))
	return Slice[K, V]{entries: m.core.entries, config: m.core.config}
}
func (m *Map[K, V]) AsMutSlice(contexts ...Context) Slice[K, V] {
	m.check(current(contexts))
	return Slice[K, V]{entries: m.core.entries, config: m.core.config, mutable: true}
}
func (s Slice[K, V]) Len() uint     { return uint(len(s.entries)) }
func (s Slice[K, V]) IsEmpty() bool { return len(s.entries) == 0 }
func (s Slice[K, V]) GetIndex(index uint) (*K, *V, bool) {
	if index >= uint(len(s.entries)) {
		return nil, nil, false
	}
	entry := &s.entries[index]
	return &entry.key, &entry.value, true
}
func (s Slice[K, V]) GetIndexMut(index uint, contexts ...Context) (*K, *V, bool) {
	if !s.mutable {
		current(contexts).Panic("rustindexmap: mutable access through shared Slice witness")
	}
	return s.GetIndex(index)
}
func (s Slice[K, V]) First() (*K, *V, bool) { return s.GetIndex(0) }
func (s Slice[K, V]) Last() (*K, *V, bool) {
	if len(s.entries) == 0 {
		return nil, nil, false
	}
	return s.GetIndex(uint(len(s.entries) - 1))
}
func (s Slice[K, V]) FirstMut(contexts ...Context) (*K, *V, bool) {
	return s.GetIndexMut(0, contexts...)
}
func (s Slice[K, V]) LastMut(contexts ...Context) (*K, *V, bool) {
	if !s.mutable {
		current(contexts).Panic("rustindexmap: mutable access through shared Slice witness")
	}
	return s.Last()
}
