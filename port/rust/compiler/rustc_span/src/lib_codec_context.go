// SPDX-License-Identifier: MIT OR Apache-2.0
// Current-context opaque Span codec operations. Constructors retain guard
// flags only; explicit invocation metadata is installed for this borrow.
package rustcspan

import (
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (e *SpanMemEncoder) EncodeSpanWithContext(value Span, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeSpan(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeSymbolWithContext(value Symbol, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeSymbol(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeByteSymbolWithContext(value ByteSymbol, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeByteSymbol(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeExpnIdWithContext(value ExpnId, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeExpnId(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeSyntaxContextWithContext(value SyntaxContext, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeSyntaxContext(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeCrateNumWithContext(value CrateNum, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeCrateNum(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeDefIndexWithContext(value DefIndex, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeDefIndex(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeDefIdWithContext(value DefId, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeDefId(value); return struct{}{} })
}
func (e *SpanMemEncoder) EncodeLocalDefIdWithContext(value LocalDefId, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanMemEncoder) struct{} { e.EncodeLocalDefId(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeSpanWithContext(value Span, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeSpan(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeSymbolWithContext(value Symbol, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeSymbol(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeByteSymbolWithContext(value ByteSymbol, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeByteSymbol(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeExpnIdWithContext(value ExpnId, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeExpnId(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeSyntaxContextWithContext(value SyntaxContext, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeSyntaxContext(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeCrateNumWithContext(value CrateNum, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeCrateNum(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeDefIndexWithContext(value DefIndex, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeDefIndex(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeDefIdWithContext(value DefId, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeDefId(value); return struct{}{} })
}
func (e *SpanFileEncoder) EncodeLocalDefIdWithContext(value LocalDefId, context rustfmt.Context) {
	rustcserialize.WithRuntimeContext(e, context, func(e *SpanFileEncoder) struct{} { e.EncodeLocalDefId(value); return struct{}{} })
}
func (d *SpanMemDecoder) DecodeSpanWithContext(context rustfmt.Context) Span {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) Span { return d.DecodeSpan() })
}
func (d *SpanMemDecoder) DecodeSymbolWithContext(context rustfmt.Context) Symbol {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) Symbol { return d.DecodeSymbol() })
}
func (d *SpanMemDecoder) DecodeByteSymbolWithContext(context rustfmt.Context) ByteSymbol {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) ByteSymbol { return d.DecodeByteSymbol() })
}
func (d *SpanMemDecoder) DecodeExpnIdWithContext(context rustfmt.Context) ExpnId {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) ExpnId { return d.DecodeExpnId() })
}
func (d *SpanMemDecoder) DecodeSyntaxContextWithContext(context rustfmt.Context) SyntaxContext {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) SyntaxContext { return d.DecodeSyntaxContext() })
}
func (d *SpanMemDecoder) DecodeCrateNumWithContext(context rustfmt.Context) CrateNum {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) CrateNum { return d.DecodeCrateNum() })
}
func (d *SpanMemDecoder) DecodeDefIndexWithContext(context rustfmt.Context) DefIndex {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) DefIndex { return d.DecodeDefIndex() })
}
func (d *SpanMemDecoder) DecodeDefIdWithContext(context rustfmt.Context) DefId {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) DefId { return d.DecodeDefId() })
}
func (d *SpanMemDecoder) DecodeLocalDefIdWithContext(context rustfmt.Context) LocalDefId {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) LocalDefId { return d.DecodeLocalDefId() })
}
func (d *SpanMemDecoder) DecodeAttrIdWithContext(context rustfmt.Context) AttrId {
	return rustcserialize.WithRuntimeContext(d, context, func(d *SpanMemDecoder) AttrId { return d.DecodeAttrId() })
}
