// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual owned iterator cells for the pending general UnordItems algebra.
// Primitive methods here are functional; remaining algebra/sorting is pending.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordIteratorOps is the selected concrete iterator implementation, not an
// arbitrary Next-only replacement. Count owns its actual self parameter; its
// body may take/move it and otherwise parameter cleanup runs at the boundary.
// For a default count implementation, supply its selected fold operation;
// a distinct count/fold override must never silently become a Next loop.
// Next mutably borrows the actual S cell; SizeHint/Clone share-borrow it.
// CloneItem witnesses derive(Clone)'s additional T:Clone bound without calling
// T::clone. Source dictionaries include complete automatic field destruction.
type UnordIteratorOps[S, T any] struct {
	Next      func(rustvec.Ref[S]) *rustvec.Owned[T]
	SizeHint  func(rustvec.Ref[S]) rustvec.Hint
	Count     func(*rustvec.Owned[S], rustfmt.Context) uint
	Clone     func(rustvec.Ref[S]) *rustvec.Owned[S]
	CloneItem bool
	// Boolean selects the concrete all/any/try_fold implementations. It is
	// separate from Next: an iterator may override each of these methods.
	Boolean *UnordBooleanOps[S, T]
}

type unordIteratorOwner[T any] interface {
	next(rustfmt.Context) *rustvec.Owned[T]
	hint(rustfmt.Context) rustvec.Hint
	count(rustfmt.Context) uint
	boolean(UnordFunction[T, bool], bool, rustfmt.Context) bool
	clone(rustfmt.Context) unordIteratorOwner[T]
	move(rustfmt.Context) unordIteratorOwner[T]
	close(rustfmt.Context)
	live() bool
}
type unordIteratorCell[S, T any] struct {
	state *rustvec.Owned[S]
	ops   UnordIteratorOps[S, T]
	valid bool
}

// UnordItems has no public Next or IntoIterator operation. It owns one actual
// iterator state cell. Call-local runtime metadata is never stored in it.
type UnordItems[T any] struct {
	owner       unordIteratorOwner[T]
	clonePublic bool
}

func NewUnordItems[S, T any](state *rustvec.Owned[S], ops UnordIteratorOps[S, T]) *UnordItems[T] {
	if state == nil || !state.IsLive() || ops.Next == nil || ops.SizeHint == nil || ops.Count == nil {
		panic("datastructures: incomplete selected iterator dictionary")
	}
	return &UnordItems[T]{owner: &unordIteratorCell[S, T]{state: state.MoveWithContext(rustfmt.Context{}), ops: ops, valid: true}, clonePublic: ops.Clone != nil && ops.CloneItem}
}
func (iterator *unordIteratorCell[S, T]) live() bool {
	return iterator.valid && iterator.state.IsLive()
}
func (iterator *unordIteratorCell[S, T]) check(c rustfmt.Context) {
	if !iterator.live() {
		c.Panic("datastructures: use of moved/dropped iterator")
	}
}
func (iterator *unordIteratorCell[S, T]) next(c rustfmt.Context) *rustvec.Owned[T] {
	iterator.check(c)
	value := iterator.ops.Next(rustvec.Ref[S]{Value: iterator.state.Borrow(), Context: c})
	if value != nil && !value.IsLive() {
		c.Panic("datastructures: Iterator::next returned a consumed value")
	}
	return value
}
func (iterator *unordIteratorCell[S, T]) hint(c rustfmt.Context) rustvec.Hint {
	iterator.check(c)
	return iterator.ops.SizeHint(rustvec.Ref[S]{Value: iterator.state.Borrow(), Context: c})
}
func (iterator *unordIteratorCell[S, T]) count(c rustfmt.Context) uint {
	iterator.check(c)
	iterator.valid = false
	state := iterator.state.MoveWithContext(c)
	return rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) uint { return iterator.ops.Count(state, c) })
}
func (iterator *unordIteratorCell[S, T]) clone(c rustfmt.Context) unordIteratorOwner[T] {
	iterator.check(c)
	if iterator.ops.Clone == nil {
		c.Panic("datastructures: source iterator type lacks Clone")
	}
	state := iterator.ops.Clone(rustvec.Ref[S]{Value: iterator.state.Borrow(), Context: c})
	if state == nil || !state.IsLive() {
		c.Panic("datastructures: iterator Clone returned an invalid state")
	}
	return &unordIteratorCell[S, T]{state: state.MoveWithContext(rustfmt.Context{}), ops: iterator.ops, valid: true}
}
func (iterator *unordIteratorCell[S, T]) move(c rustfmt.Context) unordIteratorOwner[T] {
	iterator.check(c)
	iterator.valid = false
	return &unordIteratorCell[S, T]{state: iterator.state.MoveWithContext(rustfmt.Context{}), ops: iterator.ops, valid: true}
}
func (iterator *unordIteratorCell[S, T]) close(c rustfmt.Context) {
	if iterator.live() {
		iterator.valid = false
		iterator.state.DropWithContext(c)
	}
}
func (items *UnordItems[T]) check(c rustfmt.Context) {
	if items == nil || items.owner == nil || !items.owner.live() {
		c.Panic("datastructures: use of consumed UnordItems")
	}
}
func (items *UnordItems[T]) Close(contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	if items != nil && items.owner != nil {
		items.owner.close(c)
	}
}
func (items *UnordItems[T]) Clone(contexts ...rustfmt.Context) *UnordItems[T] {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	items.check(c)
	if !items.clonePublic {
		c.Panic("datastructures: source UnordItems bounds do not expose Clone")
	}
	return &UnordItems[T]{owner: items.owner.clone(c), clonePublic: true}
}

