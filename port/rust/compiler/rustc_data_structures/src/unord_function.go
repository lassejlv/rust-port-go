// SPDX-License-Identifier: MIT OR Apache-2.0
// Owned Fn/FnMut/FnOnce dictionaries required by the remaining UnordItems port.
// This is supporting trait machinery, not additional source-file credit.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordFunctionOps preserves the three independently implemented call traits.
// Shared borrows permit mutation only through source interior mutability;
// mutable borrows are exclusive. Both the environment and argument are actual
// owned cells. Bodies may take/move their owned arguments; remaining parameter
// destruction and prepared-return leaks are handled by the call boundary.
// Callback bodies include complete destruction of their own source locals.
// Static dictionaries have no hidden destructible Go captures or runtime scope.
type UnordFunctionOps[S, A, B any] struct {
	Call     func(rustvec.Ref[S], *rustvec.Owned[A]) *rustvec.Owned[B]
	CallMut  func(rustvec.Ref[S], *rustvec.Owned[A]) *rustvec.Owned[B]
	CallOnce func(*rustvec.Owned[S], *rustvec.Owned[A], rustfmt.Context) *rustvec.Owned[B]
	// Clone is the actual optional Clone implementation for the whole Fn
	// object; the returned owner includes its complete environment Drop.
	Clone func(rustvec.Ref[S]) *rustvec.Owned[S]
}

type unordFunctionOwner[A, B any] interface {
	shared(*rustvec.Owned[A], rustfmt.Context) *rustvec.Owned[B]
	mutable(*rustvec.Owned[A], rustfmt.Context) *rustvec.Owned[B]
	once(*rustvec.Owned[A], rustfmt.Context) *rustvec.Owned[B]
	move(rustfmt.Context) unordFunctionOwner[A, B]
	clone(rustfmt.Context) unordFunctionOwner[A, B]
	close(rustfmt.Context)
	live() bool
}

type unordFunctionCell[S, A, B any] struct {
	state *rustvec.Owned[S]
	ops   UnordFunctionOps[S, A, B]
	valid bool
}

// UnordFunction owns the source function object. It is not a plain Go func:
// captures, mutation, ownership transfer and field Drop remain observable.
type UnordFunction[A, B any] struct{ owner unordFunctionOwner[A, B] }

func NewUnordFunction[S, A, B any](state *rustvec.Owned[S], ops UnordFunctionOps[S, A, B]) UnordFunction[A, B] {
	if state == nil || !state.IsLive() || ops.Call == nil || ops.CallMut == nil || ops.CallOnce == nil {
		panic("datastructures: incomplete actual Fn trait dictionary")
	}
	// This is a persistent source value, not an active operation guard.
	return UnordFunction[A, B]{owner: &unordFunctionCell[S, A, B]{state: state.MoveWithContext(rustfmt.Context{}), ops: ops, valid: true}}
}
func (f *unordFunctionCell[S, A, B]) live() bool { return f.valid && f.state.IsLive() }
func (f *unordFunctionCell[S, A, B]) check(c rustfmt.Context) {
	if !f.live() {
		c.Panic("datastructures: use of moved/dropped Fn object")
	}
}
func unordFunctionResult[B any](value *rustvec.Owned[B], c rustfmt.Context) *rustvec.Owned[B] {
	if value == nil || !value.IsLive() {
		c.Panic("datastructures: Fn returned an invalid owned result")
	}
	return value
}
func (f *unordFunctionCell[S, A, B]) shared(argument *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[B] {
	f.check(c)
	result := rustvec.ConsumeValue(argument, c, func(argument *rustvec.Owned[A]) *rustvec.Owned[B] {
		return f.ops.Call(rustvec.Ref[S]{Value: f.state.Borrow(), Context: c}, argument)
	})
	return unordFunctionResult(result, c)
}
func (f *unordFunctionCell[S, A, B]) mutable(argument *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[B] {
	f.check(c)
	result := rustvec.ConsumeValue(argument, c, func(argument *rustvec.Owned[A]) *rustvec.Owned[B] {
		return f.ops.CallMut(rustvec.Ref[S]{Value: f.state.Borrow(), Context: c}, argument)
	})
	return unordFunctionResult(result, c)
}
func (f *unordFunctionCell[S, A, B]) once(argument *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[B] {
	f.check(c)
	f.valid = false
	state := f.state.MoveWithContext(c)
	// The Args tuple parameter is destroyed before the self parameter. A
	// return place is already moved out when either parameter Drop panics.
	result := rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *rustvec.Owned[B] {
		return rustvec.ConsumeValue(argument, c, func(argument *rustvec.Owned[A]) *rustvec.Owned[B] {
			return f.ops.CallOnce(state, argument, c)
		})
	})
	return unordFunctionResult(result, c)
}
func (f *unordFunctionCell[S, A, B]) move(c rustfmt.Context) unordFunctionOwner[A, B] {
	f.check(c)
	f.valid = false
	return &unordFunctionCell[S, A, B]{state: f.state.MoveWithContext(rustfmt.Context{}), ops: f.ops, valid: true}
}
func (f *unordFunctionCell[S, A, B]) clone(c rustfmt.Context) unordFunctionOwner[A, B] {
	f.check(c)
	if f.ops.Clone == nil {
		c.Panic("datastructures: source Fn type does not supply Clone")
	}
	state := f.ops.Clone(rustvec.Ref[S]{Value: f.state.Borrow(), Context: c})
	if state == nil || !state.IsLive() {
		c.Panic("datastructures: Fn Clone returned an invalid environment")
	}
	return &unordFunctionCell[S, A, B]{state: state.MoveWithContext(rustfmt.Context{}), ops: f.ops, valid: true}
}
func (f *unordFunctionCell[S, A, B]) close(c rustfmt.Context) {
	if !f.live() {
		return
	}
	f.valid = false
	f.state.DropWithContext(c)
}
func (f UnordFunction[A, B]) IsLive() bool { return f.owner != nil && f.owner.live() }
func (f UnordFunction[A, B]) check(c rustfmt.Context) {
	if !f.IsLive() {
		c.Panic("datastructures: missing or consumed Fn object")
	}
}
func (f UnordFunction[A, B]) Call(argument *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[B] {
	f.check(c)
	defer unordHashBoundary(c)
	return f.owner.shared(argument, c)
}
func (f UnordFunction[A, B]) CallMut(argument *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[B] {
	f.check(c)
	defer unordHashBoundary(c)
	return f.owner.mutable(argument, c)
}
func (f UnordFunction[A, B]) CallOnce(argument *rustvec.Owned[A], c rustfmt.Context) *rustvec.Owned[B] {
	f.check(c)
	defer unordHashBoundary(c)
	return f.owner.once(argument, c)
}
func (f UnordFunction[A, B]) Move(c rustfmt.Context) UnordFunction[A, B] {
	f.check(c)
	return UnordFunction[A, B]{owner: f.owner.move(c)}
}
func (f UnordFunction[A, B]) Clone(c rustfmt.Context) UnordFunction[A, B] {
	f.check(c)
	defer unordHashBoundary(c)
	return UnordFunction[A, B]{owner: f.owner.clone(c)}
}
func (f UnordFunction[A, B]) Close(c rustfmt.Context) {
	defer unordHashBoundary(c)
	if f.owner != nil {
		f.owner.close(c)
	}
}
