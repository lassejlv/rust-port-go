// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting borrowed iterator paths from indexmap2.14.0 src/map/iter.rs and
// iterator_methods!/double_ended_iterator_methods!. Generic collection/fold
// contracts remain pending; no whole-file credit.
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type borrowedCursor[K, V any] struct {
	view        Slice[K, V]
	front, back uint
	live        bool
}

func newBorrowedCursor[K, V any](view Slice[K, V]) *borrowedCursor[K, V] {
	return &borrowedCursor[K, V]{view: view, back: view.Len(), live: true}
}
func (c *borrowedCursor[K, V]) check(context Context) {
	if c == nil || !c.live {
		context.Panic("rustindexmap: use of consumed borrowed iterator")
	}
}
func (c *borrowedCursor[K, V]) len(context Context) uint { c.check(context); return c.back - c.front }
func (c *borrowedCursor[K, V]) slice(context Context) Slice[K, V] {
	c.check(context)
	return Slice[K, V]{entries: c.view.entries[c.front:c.back], config: c.view.config}
}
func (c *borrowedCursor[K, V]) clone(context Context) *borrowedCursor[K, V] {
	c.check(context)
	next := *c
	return &next
}
func (c *borrowedCursor[K, V]) next(n uint, back bool, context Context) (*K, *V, bool) {
	c.check(context)
	if c.view.config.Profile.PointerBits == 32 && uint64(n) > 0xffffffff {
		context.Panic("rustindexmap: iterator argument exceeds source usize domain")
	}
	if n >= c.back-c.front {
		c.front = c.back
		return nil, nil, false
	}
	var index uint
	if back {
		c.back -= n + 1
		index = c.back
	} else {
		c.front += n
		index = c.front
		c.front++
	}
	return c.view.GetIndex(index)
}
func (c *borrowedCursor[K, V]) close() {
	if c != nil {
		c.live = false
	}
}
func (c *borrowedCursor[K, V]) count(context Context) uint { n := c.len(context); c.close(); return n }
func (c *borrowedCursor[K, V]) last(context Context) (*K, *V, bool) {
	k, v, ok := c.next(0, true, context)
	c.close()
	return k, v, ok
}
func (c *borrowedCursor[K, V]) hint(context Context) rustvec.Hint {
	n := c.len(context)
	return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
}
func (c *borrowedCursor[K, V]) debug(f *rustfmt.Formatter, mode uint) error {
	c.check(f.Context)
	config := c.view.config
	if mode != 2 && config.Keys.Debug == nil || mode != 1 && config.Values.Debug == nil {
		f.Context.Panic("rustindexmap: missing iterator Debug witness")
	}
	pretty := f.Options.Alternate
	list := rustfmt.BeginList(f.Writer, pretty)
	for i := c.front; i < c.back; i++ {
		entry := &c.view.entries[i]
		list.Entry(f.Value(pretty, func(child *rustfmt.Formatter) error {
			switch mode {
			case 1:
				return config.Keys.Debug(&entry.key, child)
			case 2:
				return config.Values.Debug(&entry.value, child)
			}
			return rustfmt.WriteTuple(child.Writer, "", pretty, []rustfmt.Value{child.Value(pretty, func(field *rustfmt.Formatter) error { return config.Keys.Debug(&entry.key, field) }), child.Value(pretty, func(field *rustfmt.Formatter) error { return config.Values.Debug(&entry.value, field) })})
		}))
	}
	return list.Finish()
}

// Iter's items are shared borrows. IterMut/ValuesMut's value items are disjoint
// exclusive borrows; their key pointers remain shared. Borrow rules prohibit
// retaining an incompatible map/slice borrow, mutating shared pointers or using
// a cursor after consuming it. Descriptor copies are not implicit Clone.
type Iter[K, V any] struct{ cursor *borrowedCursor[K, V] }
type IterMut[K, V any] struct{ cursor *borrowedCursor[K, V] }
type Keys[K, V any] struct{ cursor *borrowedCursor[K, V] }
type Values[K, V any] struct{ cursor *borrowedCursor[K, V] }
type ValuesMut[K, V any] struct{ cursor *borrowedCursor[K, V] }

