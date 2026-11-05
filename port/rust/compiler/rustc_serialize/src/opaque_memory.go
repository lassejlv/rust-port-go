// SPDX-License-Identifier: MIT OR Apache-2.0
// Genuine scoped opaque::MemEncoder/MemDecoder and LEB128 algorithms from the
// pinned compiler source; no whole rustc_serialize file completion is claimed.
package rustcserialize

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const MagicEndBytes = "rust-end-file"

// MemDecoderExhaustedPanicSite selects the caller's exact source location for
// opaque.rs's shared decoder_exhausted failure. No build path is inferred.
const MemDecoderExhaustedPanicSite = "rustc_serialize.opaque.decoder_exhausted"

type MemEncoder struct {
	Data     []byte
	context  rustfmt.Context
	profile  Profile
	consumed bool
}

func (*MemEncoder) OpaqueContiguousBytes() bool { return true }

func NewMemEncoder() *MemEncoder {
	return NewMemEncoderWithContext(rustfmt.Context{}, DefaultProfile())
}
func NewMemEncoderWithContext(context rustfmt.Context, profile Profile) *MemEncoder {
	return &MemEncoder{Data: []byte{}, context: contextGuardsOnly(context), profile: profile}
}
func (e *MemEncoder) RuntimeContext() rustfmt.Context           { return e.context }
func (e *MemEncoder) BuildProfile() Profile                     { return e.profile }
func (e *MemEncoder) SetRuntimeContext(context rustfmt.Context) { e.context = context }
func (e *MemEncoder) check() {
	if e.consumed {
		e.context.Panic("rustcserialize: use of consumed MemEncoder")
	}
}
func (e *MemEncoder) Position() uint { e.check(); return uint(len(e.Data)) }
func (e *MemEncoder) Finish() []byte {
	e.check()
	e.consumed = true
	data := e.Data
	e.Data = nil
	return data
}
func (e *MemEncoder) reserve(additional uint) {
	required := uint(len(e.Data)) + additional
	if required < uint(len(e.Data)) || required > uint(^uint(0)>>1) {
		e.context.Panic("capacity overflow")
	}
	if required <= uint(cap(e.Data)) {
		return
	}
	newCap := uint(cap(e.Data)) * 2
	if newCap < required {
		newCap = required
	}
	if newCap < 8 {
		newCap = 8
	}
	if newCap > uint(^uint(0)>>1) {
		newCap = required
	}
	next := make([]byte, len(e.Data), newCap)
	copy(next, e.Data)
	e.Data = next
}

