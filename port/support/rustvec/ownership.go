// SPDX-License-Identifier: MIT OR Apache-2.0
// Go witnesses for the ownership/borrowing used by the scoped collector's input/iterator states,
// associated Hash and FromStableHash result. No Rust ABI/layout is implied.
package rustvec

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
)

type DropContext = rustfmt.Context
type Ref[T any] struct {
	Value   *T
	Context DropContext
}
type ValueOps[T any] struct{ Drop func(*T, DropContext) }
type ownedCell[T any] struct {
	value      T
	ops        ValueOps[T]
	live       bool
	generation uint64
}
type Owned[T any] struct {
	cell       *ownedCell[T]
	generation uint64
	context    DropContext
}

func Own[T any](value T) *Owned[T] { return NewOwned(value, ValueOps[T]{}) }
func NewOwned[T any](value T, ops ValueOps[T]) *Owned[T] {
	return &Owned[T]{cell: &ownedCell[T]{value: value, ops: ops, live: true}}
}
func (o *Owned[T]) IsLive() bool {
	return o != nil && o.cell != nil && o.cell.live && o.cell.generation == o.generation
}
func (o *Owned[T]) check() {
	if !o.IsLive() {
		o.Context().Panic("rustvec: use of consumed owned witness")
	}
}
func (o *Owned[T]) Borrow() *T { o.check(); return &o.cell.value }
func (o *Owned[T]) Context() DropContext {
	if o == nil {
		return DropContext{}
	}
	return o.context
}
func (o *Owned[T]) Take() T         { o.check(); o.cell.live = false; return o.cell.value }
func (o *Owned[T]) Move() *Owned[T] { return o.MoveWithContext(o.Context()) }
func (o *Owned[T]) MoveWithContext(context DropContext) *Owned[T] {
	o.check()
	o.cell.generation++
	return &Owned[T]{cell: o.cell, generation: o.cell.generation, context: context}
}
func (o *Owned[T]) Drop() { o.DropWithContext(o.Context()) }
func (o *Owned[T]) DropWithContext(context DropContext) {
	if !o.IsLive() {
		return
	}
	o.cell.live = false
	if o.cell.ops.Drop != nil {
		runContext(context, func() { o.cell.ops.Drop(&o.cell.value, context) })
	}
}
func runContext(context DropContext, body func()) {
	defer func() {
		if p := recover(); p != nil {
			context.Panic(p)
		}
	}()
	body()
}
func cleanupDuringUnwind(body func()) {
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	body()
}
func cleanupOwned[T any](arg *Owned[T], context DropContext) {
	p := recover()
	if p != nil && context.ProcessingPanic {
		os.Exit(134)
	}
	if arg.IsLive() {
		if p != nil {
			unwind := context
			unwind.Unwinding = true
			cleanupDuringUnwind(func() { arg.DropWithContext(unwind) })
		} else {
			arg.DropWithContext(context)
		}
	}
	if p != nil {
		panic(p)
	}
}
func callOwned[A, B any](arg *Owned[A], context DropContext, call func(*Owned[A]) *Owned[B]) *Owned[B] {
	defer cleanupOwned(arg, context)
	result := call(arg)
	if !result.IsLive() {
		context.Panic("rustvec: callback returned a consumed or nil owned result")
	}
	return result
}
