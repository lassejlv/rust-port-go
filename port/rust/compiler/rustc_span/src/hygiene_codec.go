// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"fmt"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func hygieneEnumTag(decoder serialize.Decoder, name string, variants uint8, ctx rustfmt.Context) uint8 {
	tag := decoder.ReadU8()
	if tag >= variants {
		site := map[string]string{"Transparency": HygienePanicSite(178, 74), "MacroKind": HygienePanicSite(1294, 66), "AstPass": HygienePanicSite(1330, 52), "DesugaringKind": HygienePanicSite(1348, 52), "ExpnKind": HygienePanicSite(1266, 46), "Edition": "source:rustc_span/src/edition.rs:7:70"}[name]
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: fmt.Sprintf("invalid enum variant tag while decoding `%s`, expected 0..%d, actual %d", name, variants, tag), Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation(site)})
	}
	return tag
}
func (v Transparency) Encode(e serialize.Encoder) { e.EmitU8(uint8(v)) }
func DecodeTransparency(d serialize.Decoder) Transparency {
	return DecodeTransparencyWithContext(d, spanCodecContext(d))
}
func decodeTransparencyWithContext(d serialize.Decoder, ctx rustfmt.Context) Transparency {
	return Transparency(hygieneEnumTag(d, "Transparency", 3, ctx))
}
func (v MacroKind) Encode(e serialize.Encoder) { e.EmitU8(uint8(v)) }
func DecodeMacroKind(d serialize.Decoder) MacroKind {
	return DecodeMacroKindWithContext(d, spanCodecContext(d))
}
func decodeMacroKindWithContext(d serialize.Decoder, ctx rustfmt.Context) MacroKind {
	return MacroKind(hygieneEnumTag(d, "MacroKind", 3, ctx))
}
func (v AstPass) Encode(e serialize.Encoder) { e.EmitU8(uint8(v)) }
func DecodeAstPass(d serialize.Decoder) AstPass {
	return DecodeAstPassWithContext(d, spanCodecContext(d))
}
func decodeAstPassWithContext(d serialize.Decoder, ctx rustfmt.Context) AstPass {
	return AstPass(hygieneEnumTag(d, "AstPass", 3, ctx))
}
func (v DesugaringKind) Encode(e serialize.Encoder) {
	e.EmitU8(uint8(v.Tag))
	if v.Tag == DesugarFormatLiteral {
		serialize.EmitBool(e, v.Source)
	}
}
func DecodeDesugaringKind(d serialize.Decoder) DesugaringKind {
	return DecodeDesugaringKindWithContext(d, spanCodecContext(d))
}
func decodeDesugaringKindWithContext(d serialize.Decoder, ctx rustfmt.Context) DesugaringKind {
	tag := DesugaringTag(hygieneEnumTag(d, "DesugaringKind", 13, ctx))
	value := DesugaringKind{Tag: tag}
	if tag == DesugarFormatLiteral {
		value.Source = serialize.ReadBool(d)
	}
	return value
}
func (v ExpnKind) Encode(e SpanEncoder) {
	e.EmitU8(uint8(v.Tag))
	switch v.Tag {
	case ExpnRoot:
	case ExpnMacro:
		v.MacroKind.Encode(e)
		v.Name.Encode(e)
	case ExpnAstPass:
		v.AstPass.Encode(e)
	case ExpnDesugaring:
		v.Desugaring.Encode(e)
	}
}
func DecodeExpnKind(d SpanDecoder) ExpnKind { return DecodeExpnKindWithContext(d, spanCodecContext(d)) }
func decodeExpnKindWithContext(d SpanDecoder, ctx rustfmt.Context) ExpnKind {
	tag := ExpnKindTag(hygieneEnumTag(d, "ExpnKind", 4, ctx))
	value := ExpnKind{Tag: tag}
	switch tag {
	case ExpnRoot:
	case ExpnMacro:
		value.MacroKind = DecodeMacroKindWithContext(d, ctx)
		value.Name = DecodeSymbol(d)
	case ExpnAstPass:
		value.AstPass = DecodeAstPassWithContext(d, ctx)
	case ExpnDesugaring:
		value.Desugaring = DecodeDesugaringKindWithContext(d, ctx)
	}
	return value
}
func (v ExpnHash) Encode(e serialize.Encoder) { v.fingerprint.Encode(e) }

type hygieneFingerprintDecoder struct{ decoder serialize.Decoder }