// WriteWith's mutable window is a temporary source borrow. The visitor must
// not retain it. N is the Go runtime representation of Rust's const argument.
func (e *MemEncoder) WriteWith(n uint, visitor func([]byte) uint) {
	e.check()
	e.reserve(n)
	old := len(e.Data)
	window := e.Data[:old+int(n)][old:]
	clear(window)
	written := visitor(window)
	if written > n {
		e.context.Panic(fmt.Sprintf("MemEncoder::write_with::<%d> cannot be used to write %d bytes", n, written))
	}
	e.Data = e.Data[:old+int(written)]
}
func (e *MemEncoder) WriteArray(data []byte) {
	e.WriteWith(uint(len(data)), func(dest []byte) uint { copy(dest, data); return uint(len(data)) })
}
func (e *MemEncoder) EmitRawBytes(data []byte) {
	e.check()
	e.reserve(uint(len(data)))
	e.Data = append(e.Data, data...)
}
func (e *MemEncoder) EmitU8(v uint8) { e.WriteArray([]byte{v}) }
func (e *MemEncoder) EmitU16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	e.WriteArray(b[:])
}
func (e *MemEncoder) EmitU32(v uint32) { e.writeUnsigned(Uint128{Lo: uint64(v)}, 5) }
func (e *MemEncoder) EmitU64(v uint64) { e.writeUnsigned(Uint128{Lo: v}, 10) }
func (e *MemEncoder) EmitUsize(v uint) {
	e.writeUnsigned(Uint128{Lo: uint64(v)}, uint((strconv.IntSize+6)/7))
}
func (e *MemEncoder) EmitU128(v Uint128) { e.writeUnsigned(v, 19) }
func (e *MemEncoder) EmitI16(v int16)    { e.EmitU16(uint16(v)) }
func (e *MemEncoder) EmitI32(v int32) {
	e.writeSigned(Int128{Lo: uint64(int64(v)), Hi: signHi(int64(v))}, 5)
}
func (e *MemEncoder) EmitI64(v int64) { e.writeSigned(Int128{Lo: uint64(v), Hi: signHi(v)}, 10) }
func (e *MemEncoder) EmitIsize(v int) {
	e.writeSigned(Int128{Lo: uint64(int64(v)), Hi: signHi(int64(v))}, uint((strconv.IntSize+6)/7))
}
func (e *MemEncoder) EmitI128(v Int128) { e.writeSigned(v, 19) }
func signHi(v int64) uint64 {
	if v < 0 {
		return ^uint64(0)
	}
	return 0
}
func (e *MemEncoder) writeUnsigned(v Uint128, n uint) {
	e.WriteWith(n, func(out []byte) uint { return writeUnsignedLEB(out, v) })
}
func (e *MemEncoder) writeSigned(v Int128, n uint) {
	e.WriteWith(n, func(out []byte) uint { return writeSignedLEB(out, v) })
}
func writeUnsignedLEB(out []byte, value Uint128) uint {
	i := uint(0)
	for {
		if value.Hi == 0 && value.Lo < 0x80 {
			out[i] = byte(value.Lo)
			return i + 1
		}
		out[i] = byte(value.Lo&0x7f) | 0x80
		value.Lo = value.Lo>>7 | value.Hi<<57
		value.Hi >>= 7
		i++
	}
}
func writeSignedLEB(out []byte, value Int128) uint {
	i := uint(0)
	for {
		b := byte(value.Lo) & 0x7f
		value.Lo = value.Lo>>7 | value.Hi<<57
		value.Hi = uint64(int64(value.Hi) >> 7)
		more := !((value.Lo == 0 && value.Hi == 0 && b&0x40 == 0) || (value.Lo == ^uint64(0) && value.Hi == ^uint64(0) && b&0x40 != 0))
		if more {
			b |= 0x80
		}
		out[i] = b
		i++
		if !more {
			return i
		}
	}
}
func (e *MemEncoder) EmitI8(v int8)        { DefaultEmitI8(e, v) }
func (e *MemEncoder) EmitBool(v bool)      { DefaultEmitBool(e, v) }
func (e *MemEncoder) EmitChar(v rune)      { DefaultEmitChar(e, v) }
func (e *MemEncoder) EmitStr(v string)     { DefaultEmitStr(e, v) }
func (e *MemEncoder) EmitByteStr(v []byte) { DefaultEmitByteStr(e, v) }

type MemDecoder struct {
	data     []byte
	position uint
	context  rustfmt.Context
	profile  Profile
}

