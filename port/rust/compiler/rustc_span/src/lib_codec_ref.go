// SPDX-License-Identifier: MIT OR Apache-2.0
// Borrowed complete trait dictionaries for the pinned lib.rs codec interfaces.
// Callbacks borrow the actual source S cell and the CURRENT operation context.
// Ops are static code/borrowed metadata, never an unrepresented owning closure.
package rustcspan

import (
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type SpanEncoderBorrow[S any] struct {
	Value   *S
	Context rustfmt.Context
	Encoder *SpanEncoderRef[S]
}
type SpanEncoderOps[S any] struct {
	EmitUsize           func(SpanEncoderBorrow[S], uint)
	EmitU128            func(SpanEncoderBorrow[S], rustcserialize.Uint128)
	EmitU64             func(SpanEncoderBorrow[S], uint64)
	EmitU32             func(SpanEncoderBorrow[S], uint32)
	EmitU16             func(SpanEncoderBorrow[S], uint16)
	EmitU8              func(SpanEncoderBorrow[S], uint8)
	EmitIsize           func(SpanEncoderBorrow[S], int)
	EmitI128            func(SpanEncoderBorrow[S], rustcserialize.Int128)
	EmitI64             func(SpanEncoderBorrow[S], int64)
	EmitI32             func(SpanEncoderBorrow[S], int32)
	EmitI16             func(SpanEncoderBorrow[S], int16)
	EmitRawBytes        func(SpanEncoderBorrow[S], []byte)
	EncodeSpan          func(SpanEncoderBorrow[S], Span)
	EncodeSymbol        func(SpanEncoderBorrow[S], Symbol)
	EncodeByteSymbol    func(SpanEncoderBorrow[S], ByteSymbol)
	EncodeExpnId        func(SpanEncoderBorrow[S], ExpnId)
	EncodeSyntaxContext func(SpanEncoderBorrow[S], SyntaxContext)
	EncodeCrateNum      func(SpanEncoderBorrow[S], CrateNum)
	EncodeDefIndex      func(SpanEncoderBorrow[S], DefIndex)
	EncodeDefId         func(SpanEncoderBorrow[S], DefId)
	EmitI8              func(SpanEncoderBorrow[S], int8)
	EmitBool            func(SpanEncoderBorrow[S], bool)
	EmitChar            func(SpanEncoderBorrow[S], rune)
	EmitStr             func(SpanEncoderBorrow[S], string)
	EmitByteStr         func(SpanEncoderBorrow[S], []byte)
	EncodeLocalDefId    func(SpanEncoderBorrow[S], LocalDefId)
	EncodeDefKey        func(SpanEncoderBorrow[S], Option[DefIndex], SpanEncodableValue)
	DefPathHasher       func(SpanEncoderBorrow[S]) DefPathHasher
	// These are source TYPE properties, not observable callbacks. Set OpaqueBytes
	// only when the selected source S is exactly MemEncoder or FileEncoder.
	OpaqueBytes, DeterministicHashMaps bool
}
type SpanEncoderRef[S any] struct {
	value   *S
	ops     SpanEncoderOps[S]
	context rustfmt.Context
	profile SpanEncodingConfig
}

func NewSpanEncoderRef[S any](value *S, ops SpanEncoderOps[S], context rustfmt.Context, profile ...SpanEncodingConfig) *SpanEncoderRef[S] {
	if value == nil {
		context.Panic("rustcspan: nil Encoder self borrow")
	}
	if ops.EmitUsize == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitUsize")
	}
	if ops.EmitU128 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitU128")
	}
	if ops.EmitU64 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitU64")
	}
	if ops.EmitU32 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitU32")
	}
	if ops.EmitU16 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitU16")
	}
	if ops.EmitU8 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitU8")
	}
	if ops.EmitIsize == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitIsize")
	}
	if ops.EmitI128 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitI128")
	}
	if ops.EmitI64 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitI64")
	}
	if ops.EmitI32 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitI32")
	}
	if ops.EmitI16 == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitI16")
	}
	if ops.EmitRawBytes == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EmitRawBytes")
	}
	if ops.EncodeSpan == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeSpan")
	}
	if ops.EncodeSymbol == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeSymbol")
	}
	if ops.EncodeByteSymbol == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeByteSymbol")
	}
	if ops.EncodeExpnId == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeExpnId")
	}
	if ops.EncodeSyntaxContext == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeSyntaxContext")
	}
	if ops.EncodeCrateNum == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeCrateNum")
	}
	if ops.EncodeDefIndex == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeDefIndex")
	}
	if ops.EncodeDefId == nil {
		context.Panic("rustcspan: incomplete Encoder dictionary: EncodeDefId")
	}
	return &SpanEncoderRef[S]{value: value, ops: ops, context: context, profile: spanEncodingConfig(nil, profile...)}
}
func (e *SpanEncoderRef[S]) borrow() SpanEncoderBorrow[S] {
	return SpanEncoderBorrow[S]{Value: e.value, Context: e.context, Encoder: e}
}
func (e *SpanEncoderRef[S]) RuntimeContext() rustfmt.Context { return e.context }
func (e *SpanEncoderRef[S]) BuildProfile() rustcserialize.Profile {
	return rustcserialize.Profile{DebugAssertions: e.profile.DebugAssertions, OverflowChecks: e.profile.OverflowChecks}
}
func (e *SpanEncoderRef[S]) WithContext(context rustfmt.Context) *SpanEncoderRef[S] {
	view := *e
	view.context = context
	return &view
}
func (e *SpanEncoderRef[S]) OpaqueContiguousBytes() bool           { return e.ops.OpaqueBytes }
func (e *SpanEncoderRef[S]) DeterministicHashMapEncode() bool      { return e.ops.DeterministicHashMaps }
func (e *SpanEncoderRef[S]) EmitUsize(value uint)                  { e.ops.EmitUsize(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitU128(value rustcserialize.Uint128) { e.ops.EmitU128(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitU64(value uint64)                  { e.ops.EmitU64(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitU32(value uint32)                  { e.ops.EmitU32(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitU16(value uint16)                  { e.ops.EmitU16(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitU8(value uint8)                    { e.ops.EmitU8(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitIsize(value int)                   { e.ops.EmitIsize(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitI128(value rustcserialize.Int128)  { e.ops.EmitI128(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitI64(value int64)                   { e.ops.EmitI64(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitI32(value int32)                   { e.ops.EmitI32(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitI16(value int16)                   { e.ops.EmitI16(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitRawBytes(value []byte)             { e.ops.EmitRawBytes(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EncodeSpan(value Span)                 { e.ops.EncodeSpan(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EncodeSymbol(value Symbol)             { e.ops.EncodeSymbol(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EncodeByteSymbol(value ByteSymbol) {
	e.ops.EncodeByteSymbol(e.borrow(), value)
}
func (e *SpanEncoderRef[S]) EncodeExpnId(value ExpnId) { e.ops.EncodeExpnId(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EncodeSyntaxContext(value SyntaxContext) {
	e.ops.EncodeSyntaxContext(e.borrow(), value)
}
func (e *SpanEncoderRef[S]) EncodeCrateNum(value CrateNum) { e.ops.EncodeCrateNum(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EncodeDefIndex(value DefIndex) { e.ops.EncodeDefIndex(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EncodeDefId(value DefId)       { e.ops.EncodeDefId(e.borrow(), value) }
func (e *SpanEncoderRef[S]) EmitI8(value int8) {
	if e.ops.EmitI8 != nil {
		e.ops.EmitI8(e.borrow(), value)
		return
	}
	rustcserialize.DefaultEmitI8(e, value)
}
func (e *SpanEncoderRef[S]) EmitBool(value bool) {
	if e.ops.EmitBool != nil {
		e.ops.EmitBool(e.borrow(), value)
		return
	}
	rustcserialize.DefaultEmitBool(e, value)
}
func (e *SpanEncoderRef[S]) EmitChar(value rune) {
	if e.ops.EmitChar != nil {
		e.ops.EmitChar(e.borrow(), value)
		return
	}
	rustcserialize.DefaultEmitChar(e, value)
}
func (e *SpanEncoderRef[S]) EmitStr(value string) {
	if e.ops.EmitStr != nil {
		e.ops.EmitStr(e.borrow(), value)
		return
	}
	rustcserialize.DefaultEmitStr(e, value)
}
func (e *SpanEncoderRef[S]) EmitByteStr(value []byte) {
	if e.ops.EmitByteStr != nil {
		e.ops.EmitByteStr(e.borrow(), value)
		return
	}
	rustcserialize.DefaultEmitByteStr(e, value)
}
func (e *SpanEncoderRef[S]) EncodeLocalDefId(value LocalDefId) {
	if e.ops.EncodeLocalDefId != nil {
		e.ops.EncodeLocalDefId(e.borrow(), value)
		return
	}
	DefaultSpanEncodeLocalDefId(e, value)
}
func (e *SpanEncoderRef[S]) EncodeDefKey(parent Option[DefIndex], value SpanEncodableValue) {
	owned := value.MoveWithContext(e.context)
	defer spanCloseOwnedCallback(e.context, owned.CloseWithContext)
	if e.ops.EncodeDefKey != nil {
		e.ops.EncodeDefKey(e.borrow(), parent, owned)
		return
	}
	defaultSpanEncodeDefKeyBorrowed(e, parent, owned, e.context)
}
func (e *SpanEncoderRef[S]) DefPathHasher() DefPathHasher {
	if e.ops.DefPathHasher == nil {
		return DefaultSpanDefPathHasher(e)
	}
	return e.ops.DefPathHasher(e.borrow())
}
func (e *SpanEncoderRef[S]) DefPathHasherWithContext(context rustfmt.Context) DefPathHasher {
	return e.WithContext(context).DefPathHasher()
}

type SpanDecoderBorrow[S any] struct {
	Value   *S
	Context rustfmt.Context
	Decoder *SpanDecoderRef[S]
}
type SpanDecoderOps[S any] struct {
	ReadUsize           func(SpanDecoderBorrow[S]) uint
	ReadU128            func(SpanDecoderBorrow[S]) rustcserialize.Uint128
	ReadU64             func(SpanDecoderBorrow[S]) uint64
	ReadU32             func(SpanDecoderBorrow[S]) uint32
	ReadU16             func(SpanDecoderBorrow[S]) uint16
	ReadU8              func(SpanDecoderBorrow[S]) uint8
	ReadIsize           func(SpanDecoderBorrow[S]) int
	ReadI128            func(SpanDecoderBorrow[S]) rustcserialize.Int128
	ReadI64             func(SpanDecoderBorrow[S]) int64
	ReadI32             func(SpanDecoderBorrow[S]) int32
	ReadI16             func(SpanDecoderBorrow[S]) int16
	PeekByte            func(SpanDecoderBorrow[S]) uint8
	Position            func(SpanDecoderBorrow[S]) uint
	DecodeSymbol        func(SpanDecoderBorrow[S]) Symbol
	DecodeByteSymbol    func(SpanDecoderBorrow[S]) ByteSymbol
	DecodeDefIndex      func(SpanDecoderBorrow[S]) DefIndex
	DecodeLocalDefId    func(SpanDecoderBorrow[S]) LocalDefId
	DecodeSpan          func(SpanDecoderBorrow[S]) Span
	DecodeExpnId        func(SpanDecoderBorrow[S]) ExpnId
	DecodeSyntaxContext func(SpanDecoderBorrow[S]) SyntaxContext
	DecodeCrateNum      func(SpanDecoderBorrow[S]) CrateNum
	DecodeDefId         func(SpanDecoderBorrow[S]) DefId
	DecodeAttrId        func(SpanDecoderBorrow[S]) AttrId
	ReadI8              func(SpanDecoderBorrow[S]) int8
	ReadBool            func(SpanDecoderBorrow[S]) bool
	ReadChar            func(SpanDecoderBorrow[S]) rune
	ReadStr             func(SpanDecoderBorrow[S]) string
	ReadByteStr         func(SpanDecoderBorrow[S]) []byte
	ReadRawBytes        func(SpanDecoderBorrow[S], uint) []byte
}
type SpanDecoderRef[S any] struct {
	value   *S
	ops     SpanDecoderOps[S]
	context rustfmt.Context
	profile SpanEncodingConfig
}

func NewSpanDecoderRef[S any](value *S, ops SpanDecoderOps[S], context rustfmt.Context, profile ...SpanEncodingConfig) *SpanDecoderRef[S] {
	if value == nil {
		context.Panic("rustcspan: nil Decoder self borrow")
	}
	if ops.ReadUsize == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadUsize")
	}
	if ops.ReadU128 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadU128")
	}
	if ops.ReadU64 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadU64")
	}
	if ops.ReadU32 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadU32")
	}
	if ops.ReadU16 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadU16")
	}
	if ops.ReadU8 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadU8")
	}
	if ops.ReadIsize == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadIsize")
	}
	if ops.ReadI128 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadI128")
	}
	if ops.ReadI64 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadI64")
	}
	if ops.ReadI32 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadI32")
	}
	if ops.ReadI16 == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadI16")
	}
	if ops.PeekByte == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: PeekByte")
	}
	if ops.Position == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: Position")
	}
	if ops.DecodeSymbol == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeSymbol")
	}
	if ops.DecodeByteSymbol == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeByteSymbol")
	}
	if ops.DecodeDefIndex == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeDefIndex")
	}
	if ops.DecodeLocalDefId == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeLocalDefId")
	}
	if ops.DecodeSpan == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeSpan")
	}
	if ops.DecodeExpnId == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeExpnId")
	}
	if ops.DecodeSyntaxContext == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeSyntaxContext")
	}
	if ops.DecodeCrateNum == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeCrateNum")
	}
	if ops.DecodeDefId == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeDefId")
	}
	if ops.DecodeAttrId == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: DecodeAttrId")
	}
	if ops.ReadRawBytes == nil {
		context.Panic("rustcspan: incomplete Decoder dictionary: ReadRawBytes")
	}
	return &SpanDecoderRef[S]{value: value, ops: ops, context: context, profile: spanEncodingConfig(nil, profile...)}
}
func (d *SpanDecoderRef[S]) borrow() SpanDecoderBorrow[S] {
	return SpanDecoderBorrow[S]{Value: d.value, Context: d.context, Decoder: d}
}
func (d *SpanDecoderRef[S]) RuntimeContext() rustfmt.Context { return d.context }
func (d *SpanDecoderRef[S]) BuildProfile() rustcserialize.Profile {
	return rustcserialize.Profile{DebugAssertions: d.profile.DebugAssertions, OverflowChecks: d.profile.OverflowChecks}
}
func (d *SpanDecoderRef[S]) WithContext(context rustfmt.Context) *SpanDecoderRef[S] {
	view := *d
	view.context = context
	return &view
}
func (d *SpanDecoderRef[S]) ReadUsize() uint                  { return d.ops.ReadUsize(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadU128() rustcserialize.Uint128 { return d.ops.ReadU128(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadU64() uint64                  { return d.ops.ReadU64(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadU32() uint32                  { return d.ops.ReadU32(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadU16() uint16                  { return d.ops.ReadU16(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadU8() uint8                    { return d.ops.ReadU8(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadIsize() int                   { return d.ops.ReadIsize(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadI128() rustcserialize.Int128  { return d.ops.ReadI128(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadI64() int64                   { return d.ops.ReadI64(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadI32() int32                   { return d.ops.ReadI32(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadI16() int16                   { return d.ops.ReadI16(d.borrow()) }
func (d *SpanDecoderRef[S]) PeekByte() uint8                  { return d.ops.PeekByte(d.borrow()) }
func (d *SpanDecoderRef[S]) Position() uint                   { return d.ops.Position(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeSymbol() Symbol             { return d.ops.DecodeSymbol(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeByteSymbol() ByteSymbol     { return d.ops.DecodeByteSymbol(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeDefIndex() DefIndex         { return d.ops.DecodeDefIndex(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeLocalDefId() LocalDefId     { return d.ops.DecodeLocalDefId(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeSpan() Span                 { return d.ops.DecodeSpan(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeExpnId() ExpnId             { return d.ops.DecodeExpnId(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeSyntaxContext() SyntaxContext {
	return d.ops.DecodeSyntaxContext(d.borrow())
}
func (d *SpanDecoderRef[S]) DecodeCrateNum() CrateNum { return d.ops.DecodeCrateNum(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeDefId() DefId       { return d.ops.DecodeDefId(d.borrow()) }
func (d *SpanDecoderRef[S]) DecodeAttrId() AttrId     { return d.ops.DecodeAttrId(d.borrow()) }
func (d *SpanDecoderRef[S]) ReadI8() int8 {
	if d.ops.ReadI8 != nil {
		return d.ops.ReadI8(d.borrow())
	}
	return rustcserialize.DefaultReadI8(d)
}
func (d *SpanDecoderRef[S]) ReadBool() bool {
	if d.ops.ReadBool != nil {
		return d.ops.ReadBool(d.borrow())
	}
	return rustcserialize.DefaultReadBool(d)
}
func (d *SpanDecoderRef[S]) ReadChar() rune {
	if d.ops.ReadChar != nil {
		return d.ops.ReadChar(d.borrow())
	}
	return rustcserialize.DefaultReadChar(d)
}
func (d *SpanDecoderRef[S]) ReadStr() string {
	if d.ops.ReadStr != nil {
		return d.ops.ReadStr(d.borrow())
	}
	return rustcserialize.DefaultReadStr(d)
}
func (d *SpanDecoderRef[S]) ReadByteStr() []byte {
	if d.ops.ReadByteStr != nil {
		return d.ops.ReadByteStr(d.borrow())
	}
	return rustcserialize.DefaultReadByteStr(d)
}
func (d *SpanDecoderRef[S]) ReadRawBytes(n uint) []byte { return d.ops.ReadRawBytes(d.borrow(), n) }
