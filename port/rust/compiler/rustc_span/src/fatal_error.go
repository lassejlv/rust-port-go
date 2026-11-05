// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_span/src/fatal_error.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// FatalError is the source's non-Send fatal compiler error. Go cannot express
// the negative Send bound; callers must keep it within the originating scope.
type FatalError struct{}
type FatalErrorMarker = datastructures.FatalErrorMarker

func (value FatalError) Clone() FatalError            { return value }
func (value *FatalError) CloneFrom(source FatalError) { *value = source.Clone() }
func (FatalError) Error() string                      { return "fatal error" }
func (FatalError) Source() error                      { return nil }
func (FatalError) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	return f.WriteString("fatal error")
}
func (FatalError) WriteDebugFormatter(f *rustfmt.Formatter) error { return f.WriteString("FatalError") }
func (value FatalError) RustDebug() string                        { return rustfmt.String(value.WriteDebugFormatter) }

// Raise resumes unwinding with the exact marker, bypassing panic-hook dispatch.
// It is not an ordinary panic-message operation. The contextual primary form
// also preserves immediate abort when called while processing another panic.
func (value FatalError) Raise()                             { value.RaiseWithContext(rustfmt.Context{}) }
func (FatalError) RaiseWithContext(context rustfmt.Context) { context.ResumeUnwind(FatalErrorMarker{}) }

// FatalFnOnce is a consumed FnOnce producing an owned generic value. Close
// describes destruction of its capture environment, including unused captures.
// Primary translated panic sites use the supplied context.Panic/Raise; ordinary
// Go panic/defer callbacks retain Go local cleanup timing. A consumed non-Copy
// callback must not be reused. Destructible captures live in the stated owned
// environment rather than hidden additional callback closures.
type FatalFnOnce[T any] struct {
	Call  func(rustfmt.Context) *SpanOwned[T]
	Close func(rustfmt.Context)
}

func invokeFatalFnOnce[T any](context rustfmt.Context, callback FatalFnOnce[T]) (result *SpanOwned[T]) {
	defer spanCloseOwnedCallback(context, callback.Close)
	result = callback.Call(context)
	if !result.IsLive() {
		context.Panic("fatal_error: callback returned an invalid owned value")
	}
	result.context = spanOwnedGuardContext(context)
	return
}

// CatchFatalErrorsOwnedWithContext preserves catch_unwind's fatal-only mapping.
// Unrelated panic payloads are resumed unchanged, without another hook call.
// If capture destruction panics after preparing a return value, that prepared
// value is leaked as in Rust's FnOnce return-place cleanup.
func CatchFatalErrorsOwnedWithContext[T any](context rustfmt.Context, callback FatalFnOnce[T]) (result *SpanOwned[T], failure Option[FatalError]) {
	defer func() {
		if payload := recover(); payload != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			if _, ok := payload.(FatalErrorMarker); ok {
				result = nil
				failure = Some(FatalError{})
				return
			}
			context.ResumeUnwind(payload)
		}
	}()
	result = invokeFatalFnOnce(context, callback)
	return
}
func CatchFatalErrorsOwned[T any](callback FatalFnOnce[T]) (*SpanOwned[T], Option[FatalError]) {
	return CatchFatalErrorsOwnedWithContext(rustfmt.Context{}, callback)
}

// CatchFatalErrors is the ordinary-context convenience for a trivial-drop
// closure environment and a plain-value result. Use the owned form when either
// environment or result has nontrivial source destruction.
func CatchFatalErrors[T any](callback func() T) (T, Option[FatalError]) {
	value, failure := CatchFatalErrorsOwned(FatalFnOnce[T]{Call: func(rustfmt.Context) *SpanOwned[T] { return NewSpanOwned(callback(), nil) }})
	if failure.IsSome() {
		var zero T
		return zero, failure
	}
	return value.Take(), failure
}