func NewMemDecoder(data []byte, position uint) (*MemDecoder, error) {
	return NewMemDecoderWithContext(data, position, rustfmt.Context{}, DefaultProfile())
}
func NewMemDecoderWithContext(data []byte, position uint, context rustfmt.Context, profile Profile) (*MemDecoder, error) {
	if !bytes.HasSuffix(data, []byte(MagicEndBytes)) {
		return nil, errors.New("missing rust-end-file footer")
	}
	data = data[:len(data)-len(MagicEndBytes)]
	if position > uint(len(data)) {
		context.Panic("range start index out of range for MemDecoder")
	}
	return &MemDecoder{data: data, position: position, context: contextGuardsOnly(context), profile: profile}, nil
}
func (d *MemDecoder) RuntimeContext() rustfmt.Context           { return d.context }
func (d *MemDecoder) BuildProfile() Profile                     { return d.profile }
func (d *MemDecoder) SetRuntimeContext(context rustfmt.Context) { d.context = context }
func (d *MemDecoder) Position() uint                            { return d.position }
func (d *MemDecoder) Len() uint                                 { return uint(len(d.data)) }
func (d *MemDecoder) Remaining() uint                           { return uint(len(d.data)) - d.position }
func (d *MemDecoder) SplitAt(position uint) *MemDecoder {
	if position > d.Len() {
		d.context.Panic("assertion failed: position <= self.len()")
	}
	out := *d
	out.position = position
	return &out
}
func (d *MemDecoder) WithPosition(position uint, call func(*MemDecoder)) {
	context := d.context
	if position >= d.Len() {
		d.decoderExhausted()
	}
	old := d.position
	d.position = position
	defer func() {
		if payload := recover(); payload != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			d.position = old
			context.ResumeUnwind(payload)
		}
		d.position = old
	}()
	call(d)
}
func (d *MemDecoder) ReadRawBytes(n uint) []byte {
	if n > d.Remaining() {
		d.decoderExhausted()
	}
	out := d.data[d.position : d.position+n]
	d.position += n
	return out
}
func (d *MemDecoder) ReadArray(n uint) []byte {
	out := make([]byte, n)
	copy(out, d.ReadRawBytes(n))
	return out
}
func (d *MemDecoder) ReadU8() uint8 {
	if d.position == d.Len() {
		d.decoderExhausted()
	}
	out := d.data[d.position]
	d.position++
	return out
}
func (d *MemDecoder) PeekByte() uint8 {
	if d.position == d.Len() {
		d.decoderExhausted()
	}
	return d.data[d.position]
}
func (d *MemDecoder) decoderExhausted() {
	d.context.PanicAt(rustfmt.SourcePanicInfo{
		Payload: "MemDecoder exhausted", Kind: rustfmt.PanicStaticStr,
		Location:  d.context.SourcePanicLocation(MemDecoderExhaustedPanicSite),
		CanUnwind: true,
	})
}
func (d *MemDecoder) ReadU16() uint16   { return binary.LittleEndian.Uint16(d.ReadRawBytes(2)) }
func (d *MemDecoder) ReadI16() int16    { return int16(d.ReadU16()) }
func (d *MemDecoder) ReadU32() uint32   { return uint32(d.readUnsigned(32).Lo) }
func (d *MemDecoder) ReadU64() uint64   { return d.readUnsigned(64).Lo }
func (d *MemDecoder) ReadUsize() uint   { return uint(d.readUnsigned(uint(strconv.IntSize)).Lo) }
func (d *MemDecoder) ReadU128() Uint128 { return d.readUnsigned(128) }
func (d *MemDecoder) ReadI32() int32    { return int32(d.readSigned(32).Lo) }
func (d *MemDecoder) ReadI64() int64    { return int64(d.readSigned(64).Lo) }
func (d *MemDecoder) ReadIsize() int    { return int(d.readSigned(uint(strconv.IntSize)).Lo) }
func (d *MemDecoder) ReadI128() Int128  { return d.readSigned(128) }
func (d *MemDecoder) shiftByte(b byte, shift, width uint) Uint128 {
	if d.profile.OverflowChecks && shift >= width {
		d.context.Panic("attempt to shift left with overflow")
	}
	shift %= width
	if shift >= 64 {
		return Uint128{Hi: uint64(b) << (shift - 64)}
	}
	out := Uint128{Lo: uint64(b) << shift}
	if shift > 57 {
		out.Hi = uint64(b) >> (64 - shift)
	}
	return out
}
func (d *MemDecoder) readUnsigned(width uint) Uint128 {
	b := d.ReadU8()
	if b&0x80 == 0 {
		return Uint128{Lo: uint64(b)}
	}
	out := Uint128{Lo: uint64(b & 0x7f)}
	shift := uint(7)
	for {
		b = d.ReadU8()
		part := d.shiftByte(b&0x7f, shift, width)
		out.Lo |= part.Lo
		out.Hi |= part.Hi
		if b&0x80 == 0 {
			return out
		}
		shift += 7
	}
}
func (d *MemDecoder) readSigned(width uint) Int128 {
	out := Int128{}
	shift := uint(0)
	for {
		b := d.ReadU8()
		part := d.shiftByte(b&0x7f, shift, width)
		out.Lo |= part.Lo
		out.Hi |= part.Hi
		shift += 7
		if b&0x80 == 0 {
			if shift < width && b&0x40 != 0 {
				if shift < 64 {
					out.Lo |= ^uint64(0) << shift
					out.Hi = ^uint64(0)
				} else {
					out.Hi |= ^uint64(0) << (shift - 64)
				}
			}
			return out
		}
	}
}
func (d *MemDecoder) ReadI8() int8        { return DefaultReadI8(d) }
func (d *MemDecoder) ReadBool() bool      { return DefaultReadBool(d) }
func (d *MemDecoder) ReadChar() rune      { return DefaultReadChar(d) }
func (d *MemDecoder) ReadStr() string     { return DefaultReadStr(d) }
func (d *MemDecoder) ReadByteStr() []byte { return DefaultReadByteStr(d) }