func DefaultIter[K, V any](config Config[K, V]) *Iter[K, V] {
	return &Iter[K, V]{newBorrowedCursor(Slice[K, V]{config: config})}
}
func DefaultIterMut[K, V any](config Config[K, V]) *IterMut[K, V] {
	return &IterMut[K, V]{newBorrowedCursor(Slice[K, V]{config: config, mutable: true})}
}
func DefaultKeys[K, V any](config Config[K, V]) *Keys[K, V] {
	return &Keys[K, V]{newBorrowedCursor(Slice[K, V]{config: config})}
}
func DefaultValues[K, V any](config Config[K, V]) *Values[K, V] {
	return &Values[K, V]{newBorrowedCursor(Slice[K, V]{config: config})}
}
func DefaultValuesMut[K, V any](config Config[K, V]) *ValuesMut[K, V] {
	return &ValuesMut[K, V]{newBorrowedCursor(Slice[K, V]{config: config, mutable: true})}
}

func (s Slice[K, V]) Iter() *Iter[K, V] { return &Iter[K, V]{newBorrowedCursor(s)} }
func (s Slice[K, V]) IterMut(contexts ...Context) *IterMut[K, V] {
	if !s.mutable {
		current(contexts).Panic("rustindexmap: mutable iterator from shared Slice")
	}
	return &IterMut[K, V]{newBorrowedCursor(s)}
}
func (s Slice[K, V]) Keys() *Keys[K, V]     { return &Keys[K, V]{newBorrowedCursor(s)} }
func (s Slice[K, V]) Values() *Values[K, V] { return &Values[K, V]{newBorrowedCursor(s)} }
func (s Slice[K, V]) ValuesMut(contexts ...Context) *ValuesMut[K, V] {
	if !s.mutable {
		current(contexts).Panic("rustindexmap: mutable iterator from shared Slice")
	}
	return &ValuesMut[K, V]{newBorrowedCursor(s)}
}
func (m *Map[K, V]) Iter(contexts ...Context) *Iter[K, V] { return m.AsSlice(contexts...).Iter() }
func (m *Map[K, V]) IterMut(contexts ...Context) *IterMut[K, V] {
	return m.AsMutSlice(contexts...).IterMut(contexts...)
}
func (m *Map[K, V]) Keys(contexts ...Context) *Keys[K, V]     { return m.AsSlice(contexts...).Keys() }
func (m *Map[K, V]) Values(contexts ...Context) *Values[K, V] { return m.AsSlice(contexts...).Values() }
func (m *Map[K, V]) ValuesMut(contexts ...Context) *ValuesMut[K, V] {
	return m.AsMutSlice(contexts...).ValuesMut(contexts...)
}

func (it *Iter[K, V]) Clone(contexts ...Context) *Iter[K, V] {
	return &Iter[K, V]{it.cursor.clone(current(contexts))}
}
func (it *Iter[K, V]) CloneFrom(source *Iter[K, V], contexts ...Context) {
	next := source.Clone(contexts...)
	it.Close()
	it.cursor = next.cursor
}
func (it *Keys[K, V]) Clone(contexts ...Context) *Keys[K, V] {
	return &Keys[K, V]{it.cursor.clone(current(contexts))}
}
func (it *Keys[K, V]) CloneFrom(source *Keys[K, V], contexts ...Context) {
	next := source.Clone(contexts...)
	it.Close()
	it.cursor = next.cursor
}
func (it *Values[K, V]) Clone(contexts ...Context) *Values[K, V] {
	return &Values[K, V]{it.cursor.clone(current(contexts))}
}
func (it *Values[K, V]) CloneFrom(source *Values[K, V], contexts ...Context) {
	next := source.Clone(contexts...)
	it.Close()
	it.cursor = next.cursor
}
func (it *Iter[K, V]) AsSlice(contexts ...Context) Slice[K, V] {
	return it.cursor.slice(current(contexts))
}
func (it *IterMut[K, V]) AsSlice(contexts ...Context) Slice[K, V] {
	return it.cursor.slice(current(contexts))
}
func (it *IterMut[K, V]) IntoSlice(contexts ...Context) Slice[K, V] {
	view := it.cursor.slice(current(contexts))
	view.mutable = true
	it.Close()
	return view
}

