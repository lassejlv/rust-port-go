// SPDX-License-Identifier: MIT OR Apache-2.0
// Position/metadata blocks from rustc_span/src/lib.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The whole lib port is in progress.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Pos is the legacy function convenience for the four associated/borrowed
// operations. PosOperations is the full current-context primary for arbitrary
// implementations; Bind creates this ephemeral view for one operation. A
// provider supplies real conversion code, with no generic fallback casts.
type Pos[P any] struct {
	FromUsize func(uint) P
	ToUsize   func(*P) uint
	FromU32   func(uint32) P
	ToU32     func(*P) uint32
}

type RelativeBytePos uint32
type CharPos uint

func RelativeBytePosFromU32(value uint32) RelativeBytePos { return RelativeBytePos(value) }
func RelativeBytePosFromUsize(value uint) RelativeBytePos { return RelativeBytePos(uint32(value)) }
func CharPosFromU32(value uint32) CharPos                 { return CharPos(value) }
func CharPosFromUsize(value uint) CharPos                 { return CharPos(value) }
func (value BytePos) ToU32() uint32                       { return uint32(value) }
func (value BytePos) ToUsize() uint                       { return uint(value) }
func (value RelativeBytePos) ToU32() uint32               { return uint32(value) }
func (value RelativeBytePos) ToUsize() uint               { return uint(value) }
func (value RelativeBytePos) AsU32() uint32               { return value.ToU32() }
func (value RelativeBytePos) AsUsize() uint               { return value.ToUsize() }
func (value CharPos) ToU32() uint32                       { return uint32(value) }
func (value CharPos) ToUsize() uint                       { return uint(value) }
func (value CharPos) AsU32() uint32                       { return value.ToU32() }
func (value CharPos) AsUsize() uint                       { return value.ToUsize() }
func BytePosOps() Pos[BytePos] {
	return Pos[BytePos]{BytePosFromUsize, func(v *BytePos) uint { return v.ToUsize() }, BytePosFromU32, func(v *BytePos) uint32 { return v.ToU32() }}
}
func RelativeBytePosOps() Pos[RelativeBytePos] {
	return Pos[RelativeBytePos]{RelativeBytePosFromUsize, func(v *RelativeBytePos) uint { return v.ToUsize() }, RelativeBytePosFromU32, func(v *RelativeBytePos) uint32 { return v.ToU32() }}
}
func CharPosOps() Pos[CharPos] {
	return Pos[CharPos]{CharPosFromUsize, func(v *CharPos) uint { return v.ToUsize() }, CharPosFromU32, func(v *CharPos) uint32 { return v.ToU32() }}
}

func spanPosAdd(a, b uint32, config SpanEncodingConfig, context rustfmt.Context, locations ...spanLibCodePoint) uint32 {
	if config.OverflowChecks && b > ^uint32(0)-a {
		spanStaticPanicAt(context, spanPosPanicLocation(locations), "attempt to add with overflow")
	}
	return a + b
}
func spanPosSub(a, b uint32, config SpanEncodingConfig, context rustfmt.Context, locations ...spanLibCodePoint) uint32 {
	if config.OverflowChecks && b > a {
		spanStaticPanicAt(context, spanPosPanicLocation(locations), "attempt to subtract with overflow")
	}
	return a - b
}
func spanCharPosAdd(a, b uint, config SpanEncodingConfig, context rustfmt.Context, locations ...spanLibCodePoint) uint {
	if config.OverflowChecks && b > ^uint(0)-a {
		spanStaticPanicAt(context, spanPosPanicLocation(locations), "attempt to add with overflow")
	}
	return a + b
}
func spanCharPosSub(a, b uint, config SpanEncodingConfig, context rustfmt.Context, locations ...spanLibCodePoint) uint {
	if config.OverflowChecks && b > a {
		spanStaticPanicAt(context, spanPosPanicLocation(locations), "attempt to subtract with overflow")
	}
	return a - b
}

