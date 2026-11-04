// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: rustc-stable-hash 0.1.2 src/sip128.rs. The64-byte buffer,8-byte
// spill, batching, counter updates, and 1/3-round schedule follow the source.
package stablehash

import (
	"encoding/binary"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"math/bits"
	"strconv"
)

const elementSize = 8
const bufferCapacity = 8
const BufferSize = bufferCapacity * elementSize
const bufferWithSpillSize = BufferSize + elementSize

type SipHasher128Hash [2]uint64

func (v SipHasher128Hash) Clone() SipHasher128Hash            { return v }
func (v *SipHasher128Hash) CloneFrom(source SipHasher128Hash) { *v = source }
func (v SipHasher128Hash) Equal(other SipHasher128Hash) bool  { return v == other }

type sipState struct{ v0, v2, v1, v3 uint64 }

// sip13Rounds is the source private zero-state Sip13Rounds policy. Its Default
// is the Go zero value; the concrete one/three-round calls remain distinct.
type sip13Rounds struct{}

func (value sip13Rounds) clone() sip13Rounds              { return value }
func (value *sip13Rounds) cloneFrom(source sip13Rounds)   { *value = source.clone() }
func (sip13Rounds) writeDebug(f *rustfmt.Formatter) error { return f.WriteString("Sip13Rounds") }
func (sip13Rounds) cRounds(state *sipState)               { compress(state) }
func (sip13Rounds) dRounds(state *sipState)               { compress(state); compress(state); compress(state) }

type SipHasher128 struct {
	nbuf                  uint
	buf                   [bufferWithSpillSize]byte
	state                 sipState
	processed             uint
	profile               ArithmeticProfile
	context               rustfmt.Context
	initialized, consumed bool
}

func NewSipHasher128() *SipHasher128 { return NewSipHasher128WithKeys(0, 0) }
func NewSipHasher128WithKeys(k0, k1 uint64) *SipHasher128 {
	return NewSipHasher128WithProfile(k0, k1, DefaultProfile())
}
func NewSipHasher128WithProfile(k0, k1 uint64, profile ArithmeticProfile) *SipHasher128 {
	return &SipHasher128{state: sipState{v0: k0 ^ 0x736f6d6570736575, v1: k1 ^ (0x646f72616e646f6d ^ 0xee), v2: k0 ^ 0x6c7967656e657261, v3: k1 ^ 0x7465646279746573}, profile: profile, initialized: true}
}
func (h *SipHasher128) checkLive() {
	if h.consumed {
		h.context.Panic("stablehash: use of consumed SipHasher128")
	}
	if !h.initialized {
		context := h.context
		*h = *NewSipHasher128()
		h.context = context
	}
}
func (h *SipHasher128) SetContext(context rustfmt.Context) { h.context = context }
func (h *SipHasher128) Clone() *SipHasher128               { h.checkLive(); clone := *h; return &clone }
func (h *SipHasher128) CloneFrom(source *SipHasher128) {
	source.checkLive()
	h.checkLive()
	context := h.context
	*h = *source
	// Panic context is caller ambient state, not a field of the Rust hasher.
	h.context = context
}
func compress(s *sipState) {
	s.v0 += s.v1
	s.v2 += s.v3
	s.v1 = bits.RotateLeft64(s.v1, 13)
	s.v1 ^= s.v0
	s.v3 = bits.RotateLeft64(s.v3, 16)
	s.v3 ^= s.v2
	s.v0 = bits.RotateLeft64(s.v0, 32)
	s.v2 += s.v1
	s.v0 += s.v3
	s.v1 = bits.RotateLeft64(s.v1, 17)
	s.v1 ^= s.v2
	s.v3 = bits.RotateLeft64(s.v3, 21)
	s.v3 ^= s.v0
	s.v2 = bits.RotateLeft64(s.v2, 32)
}
func processWord(state *sipState, word uint64) {
	state.v3 ^= word
	(sip13Rounds{}).cRounds(state)
	state.v0 ^= word
}
func (h *SipHasher128) add(a, b uint) uint { return debugStrictAdd(a, b, h.profile, h.context) }
func (h *SipHasher128) sub(a, b uint) uint { return debugStrictSub(a, b, h.profile, h.context) }
func (h *SipHasher128) assert(condition bool, message string) {
	if h.profile.DebugAssertions && !condition {
		h.context.Panic(message)
	}
}

// ShortWrite carries the source const LEN independently of the array payload.
// Go-only length mismatches always fail. Debug source assertions and defined
// release calls above8 bytes are retained; Rust out-of-bounds pointer domains
// fail explicitly rather than using Go copy's silent truncation.
// copySmall retains the source fixed-width 4/2/1-byte copying path.
func copySmall(source, destination []byte, count uint, profile ArithmeticProfile, context rustfmt.Context) {
	if profile.DebugAssertions && count > 8 {
		context.Panic("assertion failed: count <= 8")
	}
	if count == 8 {
		copy(destination[:8], source[:8])
		return
	}
	i := uint(0)
	if debugStrictAdd(i, 3, profile, context) < count {
		copy(destination[i:i+4], source[i:i+4])
		i = debugStrictAdd(i, 4, profile, context)
	}
	if debugStrictAdd(i, 1, profile, context) < count {
		copy(destination[i:i+2], source[i:i+2])
		i = debugStrictAdd(i, 2, profile, context)
	}
	if i < count {
		destination[i] = source[i]
		i = debugStrictAdd(i, 1, profile, context)
	}
	if profile.DebugAssertions && i != count {
		context.Panic("assertion failed: i == count")
	}
}

