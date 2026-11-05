// SPDX-License-Identifier: MIT OR Apache-2.0
// Used ownership contracts of generic source_map.rs/fatal_error.rs operations.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// SpanOwned represents a consumed generic source value, including its complete
// destructor. The destructor witness includes owned fields and local cleanup.
// Borrows must follow Rust's alias/lifetime rules and cannot outlive a move/drop.
type SpanOwned[T any] struct {
	cell       *spanOwnedCell[T]
	generation uint64
	context    rustfmt.Context
}
type spanOwnedCell[T any] struct {
	value      T
	drop       func(*T, rustfmt.Context)
	live       bool
	generation uint64
}

func NewSpanOwned[T any](value T, drop func(*T, rustfmt.Context)) *SpanOwned[T] {
	return &SpanOwned[T]{cell: &spanOwnedCell[T]{value: value, drop: drop, live: true}}
}
func (value *SpanOwned[T]) IsLive() bool {
	return value != nil && value.cell != nil && value.cell.live && value.generation == value.cell.generation
}
func (value *SpanOwned[T]) check() {
	if !value.IsLive() {
		value.Context().Panic("rustc_span: use of consumed owned value")
	}
}
func (value *SpanOwned[T]) Context() rustfmt.Context {
	if value == nil {
		return rustfmt.Context{}
	}
	return value.context
}
func (value *SpanOwned[T]) Borrow() *T          { value.check(); return &value.cell.value }
func (value *SpanOwned[T]) Take() T             { value.check(); value.cell.live = false; return value.cell.value }
func (value *SpanOwned[T]) Move() *SpanOwned[T] { return value.MoveWithContext(rustfmt.Context{}) }
func (value *SpanOwned[T]) MoveWithContext(context rustfmt.Context) *SpanOwned[T] {
	value.check()
	value.cell.generation++
	return &SpanOwned[T]{cell: value.cell, generation: value.cell.generation, context: spanOwnedGuardContext(context)}
}
func (value *SpanOwned[T]) Drop() { value.DropWithContext(rustfmt.Context{}) }
func spanOwnedGuardContext(context rustfmt.Context) rustfmt.Context {
	// Runtime borrows the caller Scope for one operation. An owning value must
	// never retain it for a later operation/destructor; contextual entry points
	// receive that later caller's witness explicitly.
	context.Runtime = nil
	return context
}
func (value *SpanOwned[T]) DropWithContext(context rustfmt.Context) {
	if !value.IsLive() {
		return
	}
	value.cell.live = false
	if value.cell.drop != nil {
		spanRunContext(context, func() { value.cell.drop(&value.cell.value, context) })
	}
}
func spanRunContext(context rustfmt.Context, body func()) {
	defer func() {
		if p := recover(); p != nil {
			context.ResumeUnwind(p)
		}
	}()
	body()
}
func spanCleanupDuringUnwind(body func()) {
	spanCleanupDuringUnwindWithContext(rustfmt.Context{}, body)
}

// The current operation owns this compiler cleanup-abort boundary. Callers
// supply its actual runtime diagnostic metadata; this helper never captures a
// construction-time scope or dispatches the original panic hook again.
func spanCleanupDuringUnwindWithContext(context rustfmt.Context, body func()) {
	defer func() {
		if recover() != nil {
			context.AbortCleanup()
		}
	}()
	body()
}

// spanCloseOwnedCallback destroys an owned callback environment. Ambient unwind
// alone remains catchable; a new panic escaping locally required cleanup aborts.
func spanCloseOwnedCallback(context rustfmt.Context, close func(rustfmt.Context)) {
	p := recover()
	spanCloseOwnedCallbackPanic(context, close, p)
}
func spanCloseOwnedCallbackPanic(context rustfmt.Context, close func(rustfmt.Context), p any) {
	if p != nil && context.ProcessingPanic {
		context.ResumeUnwind(p)
	}
	if close != nil {
		if p != nil {
			unwind := context
			unwind.Unwinding = true
			spanCleanupDuringUnwindWithContext(unwind, func() { close(unwind) })
		} else {
			spanRunContext(context, func() { close(context) })
		}
	}
	if p != nil {
		context.ResumeUnwind(p)
	}
}

func sourceMapCloseSharedOnExit[T any](value Shared[T], context rustfmt.Context) {
	payload := recover()
	if payload != nil && context.ProcessingPanic {
		context.ResumeUnwind(payload)
	}
	if payload != nil {
		unwind := context
		unwind.Unwinding = true
		spanCleanupDuringUnwindWithContext(unwind, func() { value.CloseWithContext(unwind) })
	} else {
		value.CloseWithContext(context)
	}
	if payload != nil {
		context.ResumeUnwind(payload)
	}
}

func sourceMapCloseSharedUnlessMoved[T any](value Shared[T], moved *bool, context rustfmt.Context) {
	payload := recover()
	if !*moved {
		spanCloseOwnedCallbackPanic(context, value.CloseWithContext, payload)
	} else if payload != nil {
		context.ResumeUnwind(payload)
	}
}
