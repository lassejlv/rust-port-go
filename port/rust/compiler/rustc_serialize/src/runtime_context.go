// SPDX-License-Identifier: MIT OR Apache-2.0
// Borrowed per-operation context binding for the scoped opaque codec adapters.
// Codec data owns no ambient tracing scope. The operation thunk is static code
// with borrowed metadata, not an owned Fn capture or an additional source API.
package rustcserialize

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

type RuntimeContextCell interface {
	RuntimeContext() rustfmt.Context
	SetRuntimeContext(rustfmt.Context)
}

func WithRuntimeContext[R any, T RuntimeContextCell](codec T, context rustfmt.Context, operation func(T) R) R {
	previous := codec.RuntimeContext()
	codec.SetRuntimeContext(context)
	defer func() {
		payload := recover()
		if payload != nil && context.ProcessingPanic {
			context.ResumeUnwind(payload)
		}
		codec.SetRuntimeContext(previous)
		if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	return operation(codec)
}
func contextGuardsOnly(context rustfmt.Context) rustfmt.Context {
	context.Runtime = nil
	return context
}
