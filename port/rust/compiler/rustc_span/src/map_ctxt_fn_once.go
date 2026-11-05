// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// MapCtxtConsuming is Span::map_ctxt's complete current-context FnOnce entry.
// Call receives the actual owned Self, allowing Take/Move/forget, including
// before a panic. Remaining Self is destroyed at call completion; a decoding
// failure destroys the unused Self. All owned captures and field cleanup belong
// in environment. CtxtMapper/MapCtxtOwned are borrowed-Self conveniences.
func MapCtxtConsuming[E any](span Span, session *SessionGlobals, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], SyntaxContext, rustfmt.Context) SyntaxContext, context rustfmt.Context, config ...SpanEncodingConfig) Span {
	owned := environment.MoveWithContext(context)
	envelope := NewCtxtMapper(owned, func(slot **rustvec.Owned[E], old SyntaxContext, current rustfmt.Context) SyntaxContext {
		return rustvec.ConsumeValue(*slot, current, func(argument *rustvec.Owned[E]) SyntaxContext {
			return call(argument, old, current)
		})
	}, func(slot **rustvec.Owned[E], current rustfmt.Context) {
		(*slot).DropWithContext(current)
	}, context)
	return MapCtxtOwnedWithContext(span, session, envelope, context, config...)
}