func (d hygieneFingerprintDecoder) ReadRawBytes(n int) []byte { return d.decoder.ReadRawBytes(uint(n)) }
func DecodeExpnHash(d serialize.Decoder) ExpnHash {
	return DecodeExpnHashWithContext(d, spanCodecContext(d))
}
func decodeExpnHashWithContext(d serialize.Decoder, ctx rustfmt.Context) ExpnHash {
	return ExpnHash{datastructures.DecodeFingerprintWithContext(ctx, hygieneFingerprintDecoder{d})}
}
func (v DecodedSyntaxContext) Encode(e serialize.Encoder) { e.EmitU32(v.AsU32()) }
func DecodeDecodedSyntaxContext(d serialize.Decoder) DecodedSyntaxContext {
	return DecodeDecodedSyntaxContextWithContext(d, spanCodecContext(d))
}
func decodeDecodedSyntaxContextWithContext(d serialize.Decoder, ctx rustfmt.Context) DecodedSyntaxContext {
	return DecodedSyntaxContextFromU32WithContext(d.ReadU32(), ctx)
}
func (v LocalExpnId) Encode(e SpanEncoder) { v.ToExpnId().Encode(e) }
func DecodeLocalExpnId(d SpanDecoder) LocalExpnId {
	return DecodeLocalExpnIdWithContext(d, spanCodecContext(d))
}
func hygieneDecodeOption[T any](d serialize.Decoder, ctx rustfmt.Context, decode func() T) Option[T] {
	switch d.ReadU8() {
	case 0:
		return None[T]()
	case 1:
		return Some(decode())
	default:
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "Encountered invalid discriminant while decoding `Option`.", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation("source:rustc_serialize/src/serialize.rs:459:18")})
	}
	return None[T]()
}
func (v *ExpnData) Encode(e SpanEncoder) {
	v.Kind.Encode(e)
	v.Parent.Encode(e)
	v.CallSite.Encode(e)
	e.EmitU32(v.disambiguator)
	v.DefSite.Encode(e)
	if symbols, ok := v.AllowInternalUnstable.Get(); ok {
		e.EmitU8(1)
		values := symbols.BorrowWithContext(spanCodecContext(e))
		e.EmitUsize(uint(len(*values)))
		for i := range *values {
			(*values)[i].Encode(e)
		}
	} else {
		e.EmitU8(0)
	}
	v.Edition.Encode(e)
	if value, ok := v.MacroDefID.Get(); ok {
		e.EmitU8(1)
		value.Encode(e)
	} else {
		e.EmitU8(0)
	}
	if value, ok := v.ParentModule.Get(); ok {
		e.EmitU8(1)
		value.Encode(e)
	} else {
		e.EmitU8(0)
	}
	serialize.EmitBool(e, v.AllowInternalUnsafe)
	serialize.EmitBool(e, v.LocalInnerMacros)
	serialize.EmitBool(e, v.CollapseDebuginfo)
	serialize.EmitBool(e, v.DiagnosticOpaque)
}
func DecodeExpnData(d SpanDecoder) ExpnData { return DecodeExpnDataWithContext(d, spanCodecContext(d)) }
func decodeExpnDataWithContext(d SpanDecoder, ctx rustfmt.Context) (value ExpnData) {
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if !complete {
				unwind := ctx
				unwind.Unwinding = true
				hygieneCleanup(unwind, func() { value.CloseWithContext(unwind) })
			}
			ctx.ResumeUnwind(p)
		}
	}()
	value.Kind = DecodeExpnKindWithContext(d, ctx)
	value.Parent = DecodeExpnId(d)
	value.CallSite = DecodeSpan(d)
	value.disambiguator = d.ReadU32()
	value.DefSite = DecodeSpan(d)
	value.AllowInternalUnstable = hygieneDecodeOption(d, ctx, func() Shared[[]Symbol] {
		length := d.ReadUsize()
		// The source collects a TrustedLen Map<Range<usize>> into Vec<Symbol>.
		// Its exact reservation rejects an overflowing four-byte layout before
		// invoking even the first custom decode_symbol operation.
		if length > uint(^uint(0)>>1)/4 {
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation("rustc_span::hygiene::codec.capacity_overflow")})
		}
		symbols := make([]Symbol, 0, int(length))
		for i := uint(0); i < length; i++ {
			symbols = append(symbols, DecodeSymbol(d))
		}
		return NewShared(symbols, nil)
	})
	value.Edition = Edition(hygieneEnumTag(d, "Edition", 5, ctx))
	value.MacroDefID = hygieneDecodeOption(d, ctx, func() DefId { return DecodeDefId(d) })
	value.ParentModule = hygieneDecodeOption(d, ctx, func() ModId { return DecodeModId(d) })
	value.AllowInternalUnsafe = serialize.ReadBool(d)
	value.LocalInnerMacros = serialize.ReadBool(d)
	value.CollapseDebuginfo = serialize.ReadBool(d)
	value.DiagnosticOpaque = serialize.ReadBool(d)
	complete = true
	return value
}
