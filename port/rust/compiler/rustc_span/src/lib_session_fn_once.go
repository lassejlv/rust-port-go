// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// sessionFnOnceEnvelope is a transparent owner of the actual source FnOnce
// value. It adapts the established scoped-session guards without erasing the
// callback behind a trivial-drop function. Moving this descriptor never copies
// the callback's value cell. Unused callbacks remain owned and are destroyed;
// an invoked implementation can Take/Move/forget its whole Self.
func sessionFnOnceEnvelope[E any](environment *rustvec.Owned[E], context rustfmt.Context) *SpanOwned[*rustvec.Owned[E]] {
	value := environment.MoveWithContext(context)
	return NewSpanOwned(value, func(value **rustvec.Owned[E], current rustfmt.Context) {
		(*value).DropWithContext(current)
	})
}

// The Consuming primaries model the full FnOnce::call_once(Self, ...) operation.
// Method code is borrowed static metadata; every owned capture belongs in E.
// The older *ThenOwned/WithSessionGlobalsOwned ref-only entry points remain
// conveniences for the compiler-generated forwarding call strategy.
func SetSessionGlobalsThenConsuming[R, E any](scope *SessionScope, session *SessionGlobals, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], *SessionScope, rustfmt.Context) R, context rustfmt.Context) R {
	return SetSessionGlobalsThenOwned(scope, session, sessionFnOnceEnvelope(environment, context), func(value **rustvec.Owned[E], active *SessionScope, current rustfmt.Context) R {
		return rustvec.ConsumeValue(*value, current, func(callback *rustvec.Owned[E]) R {
			return call(callback, active, current)
		})
	}, context)
}
func WithSessionGlobalsConsuming[R, E any](scope *SessionScope, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], *SessionGlobals, rustfmt.Context) R, context rustfmt.Context) R {
	return WithSessionGlobalsOwned(scope, sessionFnOnceEnvelope(environment, context), func(value **rustvec.Owned[E], session *SessionGlobals, current rustfmt.Context) R {
		return rustvec.ConsumeValue(*value, current, func(callback *rustvec.Owned[E]) R {
			return call(callback, session, current)
		})
	}, context)
}
func CreateSessionGlobalsThenConsuming[R, E any](scope *SessionScope, edition Edition, extraSymbols []string, inputs Option[SourceMapInputs], environment *rustvec.Owned[E], call func(*rustvec.Owned[E], *SessionScope, rustfmt.Context) R, context rustfmt.Context, config ...SpanEncodingConfig) R {
	return CreateSessionGlobalsThenOwned(scope, edition, extraSymbols, inputs, sessionFnOnceEnvelope(environment, context), func(value **rustvec.Owned[E], active *SessionScope, current rustfmt.Context) R {
		return rustvec.ConsumeValue(*value, current, func(callback *rustvec.Owned[E]) R {
			return call(callback, active, current)
		})
	}, context, config...)
}
func CreateSessionIfNotSetThenConsuming[R, E any](scope *SessionScope, edition Edition, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], *SessionGlobals, rustfmt.Context) R, context rustfmt.Context, config ...SpanEncodingConfig) R {
	return CreateSessionIfNotSetThenOwned(scope, edition, sessionFnOnceEnvelope(environment, context), func(value **rustvec.Owned[E], session *SessionGlobals, current rustfmt.Context) R {
		return rustvec.ConsumeValue(*value, current, func(callback *rustvec.Owned[E]) R {
			return call(callback, session, current)
		})
	}, context, config...)
}
