// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: official rustc-stable-hash 0.1.2 src/lib.rs. See source-manifest.json.
// Package stablehash preserves rustc's buffered Sip1-3 and stable hashing
// adapters; it does not substitute Go's hash/maphash or another SipHash variant.
package stablehash

import (
	"encoding/binary"
	"strconv"
	"unicode/utf8"
)

// UInt128 and Int128 hold the two's-complement bits, low limb first.
type UInt128 [2]uint64
type Int128 [2]uint64

func Int128FromInt64(value int64) Int128 {
	high := uint64(0)
	if value < 0 {
		high = ^uint64(0)
	}
	return Int128{uint64(value), high}
}

// Hasher is the required source Hasher contract. Optional typed methods below
// retain std's individual default/override dispatch (including signed calls).
type Hasher interface {
	Write([]byte)
	Finish64() uint64
}

func HashU8(h Hasher, v uint8) {
	if x, ok := h.(interface{ WriteU8(uint8) }); ok {
		x.WriteU8(v)
	} else {
		h.Write([]byte{v})
	}
}
func HashU16(h Hasher, v uint16) {
	if x, ok := h.(interface{ WriteU16(uint16) }); ok {
		x.WriteU16(v)
	} else {
		var b [2]byte
		binary.NativeEndian.PutUint16(b[:], v)
		h.Write(b[:])
	}
}
func HashU32(h Hasher, v uint32) {
	if x, ok := h.(interface{ WriteU32(uint32) }); ok {
		x.WriteU32(v)
	} else {
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], v)
		h.Write(b[:])
	}
}
func HashU64(h Hasher, v uint64) {
	if x, ok := h.(interface{ WriteU64(uint64) }); ok {
		x.WriteU64(v)
	} else {
		var b [8]byte
		binary.NativeEndian.PutUint64(b[:], v)
		h.Write(b[:])
	}
}
func HashU128(h Hasher, v UInt128) {
	if x, ok := h.(interface{ WriteU128(UInt128) }); ok {
		x.WriteU128(v)
	} else {
		b := native128(v)
		h.Write(b[:])
	}
}
func HashUsize(h Hasher, v uint) {
	if x, ok := h.(interface{ WriteUsize(uint) }); ok {
		x.WriteUsize(v)
	} else if strconv.IntSize == 64 {
		var b [8]byte
		binary.NativeEndian.PutUint64(b[:], uint64(v))
		h.Write(b[:])
	} else {
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], uint32(v))
		h.Write(b[:])
	}
}
func HashI8(h Hasher, v int8) {
	if x, ok := h.(interface{ WriteI8(int8) }); ok {
		x.WriteI8(v)
	} else {
		HashU8(h, uint8(v))
	}
}
func HashI16(h Hasher, v int16) {
	if x, ok := h.(interface{ WriteI16(int16) }); ok {
		x.WriteI16(v)
	} else {
		HashU16(h, uint16(v))
	}
}
func HashI32(h Hasher, v int32) {
	if x, ok := h.(interface{ WriteI32(int32) }); ok {
		x.WriteI32(v)
	} else {
		HashU32(h, uint32(v))
	}
}
func HashI64(h Hasher, v int64) {
	if x, ok := h.(interface{ WriteI64(int64) }); ok {
		x.WriteI64(v)
	} else {
		HashU64(h, uint64(v))
	}
}
func HashI128(h Hasher, v Int128) {
	if x, ok := h.(interface{ WriteI128(Int128) }); ok {
		x.WriteI128(v)
	} else {
		HashU128(h, UInt128(v))
	}
}
func HashIsize(h Hasher, v int) {
	if x, ok := h.(interface{ WriteIsize(int) }); ok {
		x.WriteIsize(v)
	} else {
		HashUsize(h, uint(v))
	}
}
func HashString(h Hasher, s string) {
	if !utf8.ValidString(s) {
		panic("stablehash: Rust str requires valid UTF-8")
	}
	if x, ok := h.(interface{ WriteString(string) }); ok {
		x.WriteString(s)
	} else {
		h.Write([]byte(s))
		HashU8(h, 255)
	}
}
func HashLengthPrefix(h Hasher, n uint) {
	if x, ok := h.(interface{ WriteLengthPrefix(uint) }); ok {
		x.WriteLengthPrefix(n)
	} else {
		HashUsize(h, n)
	}
}
func native128(v UInt128) (b [16]byte) {
	var probe [2]byte
	binary.NativeEndian.PutUint16(probe[:], 1)
	if probe[0] == 1 {
		binary.NativeEndian.PutUint64(b[:8], v[0])
		binary.NativeEndian.PutUint64(b[8:], v[1])
	} else {
		binary.NativeEndian.PutUint64(b[:8], v[1])
		binary.NativeEndian.PutUint64(b[8:], v[0])
	}
	return
}
