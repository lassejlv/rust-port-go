// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordBooleanOps is the selected monomorphized Iterator implementation for
// all/any and try_fold<(), ControlFlow<()>>. A true consumer result continues;
// false breaks. TryFold owns its actual check(f) closure parameter; an override
// may explicitly invoke, destroy or forget that value. It is not a borrowed
// Go callback hiding an owning predicate capture.
// All and Any independently override the defaults. DefaultTryFold is an
// explicit witness that the concrete iterator uses std's Next-based default;
// it must not be set for an iterator with a different try_fold implementation.
// The function argument belongs to the selected all/any parameter and may be
// moved or forgotten by that implementation; otherwise it is destroyed when
// that method returns, before UnordItems destroys its iterator parameter.
type UnordBooleanOps[S, T any] struct {
	All            func(rustvec.Ref[S], UnordFunction[T, bool]) bool
	Any            func(rustvec.Ref[S], UnordFunction[T, bool]) bool
	TryFold        func(rustvec.Ref[S], UnordTryFoldFunction[T]) bool
	DefaultTryFold bool
}

// UnordTryFoldFunction is the owned FnMut/FnOnce check(f) closure used by the
// source default all/any implementations. Its bool result represents
// ControlFlow<()> (true=Continue); no shared Fn operation is exposed because
// the original check closure mutably borrows its captured predicate.
type UnordTryFoldFunction[T any] struct{ function UnordFunction[T, bool] }

func (f UnordTryFoldFunction[T]) CallMut(value *rustvec.Owned[T], c rustfmt.Context) bool {
	return f.function.CallMut(value, c).Take()
}
func (f UnordTryFoldFunction[T]) CallOnce(value *rustvec.Owned[T], c rustfmt.Context) bool {
	return f.function.CallOnce(value, c).Take()
}
func (f UnordTryFoldFunction[T]) Move(c rustfmt.Context) UnordTryFoldFunction[T] {
	return UnordTryFoldFunction[T]{function: f.function.Move(c)}
}
func (f UnordTryFoldFunction[T]) Close(c rustfmt.Context) { f.function.Close(c) }
func (f UnordTryFoldFunction[T]) Forget(c rustfmt.Context) {
	// Discarding the moved owner models mem::forget: no source destructor runs.
	_ = f.Move(c)
}

type unordBooleanCapture[T any] struct {
	predicate UnordFunction[T, bool]
	any       bool
}

func unordBooleanCheck[T any](predicate UnordFunction[T, bool], any bool) UnordTryFoldFunction[T] {
	state := rustvec.NewOwned(unordBooleanCapture[T]{predicate: predicate.Move(rustfmt.Context{}), any: any}, rustvec.ValueOps[unordBooleanCapture[T]]{
		Drop: func(state *unordBooleanCapture[T], c rustfmt.Context) { state.predicate.Close(c) },
	})
	invoke := func(state *unordBooleanCapture[T], value *rustvec.Owned[T], c rustfmt.Context) *rustvec.Owned[bool] {
		result := state.predicate.CallMut(value, c).Take()
		if state.any {
			result = !result
		}
		return rustvec.Own(result)
	}
	function := NewUnordFunction(state, UnordFunctionOps[unordBooleanCapture[T], T, bool]{
		Call: func(ref rustvec.Ref[unordBooleanCapture[T]], _ *rustvec.Owned[T]) *rustvec.Owned[bool] {
			ref.Context.Panic("datastructures: check(f) implements FnMut, not Fn")
			return nil
		},
		CallMut: func(ref rustvec.Ref[unordBooleanCapture[T]], value *rustvec.Owned[T]) *rustvec.Owned[bool] {
			return invoke(ref.Value, value, ref.Context)
		},
		CallOnce: func(state *rustvec.Owned[unordBooleanCapture[T]], value *rustvec.Owned[T], c rustfmt.Context) *rustvec.Owned[bool] {
			return invoke(state.Borrow(), value, c)
		},
	})
	return UnordTryFoldFunction[T]{function: function}
}

func unordFunctionParameterCleanup[A, B any](function UnordFunction[A, B], c rustfmt.Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		bagItemsUnwind(unwind, func() { function.Close(unwind) })
		c.ResumeUnwind(p)
	}
	function.Close(c)
}

func (iterator *unordIteratorCell[S, T]) boolean(predicate UnordFunction[T, bool], any bool, c rustfmt.Context) bool {
	iterator.check(c)
	predicate = predicate.Move(c)
	defer unordFunctionParameterCleanup(predicate, c)
	ops := iterator.ops.Boolean
	if ops == nil {
		c.Panic("datastructures: missing selected Iterator::all/any dictionary")
	}
	ref := rustvec.Ref[S]{Value: iterator.state.Borrow(), Context: c}
	if any && ops.Any != nil {
		return ops.Any(ref, predicate)
	}
	if !any && ops.All != nil {
		return ops.All(ref, predicate)
	}
	consumer := unordBooleanCheck(predicate, any).Move(c)
	defer unordFunctionParameterCleanup(consumer.function, c)
	var continued bool
	if ops.TryFold != nil {
		continued = ops.TryFold(ref, consumer)
	} else if ops.DefaultTryFold {
		continued = true
		for {
			value := iterator.next(c)
			if value == nil {
				break
			}
			if !consumer.CallMut(value, c) {
				continued = false
				break
			}
		}
	} else {
		c.Panic("datastructures: missing selected Iterator::try_fold dictionary")
	}
	if any {
		return !continued
	}
	return continued
}

// All and Any consume both source parameters. Their predicate is invoked via
// FnMut, as in the underlying Iterator methods, even though UnordItems requires
// the stronger Fn bound. The function's actual shared/mutable/once operations
// are independent and are never substituted for one another.
func (items *UnordItems[T]) All(predicate UnordFunction[T, bool], contexts ...rustfmt.Context) bool {
	return items.boolean(predicate, false, unordContext(contexts))
}
func (items *UnordItems[T]) Any(predicate UnordFunction[T, bool], contexts ...rustfmt.Context) bool {
	return items.boolean(predicate, true, unordContext(contexts))
}
func (items *UnordItems[T]) boolean(predicate UnordFunction[T, bool], any bool, c rustfmt.Context) bool {
	items.check(c)
	iterator := items.owner.move(c)
	defer unordItemsParameterCleanup(iterator, c)
	return iterator.boolean(predicate, any, c)
}