// CloneFrom is derive(Clone)'s inherited default, which clones the complete
// new value before replacing and dropping the old iterator.
func (items *UnordItems[T]) CloneFrom(source *UnordItems[T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	items.check(c)
	source.check(c)
	if items == source {
		c.Panic("datastructures: CloneFrom requires disjoint source/destination borrows")
	}
	unordRequireCloneFromSelf(items, source, c)
	cloned := source.Clone(c)
	old := items.owner
	defer func() {
		items.owner, items.clonePublic = cloned.owner, cloned.clonePublic
		cloned.owner = nil
	}()
	if old != nil {
		old.close(c)
	}
}
func (items *UnordItems[T]) Count(contexts ...rustfmt.Context) uint {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	items.check(c)
	return items.owner.count(c)
}

func unordItemsParameterCleanup[T any](iterator unordIteratorOwner[T], c rustfmt.Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		bagItemsUnwind(unwind, func() { iterator.close(unwind) })
		c.ResumeUnwind(p)
	}
	iterator.close(c)
}
func (items *UnordItems[T]) GetOnly(contexts ...rustfmt.Context) (*rustvec.Owned[T], bool) {
	c := unordContext(contexts)
	items.check(c)
	iterator := items.owner.move(c)
	defer unordItemsParameterCleanup(iterator, c)
	first := iterator.next(c)
	keep := false
	defer bagItemsFirstCleanup(first, &keep, c)
	second := iterator.next(c)
	if second != nil {
		second.DropWithContext(c)
		return nil, false
	}
	keep = true
	return first, first != nil
}

// EmptyUnordItems is the concrete std::iter::Empty<T> constructor. The trait
// witness controls only the derived T:Clone bound; no T value is constructed.
func EmptyUnordItems[T any](itemClone bool) *UnordItems[T] {
	return NewUnordItems(rustvec.Own(struct{}{}), UnordIteratorOps[struct{}, T]{
		Next:      func(rustvec.Ref[struct{}]) *rustvec.Owned[T] { return nil },
		SizeHint:  func(rustvec.Ref[struct{}]) rustvec.Hint { return rustvec.Hint{HasUpper: true} },
		Count:     func(*rustvec.Owned[struct{}], rustfmt.Context) uint { return 0 },
		Clone:     func(rustvec.Ref[struct{}]) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) },
		CloneItem: itemClone,
		Boolean:   &UnordBooleanOps[struct{}, T]{DefaultTryFold: true},
	})
}
