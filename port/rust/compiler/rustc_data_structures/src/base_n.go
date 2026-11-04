// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_data_structures/src/base_n.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.

// Package datastructures contains the ported rustc_data_structures utilities.
package datastructures

import "math/bits"

const (
	BaseNMaxBase          uint = 64
	BaseNAlphanumericOnly uint = 62
	BaseNCaseInsensitive  uint = 36
)

const baseNDigits = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@$"

// BaseNU128 represents an unsigned 128-bit integer without losing precision.
// Hi contains bits 127 through 64, and Lo contains bits 63 through 0.
type BaseNU128 struct {
	Hi uint64
	Lo uint64
}

// BaseNU32 and BaseNU64 adapt Go's built-in integers to BaseNValue. The
// width-specific free functions also accept ordinary uint32 and uint64 values.
type BaseNU32 uint32
type BaseNU64 uint64

// BaseNValue is the trivial-drop normal-context convenience for Rust's
// ToBaseN: Into<u128> extension point. BaseNTrait is the primary owned API.
// BaseNEncodedLen describes the maximum encoded width of the implementing type;
// as with Rust's associated function, it should not depend on the value.
//
// Implementations may optionally provide BaseNToBase(uint) BaseNString and/or
// BaseNToBaseFixedLen(uint) BaseNString to override the corresponding defaults.
// The public free functions dispatch these overrides, including a custom
// BaseNToBase called by the default fixed-length implementation.
type BaseNValue interface {
	BaseNInto128() BaseNU128
	BaseNEncodedLen(base uint) uint
}

func (n BaseNU32) BaseNInto128() BaseNU128   { return BaseNU128{Lo: uint64(n)} }
func (n BaseNU64) BaseNInto128() BaseNU128   { return BaseNU128{Lo: uint64(n)} }
func (n BaseNU128) BaseNInto128() BaseNU128  { return n }
func (BaseNU32) BaseNEncodedLen(b uint) uint { return BaseNEncodedLen32(b) }
func (BaseNU64) BaseNEncodedLen(b uint) uint { return BaseNEncodedLen64(b) }
func (BaseNU128) BaseNEncodedLen(b uint) uint {
	return BaseNEncodedLen128(b)
}

// BaseNString is the fixed-capacity representation returned by base-N encoding.
// Pointer-receiver Deref/AsRefBorrowed borrow this object's initialized ASCII
// suffix; WriteDisplayFormatter preserves the source Formatter contract.
// String/AsRef return independent strings for ordinary Go callers. Obtain values
// through encoding functions; Rust has no public zero-value constructor. Keep a
// borrowed object in place and do not overwrite it until all its borrows end.
type BaseNString struct {
	start uint
	buf   [128]byte
}

func (s BaseNString) String() string { return string(s.buf[s.start:]) }
func (s BaseNString) AsRef() string  { return string(s.buf[s.start:]) }

// BaseNToBase applies the ToBaseN default to an extensible value, unless its
// implementation overrides this operation.
func BaseNToBase(value BaseNValue, base uint) BaseNString {
	if custom, ok := value.(interface{ BaseNToBase(uint) BaseNString }); ok {
		return custom.BaseNToBase(base)
	}
	return BaseNToBase128(value.BaseNInto128(), base)
}

// BaseNToBaseFixedLen applies the ToBaseN fixed-length default. It encodes first,
// honoring a custom BaseNToBase, then queries the implementing type's width.
func BaseNToBaseFixedLen(value BaseNValue, base uint) BaseNString {
	if custom, ok := value.(interface{ BaseNToBaseFixedLen(uint) BaseNString }); ok {
		return custom.BaseNToBaseFixedLen(base)
	}
	encoded := BaseNToBase(value, base)
	return baseNWithLength(encoded, value.BaseNEncodedLen(base))
}

// BaseNToBase128 encodes n with the upstream digit alphabet. Bases 2 through 64
// are supported. As upstream, there is no preliminary base check: division by
// zero and a digit outside the alphabet panic where those operations occur.
func BaseNToBase128(n BaseNU128, base uint) BaseNString {
	var encoded BaseNString
	for i := range encoded.buf {
		encoded.buf[i] = '0'
	}
	index := len(encoded.buf)
	for {
		index--
		quotient, remainder := baseNDiv128(n, uint64(base))
		encoded.buf[index] = baseNDigits[remainder]
		n = quotient
		if n.Hi == 0 && n.Lo == 0 {
			break
		}
	}
	encoded.start = uint(index)
	return encoded
}

func BaseNToBase64(n uint64, base uint) BaseNString {
	return BaseNToBase128(BaseNU128{Lo: n}, base)
}

func BaseNToBase32(n uint32, base uint) BaseNString {
	return BaseNToBase128(BaseNU128{Lo: uint64(n)}, base)
}

// BaseNToBaseFixedLen128 pads to the encoded length of the input type's maximum
// value, preserving lexical numeric ordering for a fixed width in bases 2..36.
func BaseNToBaseFixedLen128(n BaseNU128, base uint) BaseNString {
	encoded := BaseNToBase128(n, base)
	return baseNWithLength(encoded, BaseNEncodedLen128(base))
}

func BaseNToBaseFixedLen64(n uint64, base uint) BaseNString {
	encoded := BaseNToBase64(n, base)
	return baseNWithLength(encoded, BaseNEncodedLen64(base))
}

func BaseNToBaseFixedLen32(n uint32, base uint) BaseNString {
	encoded := BaseNToBase32(n, base)
	return baseNWithLength(encoded, BaseNEncodedLen32(base))
}

// BaseNEncodedLen128 returns the digits needed to represent u128::MAX. As in
// Rust, base zero panics and base one cannot produce a successful length.
func BaseNEncodedLen128(base uint) uint {
	max := BaseNU128{Hi: ^uint64(0), Lo: ^uint64(0)}
	var length uint
	for max.Hi != 0 || max.Lo != 0 {
		length++
		max, _ = baseNDiv128(max, uint64(base))
	}
	return length
}

func BaseNEncodedLen64(base uint) uint {
	max := ^uint64(0)
	var length uint
	for max > 0 {
		length++
		max /= uint64(base)
	}
	return length
}

func BaseNEncodedLen32(base uint) uint {
	max := ^uint32(0)
	var length uint
	for max > 0 {
		length++
		max /= uint32(base)
	}
	return length
}

func baseNDiv128(n BaseNU128, divisor uint64) (BaseNU128, uint64) {
	hi := n.Hi / divisor
	lo, remainder := bits.Div64(n.Hi%divisor, n.Lo, divisor)
	return BaseNU128{Hi: hi, Lo: lo}, remainder
}

func baseNWithLength(encoded BaseNString, width uint) BaseNString {
	if width > uint(len(encoded.buf)) {
		// Rust's checked start-offset subtraction fails at the call itself,
		// before the returned value is dereferenced or displayed.
		panic("base_n: encoded length exceeds the 128-byte buffer")
	}
	encoded.start = uint(len(encoded.buf)) - width
	return encoded
}