func (h *SipHasher128) ShortWrite(length uint, bytes []byte) {
	h.checkLive()
	if uint(len(bytes)) != length {
		h.context.Panic("stablehash: const LEN differs from array length")
	}
	nbuf := h.nbuf
	h.assert(length <= 8, "assertion failed: LEN <= 8")
	h.assert(nbuf < BufferSize, "assertion failed: nbuf < BUFFER_SIZE")
	h.assert(nbuf+length < bufferWithSpillSize, "assertion failed: nbuf + LEN < BUFFER_WITH_SPILL_SIZE")
	if h.add(nbuf, length) < BufferSize {
		copy(h.buf[nbuf:nbuf+length], bytes)
		h.nbuf = h.add(nbuf, length)
		return
	}
	h.shortWriteProcessBuffer(length, bytes)
}
func (h *SipHasher128) shortWriteProcessBuffer(length uint, bytes []byte) {
	nbuf := h.nbuf
	h.assert(length <= 8, "assertion failed: LEN <= 8")
	h.assert(nbuf < BufferSize, "assertion failed: nbuf < BUFFER_SIZE")
	h.assert(nbuf+length >= BufferSize, "assertion failed: nbuf + LEN >= BUFFER_SIZE")
	h.assert(nbuf+length < bufferWithSpillSize, "assertion failed: nbuf + LEN < BUFFER_WITH_SPILL_SIZE")
	if length == 0 || nbuf+length > bufferWithSpillSize || length-1 > elementSize {
		h.context.Panic("stablehash: short_write would enter undefined Rust pointer-copy domain")
	}
	copy(h.buf[nbuf:nbuf+length], bytes)
	for i := 0; i < bufferCapacity; i++ {
		processWord(&h.state, binary.LittleEndian.Uint64(h.buf[i*elementSize:]))
	}
	copy(h.buf[:length-1], h.buf[BufferSize:BufferSize+length-1])
	if length == 1 {
		h.nbuf = 0
	} else {
		h.nbuf = h.sub(h.add(nbuf, length), BufferSize)
	}
	h.processed = h.add(h.processed, BufferSize)
}
func (h *SipHasher128) Write(message []byte) {
	h.checkLive()
	length, nbuf := uint(len(message)), h.nbuf
	h.assert(nbuf < BufferSize, "assertion failed: nbuf < BUFFER_SIZE")
	if h.add(nbuf, length) < BufferSize {
		if length <= 8 {
			copySmall(message, h.buf[nbuf:], length, h.profile, h.context)
		} else {
			copy(h.buf[nbuf:nbuf+length], message)
		}
		h.nbuf = h.add(nbuf, length)
		return
	}
	h.sliceWriteProcessBuffer(message)
}
func (h *SipHasher128) sliceWriteProcessBuffer(message []byte) {
	length, nbuf := uint(len(message)), h.nbuf
	h.assert(nbuf < BufferSize, "assertion failed: nbuf < BUFFER_SIZE")
	h.assert(nbuf+length >= BufferSize, "assertion failed: nbuf + length >= BUFFER_SIZE")
	needed := h.sub(elementSize, nbuf%elementSize)
	copySmall(message, h.buf[nbuf:], needed, h.profile, h.context)
	last := h.add(nbuf/elementSize, 1)
	for i := uint(0); i < last; i++ {
		processWord(&h.state, binary.LittleEndian.Uint64(h.buf[i*elementSize:]))
	}
	processed := needed
	inputLeft := h.sub(length, processed)
	elementsLeft, extra := inputLeft/elementSize, inputLeft%elementSize
	for i := uint(0); i < elementsLeft; i++ {
		processWord(&h.state, binary.LittleEndian.Uint64(message[processed:]))
		processed = h.add(processed, elementSize)
	}
	copySmall(message[processed:], h.buf[:], extra, h.profile, h.context)
	h.nbuf = extra
	h.processed = h.add(h.processed, h.add(nbuf, processed))
}
func finish128Inner(nbuf uint, buffer *[bufferWithSpillSize]byte, state sipState, processed uint, profile ArithmeticProfile, context rustfmt.Context) SipHasher128Hash {
	if profile.DebugAssertions && nbuf >= BufferSize {
		context.Panic("assertion failed: nbuf < BUFFER_SIZE")
	}
	last := nbuf / elementSize
	for i := uint(0); i < last; i++ {
		processWord(&state, binary.LittleEndian.Uint64(buffer[i*elementSize:]))
	}
	var element uint64
	if nbuf%elementSize != 0 {
		clear(buffer[nbuf : nbuf+elementSize-1])
		element = binary.LittleEndian.Uint64(buffer[last*elementSize:])
	}
	length := debugStrictAdd(processed, nbuf, profile, context)
	word := ((uint64(length) & 0xff) << 56) | element
	processWord(&state, word)
	state.v2 ^= 0xee
	(sip13Rounds{}).dRounds(&state)
	lo := state.v0 ^ state.v1 ^ state.v2 ^ state.v3
	state.v1 ^= 0xdd
	(sip13Rounds{}).dRounds(&state)
	hi := state.v0 ^ state.v1 ^ state.v2 ^ state.v3
	return SipHasher128Hash{lo, hi}
}
func (h *SipHasher128) Finish128() SipHasher128Hash {
	h.checkLive()
	h.consumed = true
	return finish128Inner(h.nbuf, &h.buf, h.state, h.processed, h.profile, h.context)
}
func (h *SipHasher128) Finish64() uint64 {
	h.checkLive()
	buffer := h.buf
	sum := finish128Inner(h.nbuf, &buffer, h.state, h.processed, h.profile, h.context)
	return sum[0]*3 + sum[1]
}
func (h *SipHasher128) WriteU8(v uint8) { h.ShortWrite(1, []byte{v}) }
func (h *SipHasher128) WriteU16(v uint16) {
	var b [2]byte
	binary.NativeEndian.PutUint16(b[:], v)
	h.ShortWrite(2, b[:])
}
func (h *SipHasher128) WriteU32(v uint32) {
	var b [4]byte
	binary.NativeEndian.PutUint32(b[:], v)
	h.ShortWrite(4, b[:])
}
func (h *SipHasher128) WriteU64(v uint64) {
	var b [8]byte
	binary.NativeEndian.PutUint64(b[:], v)
	h.ShortWrite(8, b[:])
}
func (h *SipHasher128) WriteU128(v UInt128) { b := native128(v); h.Write(b[:]) }
func (h *SipHasher128) WriteUsize(v uint) {
	if strconv.IntSize == 64 {
		h.WriteU64(uint64(v))
	} else {
		h.WriteU32(uint32(v))
	}
}
func (h *SipHasher128) WriteI8(v int8)     { h.ShortWrite(1, []byte{uint8(v)}) }
func (h *SipHasher128) WriteI16(v int16)   { h.WriteU16(uint16(v)) }
func (h *SipHasher128) WriteI32(v int32)   { h.WriteU32(uint32(v)) }
func (h *SipHasher128) WriteI64(v int64)   { h.WriteU64(uint64(v)) }
func (h *SipHasher128) WriteI128(v Int128) { h.WriteU128(UInt128(v)) }
func (h *SipHasher128) WriteIsize(v int)   { h.WriteUsize(uint(v)) }
func (v SipHasher128Hash) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return rustfmt.WriteTuple(f.Writer, "SipHasher128Hash", f.Options.Alternate, []rustfmt.Value{f.Value(f.Options.Alternate, func(field *rustfmt.Formatter) error { return writeHashArray(v, field) })})
}
func writeHashArray(v SipHasher128Hash, f *rustfmt.Formatter) error {
	values := make([]rustfmt.Value, 2)
	for i, n := range v {
		values[i] = f.Value(f.Options.Alternate, func(field *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(n, 64, field) })
	}
	return rustfmt.WriteList(f.Writer, f.Options.Alternate, values)
}
func (h *SipHasher128) WriteDebugFormatter(f *rustfmt.Formatter) error {
	h.checkLive()
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "SipHasher128", pretty, []rustfmt.Field{
		{Name: "nbuf", Value: f.Value(pretty, func(field *rustfmt.Formatter) error {
			return rustfmt.FormatDebugUint(uint64(h.nbuf), uint(strconv.IntSize), field)
		})},
		{Name: "buf", Value: f.Value(pretty, func(field *rustfmt.Formatter) error {
			values := make([]rustfmt.Value, 9)
			for i := range values {
				values[i] = field.Value(field.Options.Alternate, func(child *rustfmt.Formatter) error { return child.PadString("MaybeUninit<u64>") })
			}
			return rustfmt.WriteList(field.Writer, field.Options.Alternate, values)
		})},
		{Name: "state", Value: f.Value(pretty, h.state.writeDebug)},
		{Name: "processed", Value: f.Value(pretty, func(field *rustfmt.Formatter) error {
			return rustfmt.FormatDebugUint(uint64(h.processed), uint(strconv.IntSize), field)
		})},
	})
}
func (s sipState) writeDebug(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	fields := []rustfmt.Field{}
	for _, field := range []struct {
		name  string
		value uint64
	}{{"v0", s.v0}, {"v2", s.v2}, {"v1", s.v1}, {"v3", s.v3}} {
		fields = append(fields, rustfmt.Field{Name: field.name, Value: f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(field.value, 64, f) })})
	}
	return rustfmt.WriteStruct(f.Writer, "State", pretty, fields)
}
func (h *SipHasher128) RustDebug() string    { return rustfmt.String(h.WriteDebugFormatter) }
func (v SipHasher128Hash) RustDebug() string { return rustfmt.String(v.WriteDebugFormatter) }
