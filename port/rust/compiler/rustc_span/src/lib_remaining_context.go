// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func (value Span) WithDefSiteCtxtWithContext(session *SessionGlobals, id ExpnId, context rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.withCtxtFromMark(session, id, Opaque, context, config...)
}
func (value Span) WithCallSiteCtxtWithContext(session *SessionGlobals, id ExpnId, context rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.withCtxtFromMark(session, id, Transparent, context, config...)
}
func (value Span) WithMixedSiteCtxtWithContext(session *SessionGlobals, id ExpnId, context rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.withCtxtFromMark(session, id, SemiOpaque, context, config...)
}
func (value Span) IsRust2015WithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.EditionWithContext(session, context).IsRust2015()
}
func (value Span) AtLeastRust2018WithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.EditionWithContext(session, context).AtLeastRust2018()
}
func (value Span) AtLeastRust2021WithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.EditionWithContext(session, context).AtLeastRust2021()
}
func (value Span) AtLeastRust2024WithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.EditionWithContext(session, context).AtLeastRust2024()
}
func (value Span) SubstituteDummyWithContext(session *SessionGlobals, other Span, context rustfmt.Context) Span {
	if value.IsDummyWithContext(session, context) {
		return other
	}
	return value
}
func (value *SourceFile) CountLinesWithContext(context rustfmt.Context, config ...SpanEncodingConfig) uint {
	return uint(len(value.LinesWithContext(context, config...)))
}

// WithMetavarSpansOwned consumes the actual FnOnce environment. The static call
// dictionary owns Self (and may Take/Move it), borrowing its real cell and this
// operation's current context. An absent scoped session destroys the still-owned callback; a call
// or destructor failure follows ConsumeValue's source cleanup protocol. A
// prepared result is intentionally not destroyed if callback destruction fails.
func WithMetavarSpansOwned[R, E any](scope *SessionScope, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], *MetavarSpansMap, rustfmt.Context) R, context rustfmt.Context) R {
	return rustvec.ConsumeValue(environment, context, func(callback *rustvec.Owned[E]) R {
		session := scope.CurrentWithContext(context)
		return call(callback, session.metavarSpans, context)
	})
}

// CreateDefaultSessionGlobalsThenOwned retains the complete source FnOnce
// boundary, rather than erasing its owner behind a plain Go function. The
// callback is destroyed before the scoped-slot guard, then the session, even
// when its call or destructor fails. ConsumeValue preserves a prepared return
// value on a destructor panic, matching the source call's return slot.
func CreateDefaultSessionGlobalsThenOwned[R, E any](scope *SessionScope, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], *SessionScope, rustfmt.Context) R, context rustfmt.Context, config ...SpanEncodingConfig) R {
	return CreateSessionGlobalsThenConsuming(scope, DefaultEdition, nil, None[SourceMapInputs](), environment, call, context, config...)
}
