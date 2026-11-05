// SPDX-License-Identifier: MIT OR Apache-2.0
// SpanEncoder/BlobDecoder/SpanDecoder and their delegated impls from pinned
// rustc_span/src/lib.rs. Explicit sessions replace scoped TLS, not codecs.
package rustcspan

import (
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type SpanEncoder interface {
	rustcserialize.Encoder
	EncodeSpan(Span)
	EncodeSymbol(Symbol)
	EncodeByteSymbol(ByteSymbol)
	EncodeExpnId(ExpnId)
	EncodeSyntaxContext(SyntaxContext)
	EncodeCrateNum(CrateNum)
	EncodeDefIndex(DefIndex)
	EncodeDefId(DefId)
}
type BlobDecoder interface {
	rustcserialize.Decoder
	DecodeSymbol() Symbol
	DecodeByteSymbol() ByteSymbol
	DecodeDefIndex() DefIndex
	DecodeLocalDefId() LocalDefId
}
type SpanDecoder interface {
	BlobDecoder
	DecodeSpan() Span
	DecodeExpnId() ExpnId
	DecodeSyntaxContext() SyntaxContext
	DecodeCrateNum() CrateNum
	DecodeDefId() DefId
	DecodeAttrId() AttrId
}

func SpanEncodeLocalDefId(e SpanEncoder, id LocalDefId) {
	if override, ok := e.(interface{ EncodeLocalDefId(LocalDefId) }); ok {
		override.EncodeLocalDefId(id)
		return
	}
	DefaultSpanEncodeLocalDefId(e, id)
}
func DefaultSpanEncodeLocalDefId(e SpanEncoder, id LocalDefId) { e.EncodeDefId(id.ToDefId()) }
func (value Span) Encode(e SpanEncoder)                        { e.EncodeSpan(value) }
func (value Symbol) Encode(e SpanEncoder)                      { e.EncodeSymbol(value) }
func (value ByteSymbol) Encode(e SpanEncoder)                  { e.EncodeByteSymbol(value) }
func (value ExpnId) Encode(e SpanEncoder)                      { e.EncodeExpnId(value) }
func (value SyntaxContext) Encode(e SpanEncoder)               { e.EncodeSyntaxContext(value) }
func (value CrateNum) Encode(e SpanEncoder)                    { e.EncodeCrateNum(value) }
func (value DefIndex) Encode(e SpanEncoder)                    { e.EncodeDefIndex(value) }
func (value DefId) Encode(e SpanEncoder)                       { e.EncodeDefId(value) }
func (value AttrId) Encode(e SpanEncoder)                      {}
func DecodeSpan(d SpanDecoder) Span                            { return d.DecodeSpan() }
func DecodeSymbol(d BlobDecoder) Symbol                        { return d.DecodeSymbol() }
func DecodeByteSymbol(d BlobDecoder) ByteSymbol                { return d.DecodeByteSymbol() }
func DecodeExpnId(d SpanDecoder) ExpnId                        { return d.DecodeExpnId() }
func DecodeSyntaxContext(d SpanDecoder) SyntaxContext          { return d.DecodeSyntaxContext() }
func DecodeCrateNum(d SpanDecoder) CrateNum                    { return d.DecodeCrateNum() }
func DecodeDefIndex(d BlobDecoder) DefIndex                    { return d.DecodeDefIndex() }
func DecodeDefId(d SpanDecoder) DefId                          { return d.DecodeDefId() }
func DecodeAttrId(d SpanDecoder) AttrId                        { return d.DecodeAttrId() }
func (value BytePos) Encode(e rustcserialize.Encoder)          { e.EmitU32(value.AsU32()) }
func (value RelativeBytePos) Encode(e rustcserialize.Encoder)  { e.EmitU32(value.AsU32()) }
func DecodeBytePos(d rustcserialize.Decoder) BytePos           { return BytePosFromU32(d.ReadU32()) }
func DecodeRelativeBytePos(d rustcserialize.Decoder) RelativeBytePos {
	return RelativeBytePosFromU32(d.ReadU32())
}

type SpanMemEncoder struct {
	*rustcserialize.MemEncoder
	session *SessionGlobals
	config  SpanEncodingConfig
}

func NewSpanMemEncoder(session *SessionGlobals) *SpanMemEncoder {
	return NewSpanMemEncoderWithContext(session, rustfmt.Context{})
}
func NewSpanMemEncoderWithContext(session *SessionGlobals, context rustfmt.Context, config ...SpanEncodingConfig) *SpanMemEncoder {
	profile := spanEncodingConfig(session, config...)
	return &SpanMemEncoder{MemEncoder: rustcserialize.NewMemEncoderWithContext(context, rustcserialize.Profile{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks}), session: session, config: profile}
}
func WrapSpanMemEncoder(session *SessionGlobals, base *rustcserialize.MemEncoder) *SpanMemEncoder {
	profile := base.BuildProfile()
	return &SpanMemEncoder{MemEncoder: base, session: session, config: SpanEncodingConfig{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks}}
}
func (e *SpanMemEncoder) EncodeSpan(span Span) {
	data := span.dataWithContext(e.session, e.config, e.RuntimeContext())
	data.Lo.Encode(e)
	data.Hi.Encode(e)
}
func (e *SpanMemEncoder) EncodeSymbol(symbol Symbol) {
	e.EmitStr(symbol.AsStrWithContext(e.session, e.RuntimeContext()))
}
func (e *SpanMemEncoder) EncodeByteSymbol(symbol ByteSymbol) {
	e.EmitByteStr([]byte(symbol.AsByteStrWithContext(e.session, e.RuntimeContext())))
}
func (e *SpanMemEncoder) EncodeExpnId(ExpnId) {
	spanStaticPanicAt(e.RuntimeContext(), spanLibLocation(1468, 9), "cannot encode `ExpnId` with `MemEncoder`")
}
func (e *SpanMemEncoder) EncodeSyntaxContext(SyntaxContext) {
	spanStaticPanicAt(e.RuntimeContext(), spanLibLocation(1472, 9), "cannot encode `SyntaxContext` with `MemEncoder`")
}
func (e *SpanMemEncoder) EncodeCrateNum(crate CrateNum) { e.EmitU32(crate.AsU32()) }
func (e *SpanMemEncoder) EncodeDefIndex(DefIndex) {
	spanStaticPanicAt(e.RuntimeContext(), spanLibLocation(1480, 9), "cannot encode `DefIndex` with `MemEncoder`")
}
func (e *SpanMemEncoder) EncodeDefId(id DefId)           { id.Krate.Encode(e); id.Index.Encode(e) }
func (e *SpanMemEncoder) EncodeLocalDefId(id LocalDefId) { DefaultSpanEncodeLocalDefId(e, id) }

type SpanMemDecoder struct {
	*rustcserialize.MemDecoder
	session *SessionGlobals
	config  SpanEncodingConfig
}

// SpanFileEncoder supplies the source FileEncoder's SpanEncoder impl while
// borrowing an explicit session in place of scoped TLS.
type SpanFileEncoder struct {
	*rustcserialize.FileEncoder
	session *SessionGlobals
	config  SpanEncodingConfig
}

func NewSpanFileEncoder(session *SessionGlobals, path RustPath) (*SpanFileEncoder, error) {
	return NewSpanFileEncoderWithContext(session, path, rustfmt.Context{})
}
func NewSpanFileEncoderWithContext(session *SessionGlobals, path RustPath, context rustfmt.Context, config ...SpanEncodingConfig) (*SpanFileEncoder, error) {
	profile := spanEncodingConfig(session, config...)
	base, err := rustcserialize.NewFileEncoderWithContext(path.EncodedString(), context, rustcserialize.Profile{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks})
	if err != nil {
		return nil, err
	}
	return &SpanFileEncoder{FileEncoder: base, session: session, config: profile}, nil
}
func WrapSpanFileEncoder(session *SessionGlobals, base *rustcserialize.FileEncoder) *SpanFileEncoder {
	profile := base.BuildProfile()
	return &SpanFileEncoder{FileEncoder: base, session: session, config: SpanEncodingConfig{profile.DebugAssertions, profile.OverflowChecks}}
}
func (e *SpanFileEncoder) EncodeSpan(span Span) {
	data := span.dataWithContext(e.session, e.config, e.RuntimeContext())
	data.Lo.Encode(e)
	data.Hi.Encode(e)
}
func (e *SpanFileEncoder) EncodeSymbol(symbol Symbol) {
	e.EmitStr(symbol.AsStrWithContext(e.session, e.RuntimeContext()))
}
func (e *SpanFileEncoder) EncodeByteSymbol(symbol ByteSymbol) {
	e.EmitByteStr([]byte(symbol.AsByteStrWithContext(e.session, e.RuntimeContext())))
}
func (e *SpanFileEncoder) EncodeExpnId(ExpnId) {
	spanStaticPanicAt(e.RuntimeContext(), spanLibLocation(1431, 9), "cannot encode `ExpnId` with `FileEncoder`")
}
func (e *SpanFileEncoder) EncodeSyntaxContext(SyntaxContext) {
	spanStaticPanicAt(e.RuntimeContext(), spanLibLocation(1435, 9), "cannot encode `SyntaxContext` with `FileEncoder`")
}
func (e *SpanFileEncoder) EncodeCrateNum(crate CrateNum) { e.EmitU32(crate.AsU32()) }
func (e *SpanFileEncoder) EncodeDefIndex(DefIndex) {
	spanStaticPanicAt(e.RuntimeContext(), spanLibLocation(1443, 9), "cannot encode `DefIndex` with `FileEncoder`")
}
func (e *SpanFileEncoder) EncodeDefId(id DefId)           { id.Krate.Encode(e); id.Index.Encode(e) }
func (e *SpanFileEncoder) EncodeLocalDefId(id LocalDefId) { DefaultSpanEncodeLocalDefId(e, id) }

func NewSpanMemDecoder(session *SessionGlobals, data []byte, position uint) (*SpanMemDecoder, error) {
	return NewSpanMemDecoderWithContext(session, data, position, rustfmt.Context{})
}
func NewSpanMemDecoderWithContext(session *SessionGlobals, data []byte, position uint, context rustfmt.Context, config ...SpanEncodingConfig) (*SpanMemDecoder, error) {
	profile := spanEncodingConfig(session, config...)
	base, err := rustcserialize.NewMemDecoderWithContext(data, position, context, rustcserialize.Profile{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks})
	if err != nil {
		return nil, err
	}
	return &SpanMemDecoder{MemDecoder: base, session: session, config: profile}, nil
}
func WrapSpanMemDecoder(session *SessionGlobals, base *rustcserialize.MemDecoder) *SpanMemDecoder {
	profile := base.BuildProfile()
	return &SpanMemDecoder{MemDecoder: base, session: session, config: SpanEncodingConfig{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks}}
}
func (d *SpanMemDecoder) DecodeSpan() Span {
	lo := DecodeBytePos(d)
	hi := DecodeBytePos(d)
	return newOrderedSpanWithContext(d.session, lo, hi, RootSyntaxContext(), None[LocalDefId](), d.config, d.RuntimeContext())
}
func (d *SpanMemDecoder) DecodeSymbol() Symbol {
	return InternSymbolWithContext(d.session, d.ReadStr(), d.RuntimeContext())
}
func (d *SpanMemDecoder) DecodeByteSymbol() ByteSymbol {
	return InternByteSymbolWithContext(d.session, d.ReadByteStr(), d.RuntimeContext())
}
func (d *SpanMemDecoder) DecodeDefIndex() DefIndex {
	spanStaticPanicAt(d.RuntimeContext(), spanLibLocation(1590, 9), "cannot decode `DefIndex` with `MemDecoder`")
	return DefIndex{}
}
func (d *SpanMemDecoder) DecodeLocalDefId() LocalDefId {
	return d.DecodeDefId().ExpectLocalWithContext(d.RuntimeContext())
}
func (d *SpanMemDecoder) DecodeExpnId() ExpnId {
	spanStaticPanicAt(d.RuntimeContext(), spanLibLocation(1607, 9), "cannot decode `ExpnId` with `MemDecoder`")
	return ExpnId{}
}
func (d *SpanMemDecoder) DecodeSyntaxContext() SyntaxContext {
	spanStaticPanicAt(d.RuntimeContext(), spanLibLocation(1611, 9), "cannot decode `SyntaxContext` with `MemDecoder`")
	return SyntaxContext{}
}
func (d *SpanMemDecoder) DecodeCrateNum() CrateNum {
	return CrateNumFromU32WithContext(d.ReadU32(), d.RuntimeContext())
}
func (d *SpanMemDecoder) DecodeDefId() DefId {
	return DefId{Krate: DecodeCrateNum(d), Index: DecodeDefIndex(d)}
}
func (d *SpanMemDecoder) DecodeAttrId() AttrId {
	spanStaticPanicAt(d.RuntimeContext(), spanLibLocation(1623, 9), "cannot decode `AttrId` with `MemDecoder`")
	return AttrId{}
}