// Add/Sub are the source operators; Go's built-in arithmetic on named integer
// types does not inherit Rust's independently selected overflow profile.
func (value BytePos) Add(other BytePos, config ...SpanEncodingConfig) BytePos {
	return value.AddWithContext(other, spanEncodingConfig(nil, config...), rustfmt.Context{})
}
func (value BytePos) Sub(other BytePos, config ...SpanEncodingConfig) BytePos {
	return value.SubWithContext(other, spanEncodingConfig(nil, config...), rustfmt.Context{})
}
func (value BytePos) AddWithContext(other BytePos, config SpanEncodingConfig, context rustfmt.Context) BytePos {
	return BytePos(spanPosAdd(uint32(value), uint32(other), config, context, spanLibLocation(2747, 1)))
}
func (value BytePos) SubWithContext(other BytePos, config SpanEncodingConfig, context rustfmt.Context) BytePos {
	return BytePos(spanPosSub(uint32(value), uint32(other), config, context, spanLibLocation(2747, 1)))
}
func (value RelativeBytePos) Add(other RelativeBytePos, config ...SpanEncodingConfig) RelativeBytePos {
	return value.AddWithContext(other, spanEncodingConfig(nil, config...), rustfmt.Context{})
}
func (value RelativeBytePos) Sub(other RelativeBytePos, config ...SpanEncodingConfig) RelativeBytePos {
	return value.SubWithContext(other, spanEncodingConfig(nil, config...), rustfmt.Context{})
}
func (value RelativeBytePos) AddWithContext(other RelativeBytePos, config SpanEncodingConfig, context rustfmt.Context) RelativeBytePos {
	return RelativeBytePos(spanPosAdd(uint32(value), uint32(other), config, context, spanLibLocation(2747, 1)))
}
func (value RelativeBytePos) SubWithContext(other RelativeBytePos, config SpanEncodingConfig, context rustfmt.Context) RelativeBytePos {
	return RelativeBytePos(spanPosSub(uint32(value), uint32(other), config, context, spanLibLocation(2747, 1)))
}
func (value CharPos) Add(other CharPos, config ...SpanEncodingConfig) CharPos {
	return value.AddWithContext(other, spanEncodingConfig(nil, config...), rustfmt.Context{})
}
func (value CharPos) Sub(other CharPos, config ...SpanEncodingConfig) CharPos {
	return value.SubWithContext(other, spanEncodingConfig(nil, config...), rustfmt.Context{})
}
func (value CharPos) AddWithContext(other CharPos, config SpanEncodingConfig, context rustfmt.Context) CharPos {
	return CharPos(spanCharPosAdd(uint(value), uint(other), config, context, spanLibLocation(2747, 1)))
}
func (value CharPos) SubWithContext(other CharPos, config SpanEncodingConfig, context rustfmt.Context) CharPos {
	return CharPos(spanCharPosSub(uint(value), uint(other), config, context, spanLibLocation(2747, 1)))
}
func (value RelativeBytePos) Clone() RelativeBytePos           { return value }
func (value CharPos) Clone() CharPos                           { return value }
func (value *RelativeBytePos) CloneFrom(other RelativeBytePos) { *value = other }
func (value *CharPos) CloneFrom(other CharPos)                 { *value = other }
func (value RelativeBytePos) Hash(h spanU32Hasher)             { h.WriteU32(uint32(value)) }
func (value RelativeBytePos) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU32(uint32(value))
}
func (value RelativeBytePos) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "RelativeBytePos", pretty)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(value), 32, child) }))
	return b.Finish()
}
func (value CharPos) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "CharPos", pretty)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value), uint(spanUintBits()), child)
	}))
	return b.Finish()
}
func spanUintBits() int { return 32 << (^uint(0) >> 63) }

type MultiByteChar struct {
	Pos   RelativeBytePos
	Bytes uint8
}
type NormalizedPos struct {
	Pos  RelativeBytePos
	Diff uint32
}

func (value MultiByteChar) Clone() MultiByteChar           { return value }
func (value NormalizedPos) Clone() NormalizedPos           { return value }
func (value MultiByteChar) Equal(other MultiByteChar) bool { return value == other }
func (value NormalizedPos) Equal(other NormalizedPos) bool { return value == other }
func (value MultiByteChar) StableHash(context datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	value.Pos.StableHash(context, h)
	h.WriteU8(value.Bytes)
}
func (value NormalizedPos) StableHash(context datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	value.Pos.StableHash(context, h)
	h.WriteU32(value.Diff)
}