func (it *Iter[K, V]) Len(contexts ...Context) uint { return it.cursor.len(current(contexts)) }
func (it *Iter[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.cursor.hint(current(contexts))
}
func (it *Iter[K, V]) Close()                                         { it.cursor.close() }
func (it *Iter[K, V]) Count(contexts ...Context) uint                 { return it.cursor.count(current(contexts)) }
func (it *Iter[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error { return it.cursor.debug(f, 0) }
func (it *Iter[K, V]) Next(contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(0, false, current(contexts))
}
func (it *Iter[K, V]) NextBack(contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(0, true, current(contexts))
}
func (it *Iter[K, V]) Nth(n uint, contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(n, false, current(contexts))
}
func (it *Iter[K, V]) NthBack(n uint, contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(n, true, current(contexts))
}
func (it *Iter[K, V]) Last(contexts ...Context) (*K, *V, bool) {
	return it.cursor.last(current(contexts))
}

func (it *IterMut[K, V]) Len(contexts ...Context) uint { return it.cursor.len(current(contexts)) }
func (it *IterMut[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.cursor.hint(current(contexts))
}
func (it *IterMut[K, V]) Close()                         { it.cursor.close() }
func (it *IterMut[K, V]) Count(contexts ...Context) uint { return it.cursor.count(current(contexts)) }
func (it *IterMut[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return it.cursor.debug(f, 0)
}
func (it *IterMut[K, V]) Next(contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(0, false, current(contexts))
}
func (it *IterMut[K, V]) NextBack(contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(0, true, current(contexts))
}
func (it *IterMut[K, V]) Nth(n uint, contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(n, false, current(contexts))
}
func (it *IterMut[K, V]) NthBack(n uint, contexts ...Context) (*K, *V, bool) {
	return it.cursor.next(n, true, current(contexts))
}
func (it *IterMut[K, V]) Last(contexts ...Context) (*K, *V, bool) {
	return it.cursor.last(current(contexts))
}

func (it *Keys[K, V]) Len(contexts ...Context) uint { return it.cursor.len(current(contexts)) }
func (it *Keys[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.cursor.hint(current(contexts))
}
func (it *Keys[K, V]) Close()                                         { it.cursor.close() }
func (it *Keys[K, V]) Count(contexts ...Context) uint                 { return it.cursor.count(current(contexts)) }
func (it *Keys[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error { return it.cursor.debug(f, 1) }
func (it *Keys[K, V]) Next(contexts ...Context) (*K, bool) {
	k, _, ok := it.cursor.next(0, false, current(contexts))
	return k, ok
}
func (it *Keys[K, V]) NextBack(contexts ...Context) (*K, bool) {
	k, _, ok := it.cursor.next(0, true, current(contexts))
	return k, ok
}
func (it *Keys[K, V]) Nth(n uint, contexts ...Context) (*K, bool) {
	k, _, ok := it.cursor.next(n, false, current(contexts))
	return k, ok
}
func (it *Keys[K, V]) NthBack(n uint, contexts ...Context) (*K, bool) {
	k, _, ok := it.cursor.next(n, true, current(contexts))
	return k, ok
}
func (it *Keys[K, V]) Last(contexts ...Context) (*K, bool) {
	k, _, ok := it.cursor.last(current(contexts))
	return k, ok
}

func (it *Values[K, V]) Len(contexts ...Context) uint { return it.cursor.len(current(contexts)) }
func (it *Values[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.cursor.hint(current(contexts))
}
func (it *Values[K, V]) Close()                                         { it.cursor.close() }
func (it *Values[K, V]) Count(contexts ...Context) uint                 { return it.cursor.count(current(contexts)) }
func (it *Values[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error { return it.cursor.debug(f, 2) }
func (it *Values[K, V]) Next(contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(0, false, current(contexts))
	return v, ok
}
func (it *Values[K, V]) NextBack(contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(0, true, current(contexts))
	return v, ok
}
func (it *Values[K, V]) Nth(n uint, contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(n, false, current(contexts))
	return v, ok
}
func (it *Values[K, V]) NthBack(n uint, contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(n, true, current(contexts))
	return v, ok
}
func (it *Values[K, V]) Last(contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.last(current(contexts))
	return v, ok
}

func (it *ValuesMut[K, V]) Len(contexts ...Context) uint { return it.cursor.len(current(contexts)) }
func (it *ValuesMut[K, V]) SizeHint(contexts ...Context) rustvec.Hint {
	return it.cursor.hint(current(contexts))
}
func (it *ValuesMut[K, V]) Close()                         { it.cursor.close() }
func (it *ValuesMut[K, V]) Count(contexts ...Context) uint { return it.cursor.count(current(contexts)) }
func (it *ValuesMut[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return it.cursor.debug(f, 2)
}
func (it *ValuesMut[K, V]) Next(contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(0, false, current(contexts))
	return v, ok
}
func (it *ValuesMut[K, V]) NextBack(contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(0, true, current(contexts))
	return v, ok
}
func (it *ValuesMut[K, V]) Nth(n uint, contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(n, false, current(contexts))
	return v, ok
}
func (it *ValuesMut[K, V]) NthBack(n uint, contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.next(n, true, current(contexts))
	return v, ok
}
func (it *ValuesMut[K, V]) Last(contexts ...Context) (*V, bool) {
	_, v, ok := it.cursor.last(current(contexts))
	return v, ok
}
