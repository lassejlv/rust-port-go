// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

// RawIter temporarily borrows its table. No exclusive operation, relocation or
// destruction may occur while it or one of its yielded borrows is retained.
// Clone copies only the iterator position, never the elements.
type RawIter[T any] struct {
	table     *RawTable[T]
	next      uint
	remaining uint
}

func (t *RawTable[T]) Iter(c Context) *RawIter[T] {
	t.check(c)
	return &RawIter[T]{table: t, remaining: t.items}
}
func (i *RawIter[T]) Clone() *RawIter[T] { value := *i; return &value }
func (i *RawIter[T]) Len() uint          { return i.remaining }
func (i *RawIter[T]) Next(c Context) (*T, bool) {
	i.table.check(c)
	if i.remaining == 0 {
		return nil, false
	}
	for i.next < uint(len(i.table.values)) {
		index := i.next
		i.next++
		if i.table.ctrl[index]&0x80 == 0 {
			i.remaining--
			return &i.table.values[index], true
		}
	}
	c.Panic("rusthashmap: invalidated iterator")
	return nil, false
}

// RawIntoIter owns the complete allocation. Its Close destroys unconsumed
// elements in bucket order. It cannot be cloned by Rust's IntoIter API.
type RawIntoIter[T any] struct {
	table     RawTable[T]
	next      uint
	remaining uint
	live      bool
}

func (t *RawTable[T]) IntoIter(c Context) *RawIntoIter[T] {
	t.check(c)
	i := &RawIntoIter[T]{table: *t, remaining: t.items, live: true}
	t.live = false
	t.ctrl, t.values = nil, nil
	t.items, t.growthLeft = 0, 0
	return i
}
func (i *RawIntoIter[T]) check(c Context) {
	if i == nil || !i.live {
		c.Panic("rusthashmap: use of consumed IntoIter")
	}
}
func (i *RawIntoIter[T]) Len() uint { i.check(Context{}); return i.remaining }
func (i *RawIntoIter[T]) Next(c Context) (T, bool) {
	i.check(c)
	var zero T
	if i.remaining == 0 {
		return zero, false
	}
	for i.next < uint(len(i.table.values)) {
		index := i.next
		i.next++
		if i.table.ctrl[index]&0x80 == 0 {
			i.remaining--
			value := i.table.values[index]
			i.table.values[index] = zero
			return value, true
		}
	}
	c.Panic("rusthashmap: invalid owned iterator state")
	return zero, false
}
func (i *RawIntoIter[T]) Close(c Context) {
	if i == nil || !i.live {
		return
	}
	i.live = false
	if i.table.ops.Drop != nil {
		for n := i.next; n < uint(len(i.table.values)); n++ {
			if i.table.ctrl[n]&0x80 == 0 {
				i.table.ops.Drop(&i.table.values[n], c)
			}
		}
	}
	i.table.ctrl, i.table.values = nil, nil
	i.remaining = 0
}
