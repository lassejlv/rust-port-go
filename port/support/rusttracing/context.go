// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// AttachScope borrows the active caller Scope through existing source callback
// contexts. It captures no construction-time scope and installs no TLS.
func AttachScope(ctx rustfmt.Context, scope *Scope) rustfmt.Context {
	if ScopeFromContext(ctx) == scope {
		return ctx
	}
	metadata := rustfmt.RuntimeMetadata{}
	if ctx.Runtime != nil {
		metadata = *ctx.Runtime
	}
	metadata.Value = scope
	ctx.Runtime = &metadata
	return ctx
}
func ScopeFromContext(ctx rustfmt.Context) *Scope {
	if ctx.Runtime == nil {
		return nil
	}
	scope, _ := ctx.Runtime.Value.(*Scope)
	return scope
}
func callContext(scope *Scope, ctx rustfmt.Context) Context {
	if scope == nil {
		scope = ScopeFromContext(ctx)
	}
	return Context{Scope: scope, Panic: AttachScope(ctx, scope)}
}
