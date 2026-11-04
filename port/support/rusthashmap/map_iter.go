// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

type BorrowedPair[K, V any] struct {
	Key   *K
	Value *V
}
type Iter[K, V any] struct{ inner *RawIter[Pair[K, V]] }

func (m *Map[K, V]) Iter(c Context) *Iter[K, V] { m.check(c); return &Iter[K, V]{m.table.Iter(c)} }
func (i *Iter[K, V]) Clone() *Iter[K, V]        { return &Iter[K, V]{i.inner.Clone()} }
func (i *Iter[K, V]) Len() uint                 { return i.inner.Len() }
func (i *Iter[K, V]) Next(c Context) (BorrowedPair[K, V], bool) {
	pair, ok := i.inner.Next(c)
	if !ok {
		return BorrowedPair[K, V]{}, false
	}
	return BorrowedPair[K, V]{&pair.Key, &pair.Value}, true
}

type IntoIter[K, V any] struct {
	inner  *RawIntoIter[Pair[K, V]]
	keys   KeyOps[K]
	values ValueOps[V]
}

func (m *Map[K, V]) IntoIter(c Context) *IntoIter[K, V] {
	m.check(c)
	m.live = false
	result := &IntoIter[K, V]{m.table.IntoIter(c), m.keys, m.values}
	// The hasher field is destroyed after preparing the iterator return place;
	// a throwing hasher destructor leaks that prepared iterator and its values.
	m.builder.Close(c)
	return result
}
func (i *IntoIter[K, V]) Len() uint                         { return i.inner.Len() }
func (i *IntoIter[K, V]) Next(c Context) (Pair[K, V], bool) { return i.inner.Next(c) }
func (i *IntoIter[K, V]) Close(c Context)                   { i.inner.Close(c) }
