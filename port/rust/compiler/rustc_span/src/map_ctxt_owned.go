// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting ownership adapter for Span::map_ctxt's owned FnOnce parameter.
// Grants no independent source-file completion credit.
package rustcspan

import (
	"sync/atomic"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type ctxtMapperCell[E any] struct {
	environment E
	consumed    atomic.Bool
}

// CtxtMapper is the borrowed-Self convenience transferred to MapCtxtOwned. Copies of
// this descriptor share the same ownership token and cannot be consumed twice.
// Call and Drop receive the actual same mutated cell. A Drop witness represents
// the complete source environment destruction, including any fields moved out
// during Call. Go callers must transfer exclusive ownership of that environment.
// The *E arguments are temporary borrows for that invocation; witnesses must
// not retain those pointers or access the capture after its destruction.
// The witness functions supply code/borrowed state; every nontrivial owned
// capture must be represented in E rather than hidden in a Go function closure.
//
// Context models ambient Rust panic state explicitly. Witnesses should raise
// source panics through Context.Panic, particularly while a panic is being
// processed; no attempt is made to discover global Go panic state.
type CtxtMapper[E any] struct {
	cell    *ctxtMapperCell[E]
	call    func(*E, SyntaxContext, rustfmt.Context) SyntaxContext
	drop    func(*E, rustfmt.Context)
	context rustfmt.Context
}

func NewCtxtMapper[E any](environment E, call func(*E, SyntaxContext, rustfmt.Context) SyntaxContext, drop func(*E, rustfmt.Context), context rustfmt.Context) CtxtMapper[E] {
	if call == nil {
		context.Panic("map_ctxt: nil FnOnce witness")
	}
	return CtxtMapper[E]{cell: &ctxtMapperCell[E]{environment: environment}, call: call, drop: drop, context: context}
}

func ctxtMapperDrop[E any](mapper CtxtMapper[E], context rustfmt.Context) (failure any) {
	defer func() { failure = recover() }()
	if mapper.drop != nil {
		mapper.drop(&mapper.cell.environment, context)
	}
	return nil
}

// MapCtxtOwned is the borrowed-Self convenience. MapCtxtConsuming is the complete
// source FnOnce entry point, including movement of whole Self. Decoding failures
// destroy an uncalled environment during unwind. A successful invocation
// destroys it before any target-span reconstruction, and later failures cannot
// destroy it a second time. Plain Span.MapCtxt is the trivial-drop convenience.
func MapCtxtOwned[E any](span Span, session *SessionGlobals, mapper CtxtMapper[E], explicit ...SpanEncodingConfig) (result Span) {
	context := mapper.context
	context.Runtime = nil
	return MapCtxtOwnedWithContext(span, session, mapper, context, explicit...)
}

// MapCtxtOwnedWithContext invokes the borrowed-Self convenience with the
// current caller context, replacing construction-time ambient metadata.
func MapCtxtOwnedWithContext[E any](span Span, session *SessionGlobals, mapper CtxtMapper[E], context rustfmt.Context, explicit ...SpanEncodingConfig) (result Span) {
	mapper.context = context
	if mapper.cell == nil || mapper.cell.consumed.Swap(true) {
		mapper.context.Panic("map_ctxt: FnOnce environment is absent or already consumed")
	}
	pending := true
	defer func() {
		failure := recover()
		if failure != nil && mapper.context.ProcessingPanic {
			mapper.context.ResumeUnwind(failure)
		}
		if pending {
			pending = false
			context := mapper.context
			if failure != nil {
				context.Unwinding = true
			}
			if dropFailure := ctxtMapperDrop(mapper, context); dropFailure != nil {
				if failure != nil {
					mapper.context.AbortCleanup()
				}
				mapper.context.ResumeUnwind(dropFailure)
			}
		}
		if failure != nil {
			mapper.context.ResumeUnwind(failure)
		}
	}()
	profile := spanEncodingConfig(session, explicit...)
	return span.mapCtxtWithContext(session, func(context SyntaxContext) SyntaxContext {
		mapped := mapper.call(&mapper.cell.environment, context, mapper.context)
		pending = false
		if mapper.drop != nil {
			mapper.drop(&mapper.cell.environment, mapper.context)
		}
		return mapped
	}, profile, mapper.context)
}
