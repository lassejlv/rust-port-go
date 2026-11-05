// SPDX-License-Identifier: MIT OR Apache-2.0
// SourceMap-dependent Span methods from pinned lib.rs.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (value Span) IsVisible(session *SessionGlobals, sm *SourceMap) bool {
	return value.IsVisibleWithContext(session, sm, rustfmt.Context{})
}
func (value Span) IsVisibleWithContext(session *SessionGlobals, sm *SourceMap, ctx rustfmt.Context) bool {
	return !value.IsDummyWithContext(session, ctx) && sm.IsSpanAccessibleWithContext(session, value, ctx)
}
func (value Span) InExternalMacro(session *SessionGlobals, sm *SourceMap) bool {
	return value.InExternalMacroWithContext(session, sm, rustfmt.Context{})
}
func (value Span) InExternalMacroWithContext(session *SessionGlobals, sm *SourceMap, ctx rustfmt.Context) bool {
	return value.Ctxt().InExternalMacroWithContext(session, sm, ctx)
}
func (value Span) FindAncestorNotFromExternMacro(session *SessionGlobals, sm *SourceMap) Option[Span] {
	return value.FindAncestorNotFromExternMacroWithContext(session, sm, rustfmt.Context{})
}
func (value Span) FindAncestorNotFromExternMacroWithContext(session *SessionGlobals, sm *SourceMap, ctx rustfmt.Context) Option[Span] {
	for value.InExternalMacroWithContext(session, sm, ctx) {
		parent, ok := value.ParentCallsiteWithContext(session, ctx).Get()
		if !ok {
			return None[Span]()
		}
		value = parent
	}
	return Some(value)
}
func (value Span) WithinMacro(session *SessionGlobals, within Span, sm *SourceMap, config ...SpanEncodingConfig) Option[Span] {
	return value.WithinMacroWithContext(session, within, sm, rustfmt.Context{}, config...)
}
func (value Span) WithinMacroWithContext(session *SessionGlobals, within Span, sm *SourceMap, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[Span] {
	p := spanEncodingConfig(session, config...)
	this, _, parent, _, ok := spanPrepareToCombine(session, value, within, ctx, p)
	if !ok {
		return None[Span]()
	}
	if value.dataWithContext(session, p, ctx).Contains(this) || sm.IsImportedWithContext(session, within, ctx) {
		return None[Span]()
	}
	if spanUseExpnData(session, within.Ctxt(), ctx, func(data *ExpnData) bool { return data.DiagnosticOpaque }) {
		return None[Span]()
	}
	return Some(newOrderedSpanWithContext(session, this.Lo, this.Hi, this.Ctxt, parent, p, ctx))
}
