// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Complete borrowed dictionaries and opaque context cells enter the current
// invocation here. Plain dictionaries remain useful for callbacks with no
// runtime metadata. No helper owns an additional source closure environment.
func hygieneDecoderContext[R any](d serialize.Decoder, c rustfmt.Context, call func(serialize.Decoder) R) R {
	if view, ok := d.(interface {
		BorrowSpanDecoderWithContext(rustfmt.Context) SpanDecoder
	}); ok {
		return call(view.BorrowSpanDecoderWithContext(c))
	}
	if cell, ok := d.(serialize.RuntimeContextCell); ok {
		return serialize.WithRuntimeContext(cell, c, func(serialize.RuntimeContextCell) R { return call(d) })
	}
	return call(d)
}
func hygieneSpanDecoderContext[R any](d SpanDecoder, c rustfmt.Context, call func(SpanDecoder) R) R {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) R { return call(d.(SpanDecoder)) })
}
func hygieneEncoderContext(e serialize.Encoder, c rustfmt.Context, call func(serialize.Encoder)) {
	if view, ok := e.(interface {
		BorrowSpanEncoderWithContext(rustfmt.Context) SpanEncoder
	}); ok {
		call(view.BorrowSpanEncoderWithContext(c))
		return
	}
	if cell, ok := e.(serialize.RuntimeContextCell); ok {
		serialize.WithRuntimeContext(cell, c, func(serialize.RuntimeContextCell) struct{} { call(e); return struct{}{} })
		return
	}
	call(e)
}
func hygieneSpanEncoderContext(e SpanEncoder, c rustfmt.Context, call func(SpanEncoder)) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { call(e.(SpanEncoder)) })
}

func DecodeTransparencyWithContext(d serialize.Decoder, c rustfmt.Context) Transparency {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) Transparency { return decodeTransparencyWithContext(d, c) })
}
func DecodeMacroKindWithContext(d serialize.Decoder, c rustfmt.Context) MacroKind {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) MacroKind { return decodeMacroKindWithContext(d, c) })
}
func DecodeAstPassWithContext(d serialize.Decoder, c rustfmt.Context) AstPass {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) AstPass { return decodeAstPassWithContext(d, c) })
}
func DecodeDesugaringKindWithContext(d serialize.Decoder, c rustfmt.Context) DesugaringKind {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) DesugaringKind { return decodeDesugaringKindWithContext(d, c) })
}
func DecodeExpnKindWithContext(d SpanDecoder, c rustfmt.Context) ExpnKind {
	return hygieneSpanDecoderContext(d, c, func(d SpanDecoder) ExpnKind { return decodeExpnKindWithContext(d, c) })
}
func DecodeExpnHashWithContext(d serialize.Decoder, c rustfmt.Context) ExpnHash {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) ExpnHash { return decodeExpnHashWithContext(d, c) })
}
func DecodeDecodedSyntaxContextWithContext(d serialize.Decoder, c rustfmt.Context) DecodedSyntaxContext {
	return hygieneDecoderContext(d, c, func(d serialize.Decoder) DecodedSyntaxContext { return decodeDecodedSyntaxContextWithContext(d, c) })
}
func DecodeExpnDataWithContext(d SpanDecoder, c rustfmt.Context) ExpnData {
	return hygieneSpanDecoderContext(d, c, func(d SpanDecoder) ExpnData { return decodeExpnDataWithContext(d, c) })
}
func DecodeLocalExpnIdWithContext(d SpanDecoder, c rustfmt.Context) LocalExpnId {
	return hygieneSpanDecoderContext(d, c, func(d SpanDecoder) LocalExpnId {
		return DecodeExpnId(d).expectLocalWithHygieneSite(c, HygienePanicSite(1641, 9))
	})
}

func (v Transparency) EncodeWithContext(e serialize.Encoder, c rustfmt.Context) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { v.Encode(e) })
}
func (v MacroKind) EncodeWithContext(e serialize.Encoder, c rustfmt.Context) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { v.Encode(e) })
}
func (v AstPass) EncodeWithContext(e serialize.Encoder, c rustfmt.Context) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { v.Encode(e) })
}
func (v DesugaringKind) EncodeWithContext(e serialize.Encoder, c rustfmt.Context) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { v.Encode(e) })
}
func (v ExpnHash) EncodeWithContext(e serialize.Encoder, c rustfmt.Context) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { v.Encode(e) })
}
func (v DecodedSyntaxContext) EncodeWithContext(e serialize.Encoder, c rustfmt.Context) {
	hygieneEncoderContext(e, c, func(e serialize.Encoder) { v.Encode(e) })
}
func (v ExpnKind) EncodeWithContext(e SpanEncoder, c rustfmt.Context) {
	hygieneSpanEncoderContext(e, c, func(e SpanEncoder) { v.Encode(e) })
}
func (v *ExpnData) EncodeWithContext(e SpanEncoder, c rustfmt.Context) {
	hygieneSpanEncoderContext(e, c, func(e SpanEncoder) { v.Encode(e) })
}
func (v LocalExpnId) EncodeWithContext(e SpanEncoder, c rustfmt.Context) {
	hygieneSpanEncoderContext(e, c, func(e SpanEncoder) { v.Encode(e) })
}
