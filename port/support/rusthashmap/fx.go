// SPDX-License-Identifier: MIT OR Apache-2.0
// Used hasher implementation from rustc-hash 2.1.1 src/lib.rs. The crate's
// unrelated random-state APIs are not included or given source-file credit.
package rusthashmap

import (
	"encoding/binary"
	"math/bits"
	"strconv"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
)

// These aliases share the complete std Hasher default/override adapters already
// implemented by the stable-hash dependency; they do not select its algorithm.
type Hasher = stablehash.Hasher
type UInt128 = stablehash.UInt128
type Int128 = stablehash.Int128

func HashU8(h Hasher, v uint8)          { stablehash.HashU8(h, v) }
func HashU16(h Hasher, v uint16)        { stablehash.HashU16(h, v) }
func HashU32(h Hasher, v uint32)        { stablehash.HashU32(h, v) }
func HashU64(h Hasher, v uint64)        { stablehash.HashU64(h, v) }
func HashU128(h Hasher, v UInt128)      { stablehash.HashU128(h, v) }
func HashUsize(h Hasher, v uint)        { stablehash.HashUsize(h, v) }
func HashI8(h Hasher, v int8)           { stablehash.HashI8(h, v) }
func HashI16(h Hasher, v int16)         { stablehash.HashI16(h, v) }
func HashI32(h Hasher, v int32)         { stablehash.HashI32(h, v) }
func HashI64(h Hasher, v int64)         { stablehash.HashI64(h, v) }
func HashI128(h Hasher, v Int128)       { stablehash.HashI128(h, v) }
func HashIsize(h Hasher, v int)         { stablehash.HashIsize(h, v) }
func HashString(h Hasher, v string)     { stablehash.HashString(h, v) }
func HashLengthPrefix(h Hasher, n uint) { stablehash.HashLengthPrefix(h, n) }

// FxHasher retains the source usize word profile. Its zero value is the normal
// host-width Default; NewFxHasherProfile also permits the 32-bit source profile
// on a 64-bit Go host. It is a plain Clone value with no destructor.
type FxHasher struct {
	state    uint64
	wordBits uint
}

func NewFxHasher() *FxHasher { return NewFxHasherProfile(uint(strconv.IntSize), 0) }
func NewFxHasherProfile(wordBits uint, seed uint64) *FxHasher {
	if wordBits != 32 && wordBits != 64 {
		panic("rusthashmap: invalid FxHasher word width")
	}
	h := &FxHasher{wordBits: wordBits}
	if wordBits == 32 {
		h.state = uint64(uint32(seed))
	} else {
		h.state = seed
	}
	return h
}

func (h *FxHasher) width() uint {
	if h.wordBits == 0 {
		return uint(strconv.IntSize)
	}
	return h.wordBits
}
func (h *FxHasher) Clone() *FxHasher           { value := *h; return &value }
func (h *FxHasher) CloneFrom(source *FxHasher) { *h = *source }
func (h *FxHasher) add(value uint64) {
	if h.width() == 32 {
		h.state = uint64((uint32(h.state) + uint32(value)) * uint32(0x93d765dd))
	} else {
		h.state = (h.state + value) * uint64(0xf1357aea2e62a9c5)
	}
}
func (h *FxHasher) WriteU8(value uint8)   { h.add(uint64(value)) }
func (h *FxHasher) WriteU16(value uint16) { h.add(uint64(value)) }
func (h *FxHasher) WriteU32(value uint32) { h.add(uint64(value)) }
func (h *FxHasher) WriteU64(value uint64) {
	h.add(value)
	if h.width() == 32 {
		h.add(value >> 32)
	}
}
func (h *FxHasher) WriteU128(value UInt128) {
	h.add(value[0])
	if h.width() == 32 {
		h.add(value[0] >> 32)
	}
	h.add(value[1])
	if h.width() == 32 {
		h.add(value[1] >> 32)
	}
}
func (h *FxHasher) WriteUsize(value uint)  { h.add(uint64(value)) }
func (h *FxHasher) WriteI8(value int8)     { h.WriteU8(uint8(value)) }
func (h *FxHasher) WriteI16(value int16)   { h.WriteU16(uint16(value)) }
func (h *FxHasher) WriteI32(value int32)   { h.WriteU32(uint32(value)) }
func (h *FxHasher) WriteI64(value int64)   { h.WriteU64(uint64(value)) }
func (h *FxHasher) WriteI128(value Int128) { h.WriteU128(UInt128(value)) }
func (h *FxHasher) WriteIsize(value int)   { h.WriteUsize(uint(value)) }
func (h *FxHasher) Finish64() uint64 {
	if h.width() == 32 {
		return uint64(bits.RotateLeft32(uint32(h.state), 15))
	}
	return bits.RotateLeft64(h.state, 26)
}

func (h *FxHasher) multiplyMix(x, y uint64) uint64 {
	if h.width() == 32 {
		a := uint64(uint32(x)) * uint64(uint32(y>>32))
		b := uint64(uint32(x>>32)) * uint64(uint32(y))
		return a ^ bits.RotateLeft64(b, -32)
	}
	hi, lo := bits.Mul64(x, y)
	return hi ^ lo
}

func (h *FxHasher) hashBytes(value []byte) uint64 {
	s0, s1 := uint64(0x243f6a8885a308d3), uint64(0x13198a2e03707344)
	n := len(value)
	if n <= 16 {
		switch {
		case n >= 8:
			s0 ^= binary.LittleEndian.Uint64(value[:8])
			s1 ^= binary.LittleEndian.Uint64(value[n-8:])
		case n >= 4:
			s0 ^= uint64(binary.LittleEndian.Uint32(value[:4]))
			s1 ^= uint64(binary.LittleEndian.Uint32(value[n-4:]))
		case n > 0:
			s0 ^= uint64(value[0])
			s1 ^= uint64(value[n-1])<<8 | uint64(value[n/2])
		}
	} else {
		for off := 0; off < n-16; off += 16 {
			x := binary.LittleEndian.Uint64(value[off : off+8])
			y := binary.LittleEndian.Uint64(value[off+8 : off+16])
			mix := h.multiplyMix(s0^x, 0xa4093822299f31d0^y)
			s0, s1 = s1, mix
		}
		s0 ^= binary.LittleEndian.Uint64(value[n-16 : n-8])
		s1 ^= binary.LittleEndian.Uint64(value[n-8:])
	}
	return h.multiplyMix(s0, s1) ^ uint64(n)
}
func (h *FxHasher) Write(value []byte) { h.WriteU64(h.hashBytes(value)) }

// The pinned rustc_data_structures dependency enables rustc-hash's std feature,
// not its nightly feature. WriteString and WriteLengthPrefix deliberately use
// the std defaults (terminator and usize prefix respectively).
func (h *FxHasher) WriteString(value string) { h.Write([]byte(value)); h.WriteU8(255) }
func (h *FxHasher) WriteLengthPrefix(n uint) { h.WriteUsize(n) }
