// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL, genuine used-contract port of rustc_serialize at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. This supplies the Encoder/Decoder
// primitive contracts and opaque implementations used by rustc_span. It does
// not claim the complete serialize.rs/lib.rs or their unrelated container APIs.
package rustcserialize

import (
	"encoding/binary"
	"unicode/utf8"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type Uint128 = rustchashes.Uint128

// Int128 carries the two's-complement bits of Rust i128.
type Int128 struct{ Lo, Hi uint64 }
type Profile struct{ DebugAssertions, OverflowChecks bool }

func DefaultProfile() Profile { return Profile{true, true} }

type Encoder interface {
	EmitUsize(uint)
	EmitU128(Uint128)
	EmitU64(uint64)
	EmitU32(uint32)
	EmitU16(uint16)
	EmitU8(uint8)
	EmitIsize(int)
	EmitI128(Int128)
	EmitI64(int64)
	EmitI32(int32)
	EmitI16(int16)
	EmitRawBytes([]byte)
}
type Decoder interface {
	ReadUsize() uint
	ReadU128() Uint128
	ReadU64() uint64
	ReadU32() uint32
	ReadU16() uint16
	ReadU8() uint8
	ReadIsize() int
	ReadI128() Int128
	ReadI64() int64
	ReadI32() int32
	ReadI16() int16
	ReadRawBytes(uint) []byte
	PeekByte() uint8
	Position() uint
}

func codecContext(value any) rustfmt.Context {
	if p, ok := value.(interface{ RuntimeContext() rustfmt.Context }); ok {
		return p.RuntimeContext()
	}
	return rustfmt.Context{}
}
func codecProfile(value any) Profile {
	if p, ok := value.(interface{ BuildProfile() Profile }); ok {
		return p.BuildProfile()
	}
	return DefaultProfile()
}
func EmitI8(e Encoder, v int8) {
	if override, ok := e.(interface{ EmitI8(int8) }); ok {
		override.EmitI8(v)
		return
	}
	DefaultEmitI8(e, v)
}
func DefaultEmitI8(e Encoder, v int8) { e.EmitU8(uint8(v)) }
func EmitBool(e Encoder, v bool) {
	if override, ok := e.(interface{ EmitBool(bool) }); ok {
		override.EmitBool(v)
		return
	}
	DefaultEmitBool(e, v)
}
func DefaultEmitBool(e Encoder, v bool) {
	if v {
		e.EmitU8(1)
	} else {
		e.EmitU8(0)
	}
}
func EmitChar(e Encoder, v rune) {
	if override, ok := e.(interface{ EmitChar(rune) }); ok {
		override.EmitChar(v)
		return
	}
	DefaultEmitChar(e, v)
}
func DefaultEmitChar(e Encoder, v rune) {
	if v < 0 || v > utf8.MaxRune || v >= 0xd800 && v <= 0xdfff {
		codecContext(e).Panic("rustcserialize: char requires a Unicode scalar")
	}
	e.EmitU32(uint32(v))
}
func EmitStr(e Encoder, v string) {
	if override, ok := e.(interface{ EmitStr(string) }); ok {
		override.EmitStr(v)
		return
	}
	DefaultEmitStr(e, v)
}
func DefaultEmitStr(e Encoder, v string) {
	if !utf8.ValidString(v) {
		codecContext(e).Panic("rustcserialize: str requires valid UTF-8")
	}
	e.EmitUsize(uint(len(v)))
	e.EmitRawBytes([]byte(v))
	e.EmitU8(0xc1)
}
func EmitByteStr(e Encoder, v []byte) {
	if override, ok := e.(interface{ EmitByteStr([]byte) }); ok {
		override.EmitByteStr(v)
		return
	}
	DefaultEmitByteStr(e, v)
}
func DefaultEmitByteStr(e Encoder, v []byte) {
	e.EmitUsize(uint(len(v)))
	e.EmitRawBytes(v)
	e.EmitU8(0xc2)
}
func ReadI8(d Decoder) int8 {
	if override, ok := d.(interface{ ReadI8() int8 }); ok {
		return override.ReadI8()
	}
	return DefaultReadI8(d)
}
func DefaultReadI8(d Decoder) int8 { return int8(d.ReadU8()) }
func ReadBool(d Decoder) bool {
	if override, ok := d.(interface{ ReadBool() bool }); ok {
		return override.ReadBool()
	}
	return DefaultReadBool(d)
}
func DefaultReadBool(d Decoder) bool { return d.ReadU8() != 0 }
func ReadChar(d Decoder) rune {
	if override, ok := d.(interface{ ReadChar() rune }); ok {
		return override.ReadChar()
	}
	return DefaultReadChar(d)
}
func DefaultReadChar(d Decoder) rune {
	v := d.ReadU32()
	if v > utf8.MaxRune || v >= 0xd800 && v <= 0xdfff {
		codecContext(d).Panic("called `Option::unwrap()` on a `None` value")
	}
	return rune(v)
}
func ReadStr(d Decoder) string {
	if override, ok := d.(interface{ ReadStr() string }); ok {
		return override.ReadStr()
	}
	return DefaultReadStr(d)
}
func DefaultReadStr(d Decoder) string {
	n := d.ReadUsize()
	if codecProfile(d).OverflowChecks && n == ^uint(0) {
		codecContext(d).Panic("attempt to add with overflow")
	}
	data := d.ReadRawBytes(n + 1)
	if n >= uint(len(data)) {
		codecContext(d).Panic("index out of bounds")
	}
	if data[n] != 0xc1 {
		codecContext(d).Panic("assertion failed: bytes[len] == STR_SENTINEL")
	}
	if !utf8.Valid(data[:n]) {
		codecContext(d).Panic("rustcserialize: decoded str outside valid Rust UTF-8 domain")
	}
	return string(data[:n])
}
func ReadByteStr(d Decoder) []byte {
	if override, ok := d.(interface{ ReadByteStr() []byte }); ok {
		return override.ReadByteStr()
	}
	return DefaultReadByteStr(d)
}
func DefaultReadByteStr(d Decoder) []byte {
	n := d.ReadUsize()
	if codecProfile(d).OverflowChecks && n == ^uint(0) {
		codecContext(d).Panic("attempt to add with overflow")
	}
	data := d.ReadRawBytes(n + 1)
	if n >= uint(len(data)) {
		codecContext(d).Panic("index out of bounds")
	}
	if data[n] != 0xc2 {
		codecContext(d).Panic("assertion failed: bytes[len] == BYTE_STR_SENTINEL")
	}
	return data[:n]
}
func EncodeHash64(e Encoder, v rustchashes.Hash64) {
	var bytes [8]byte
	binary.LittleEndian.PutUint64(bytes[:], v.AsU64())
	e.EmitRawBytes(bytes[:])
}
func DecodeHash64(d Decoder) rustchashes.Hash64 {
	bytes := d.ReadRawBytes(8)
	if len(bytes) != 8 {
		codecContext(d).Panic("called `Result::unwrap()` on an `Err` value: TryFromSliceError(())")
	}
	return rustchashes.NewHash64(binary.LittleEndian.Uint64(bytes))
}
func EncodeHash128(e Encoder, v rustchashes.Hash128) {
	var bytes [16]byte
	n := v.AsU128()
	binary.LittleEndian.PutUint64(bytes[:8], n.Lo)
	binary.LittleEndian.PutUint64(bytes[8:], n.Hi)
	e.EmitRawBytes(bytes[:])
}
func DecodeHash128(d Decoder) rustchashes.Hash128 {
	bytes := d.ReadRawBytes(16)
	if len(bytes) != 16 {
		codecContext(d).Panic("called `Result::unwrap()` on an `Err` value: TryFromSliceError(())")
	}
	return rustchashes.NewHash128(Uint128{Lo: binary.LittleEndian.Uint64(bytes[:8]), Hi: binary.LittleEndian.Uint64(bytes[8:])})
}

// EncodeBytes selects specialization only for the exact raw concrete encoders.
// Source Self projections pass their explicit static strategy to the primary
// below. A promoted marker on a distinct Go wrapper is not Rust specialization.
func EncodeBytes(e Encoder, bytes []byte) {
	EncodeBytesWithStrategy(e, bytes, ByteVectorEncodeOps(e))
}

// EncodeBytesWithStrategy is the selected source [u8] Encodable operation.
// Its dictionary is static borrowed metadata; no old Context is captured.
func EncodeBytesWithStrategy(e Encoder, bytes []byte, strategy VectorEncodeOps[uint8]) {
	EncodeVector(e, VectorSlice[uint8]{Length: uint(len(bytes)), At: func(i uint) *uint8 { return &bytes[i] }, AsGoSlice: func() []uint8 { return bytes }}, strategy)
}
