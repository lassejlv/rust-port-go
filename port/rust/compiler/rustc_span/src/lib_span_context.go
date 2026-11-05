// SPDX-License-Identifier: MIT OR Apache-2.0
// Primary explicit ambient-context forms of the original Span getters/setters.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (value Span) DataWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) SpanData {
	return value.dataWithContext(session, spanEncodingConfig(session, config...), ctx)
}
func (value Span) DataUntrackedWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) SpanData {
	return value.dataUntrackedWithContext(session, spanEncodingConfig(session, config...), ctx)
}
func (value Span) LoWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) BytePos {
	return value.dataWithContext(session, spanEncodingConfig(session, config...), ctx).Lo
}
func (value Span) HiWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) BytePos {
	return value.dataWithContext(session, spanEncodingConfig(session, config...), ctx).Hi
}
func (value Span) LoHiWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) (BytePos, BytePos) {
	data := value.dataWithContext(session, spanEncodingConfig(session, config...), ctx)
	return data.Lo, data.Hi
}
func (value Span) WithLoWithContext(session *SessionGlobals, lo BytePos, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	return newSpanWithContext(session, lo, data.Hi, data.Ctxt, data.Parent, p, ctx)
}
func (value Span) WithHiWithContext(session *SessionGlobals, hi BytePos, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	return newSpanWithContext(session, data.Lo, hi, data.Ctxt, data.Parent, p, ctx)
}
func (value Span) WithCtxtWithContext(session *SessionGlobals, ctxt SyntaxContext, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.mapCtxtWithContext(session, func(SyntaxContext) SyntaxContext { return ctxt }, spanEncodingConfig(session, config...), ctx)
}
func WithRootCtxtWithContext(session *SessionGlobals, lo, hi BytePos, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return newSpanWithContext(session, lo, hi, RootSyntaxContext(), None[LocalDefId](), spanEncodingConfig(session, config...), ctx)
}
func (value SpanData) SpanWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return newSpanWithContext(session, value.Lo, value.Hi, value.Ctxt, value.Parent, spanEncodingConfig(session, config...), ctx)
}
func (value SpanData) WithLoWithContext(session *SessionGlobals, lo BytePos, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return newSpanWithContext(session, lo, value.Hi, value.Ctxt, value.Parent, spanEncodingConfig(session, config...), ctx)
}
func (value SpanData) WithHiWithContext(session *SessionGlobals, hi BytePos, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return newSpanWithContext(session, value.Lo, hi, value.Ctxt, value.Parent, spanEncodingConfig(session, config...), ctx)
}
