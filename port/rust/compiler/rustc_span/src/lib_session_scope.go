// SPDX-License-Identifier: MIT OR Apache-2.0
// Explicit scopes adapt lib.rs's scoped TLS operations. Each scope represents
// one source thread's TLS slot; it must not be shared concurrently. Session
// values remain shareable according to their actual locking contracts.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

type SessionScope struct{ current *SessionGlobals }

func NewSessionScope() *SessionScope    { return &SessionScope{} }
func (scope *SessionScope) IsSet() bool { return scope != nil && scope.current != nil }
func (scope *SessionScope) CurrentWithContext(context rustfmt.Context) *SessionGlobals {
	if !scope.IsSet() {
		context.PanicAt(rustfmt.SourcePanicInfo{Payload: "cannot access a scoped thread local variable without calling `set` first", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(ScopedSessionPanicSite), CanUnwind: true})
	}
	return scope.current
}
func (scope *SessionScope) Current() *SessionGlobals {
	return scope.CurrentWithContext(rustfmt.Context{})
}

func SetSessionGlobalsThen[R any](scope *SessionScope, session *SessionGlobals, call func() R) R {
	return SetSessionGlobalsThenOwned(scope, session, NewSpanOwned(struct{}{}, nil), func(*struct{}, *SessionScope, rustfmt.Context) R { return call() }, rustfmt.Context{})
}
func SetSessionGlobalsThenOwned[R, E any](scope *SessionScope, session *SessionGlobals, environment *SpanOwned[E], call func(*E, *SessionScope, rustfmt.Context) R, context rustfmt.Context) R {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	if scope == nil {
		context.Panic("rustcspan: absent explicit TLS scope")
	}
	if scope.IsSet() {
		spanStaticPanicAt(context, spanLibLocation(164, 5), "SESSION_GLOBALS should never be overwritten! Use another thread if you need another SessionGlobals")
	}
	if session == nil {
		context.Panic("rustcspan: nil SessionGlobals borrow")
	}
	previous := scope.current
	scope.current = session
	defer func() {
		payload := recover()
		if payload != nil && context.ProcessingPanic {
			context.ResumeUnwind(payload)
		}
		scope.current = previous
		if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	return func() R {
		defer spanCloseOwnedCallback(context, env.DropWithContext)
		return call(env.Borrow(), scope, context)
	}()
}
func WithSessionGlobals[R any](scope *SessionScope, call func(*SessionGlobals) R) R {
	return WithSessionGlobalsOwned(scope, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, session *SessionGlobals, _ rustfmt.Context) R { return call(session) }, rustfmt.Context{})
}
func WithSessionGlobalsOwned[R, E any](scope *SessionScope, environment *SpanOwned[E], call func(*E, *SessionGlobals, rustfmt.Context) R, context rustfmt.Context) R {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	session := scope.CurrentWithContext(context)
	return call(env.Borrow(), session, context)
}
func CreateSessionGlobalsThen[R any](scope *SessionScope, edition Edition, extraSymbols []string, inputs Option[SourceMapInputs], call func() R) R {
	return CreateSessionGlobalsThenOwned(scope, edition, extraSymbols, inputs, NewSpanOwned(struct{}{}, nil), func(*struct{}, *SessionScope, rustfmt.Context) R { return call() }, rustfmt.Context{})
}
func CreateSessionGlobalsThenOwned[R, E any](scope *SessionScope, edition Edition, extraSymbols []string, inputs Option[SourceMapInputs], environment *SpanOwned[E], call func(*E, *SessionScope, rustfmt.Context) R, context rustfmt.Context, config ...SpanEncodingConfig) R {
	env := environment.MoveWithContext(context)
	inputsConsumed := false
	defer spanCloseOwnedCallback(context, func(ctx rustfmt.Context) {
		if !inputsConsumed {
			if sourceInputs, ok := inputs.Get(); ok {
				sourceInputs.FileLoader.CloseWithContext(ctx)
			}
		}
	})
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	if scope == nil {
		context.Panic("rustcspan: absent explicit TLS scope")
	}
	if scope.IsSet() {
		spanStaticPanicAt(context, spanLibLocation(154, 5), "SESSION_GLOBALS should never be overwritten! Use another thread if you need another SessionGlobals")
	}
	inputsConsumed = true
	session := NewSessionGlobalsWithContext(edition, extraSymbols, inputs, context, config...)
	defer spanCloseOwnedCallback(context, session.CloseWithContext)
	return SetSessionGlobalsThenOwned(scope, session, env, call, context)
}
func CreateDefaultSessionGlobalsThen[R any](scope *SessionScope, call func() R) R {
	return CreateSessionGlobalsThen(scope, DefaultEdition, nil, None[SourceMapInputs](), call)
}
func CreateSessionIfNotSetThen[R any](scope *SessionScope, edition Edition, call func(*SessionGlobals) R) R {
	return CreateSessionIfNotSetThenOwned(scope, edition, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, session *SessionGlobals, _ rustfmt.Context) R { return call(session) }, rustfmt.Context{})
}
func CreateSessionIfNotSetThenOwned[R, E any](scope *SessionScope, edition Edition, environment *SpanOwned[E], call func(*E, *SessionGlobals, rustfmt.Context) R, context rustfmt.Context, config ...SpanEncodingConfig) R {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	if !scope.IsSet() {
		session := NewSessionGlobalsWithContext(edition, nil, None[SourceMapInputs](), context, config...)
		defer spanCloseOwnedCallback(context, session.CloseWithContext)
		return SetSessionGlobalsThenOwned(scope, session, env, func(value *E, active *SessionScope, ctx rustfmt.Context) R {
			return call(value, active.CurrentWithContext(ctx), ctx)
		}, context)
	}
	return WithSessionGlobalsOwned(scope, env, call, context)
}
func WithMetavarSpans[R any](scope *SessionScope, call func(*MetavarSpansMap) R) R {
	return WithSessionGlobals(scope, func(session *SessionGlobals) R { return call(session.metavarSpans) })
}
