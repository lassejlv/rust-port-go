// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of rust-lang/rust compiler/rustc_hashes/src/lib.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// Original Git blob: c8f3057dac5fc26da8c243f95d8a978ee8cc00b7.
// Source SHA-256: ecb7777ffca8e89ad7056cc864771983a3eb4384049d739fbd5b562ed2d53850.
// License texts: ../../../LICENSE-MIT and LICENSE-APACHE.

// Package rustchashes contains the compiler's fixed-width hash value types.
package rustchashes

import "math/bits"

// Uint128 is the value-domain adapter for Rust u128. Lo holds bits 0..63.
// It has value semantics; it does not promise Rust's u128 alignment or ABI.
type Uint128 struct{ Lo, Hi uint64 }

// StableHasherHash is the two-limb representation supplied by the locked
// rustc-stable-hash dependency. It is a value, not a hashing implementation.
type StableHasherHash [2]uint64

// Hash64 stores an unsigned 64-bit hash. Its zero Go value is Rust Default/ZERO.
// Assignment and native equality implement Copy and derived Eq respectively.
type Hash64 struct{ inner uint64 }

func NewHash64(value uint64) Hash64      { return Hash64{inner: value} }
func ZeroHash64() Hash64                 { return Hash64{} }
func (value Hash64) AsU64() uint64       { return value.inner }
func (value Hash64) Clone() Hash64       { return value }
func (value *Hash64) CloneFrom(v Hash64) { *value = v }
func (value *Hash64) XorAssign(v uint64) { value.inner ^= v }
func (value Hash64) WrappingAdd(other Hash64) Hash64 {
	return Hash64{inner: value.inner + other.inner}
}

func Hash64FromStableHash(value StableHasherHash) Hash64 {
	return Hash64{inner: value[0]}
}

func (value Hash64) Compare(other Hash64) int {
	if value.inner < other.inner {
		return -1
	}
	if value.inner > other.inner {
		return 1
	}
	return 0
}

// U64Hasher is the precise typed Hasher operation used by both hash types.
// Caller-owned overrides, state and panics remain observable.
type U64Hasher interface{ WriteU64(uint64) }

func (value Hash64) Hash(hasher U64Hasher) { hasher.WriteU64(value.inner) }

// Hash128 stores an unsigned 128-bit hash. Its native Go equality preserves Eq;
// Compare implements unsigned u128 order, which compares the high limb first.
type Hash128 struct{ inner Uint128 }

func NewHash128(value Uint128) Hash128     { return Hash128{inner: value} }
func ZeroHash128() Hash128                 { return Hash128{} }
func (value Hash128) AsU128() Uint128      { return value.inner }
func (value Hash128) Truncate() Hash64     { return Hash64{inner: value.inner.Lo} }
func (value Hash128) Clone() Hash128       { return value }
func (value *Hash128) CloneFrom(v Hash128) { *value = v }

func (value Hash128) WrappingAdd(other Hash128) Hash128 {
	lo, carry := bits.Add64(value.inner.Lo, other.inner.Lo, 0)
	hi, _ := bits.Add64(value.inner.Hi, other.inner.Hi, carry)
	return Hash128{inner: Uint128{Lo: lo, Hi: hi}}
}

func Hash128FromStableHash(value StableHasherHash) Hash128 {
	return Hash128{inner: Uint128{Lo: value[0], Hi: value[1]}}
}

func (value Hash128) Compare(other Hash128) int {
	if value.inner.Hi < other.inner.Hi {
		return -1
	}
	if value.inner.Hi > other.inner.Hi {
		return 1
	}
	if value.inner.Lo < other.inner.Lo {
		return -1
	}
	if value.inner.Lo > other.inner.Lo {
		return 1
	}
	return 0
}

// Hash128 deliberately hashes only its low half, as in the original file.
func (value Hash128) Hash(hasher U64Hasher) { hasher.WriteU64(value.Truncate().AsU64()) }

// Formatter supplies the standard integer formatting operations delegated to by
// the source. NumericFormatter below implements the used std::fmt contract.
// Keeping the delegate explicit also permits fallible, stateful output sinks.
type Formatter interface {
	DebugU64(uint64) error
	DebugU128(Uint128) error
	LowerHexU64(uint64) error
	LowerHexU128(Uint128) error
}

func (value Hash64) WriteDebug(f Formatter) error     { return f.DebugU64(value.inner) }
func (value Hash64) WriteLowerHex(f Formatter) error  { return f.LowerHexU64(value.inner) }
func (value Hash128) WriteDebug(f Formatter) error    { return f.DebugU128(value.inner) }
func (value Hash128) WriteLowerHex(f Formatter) error { return f.LowerHexU128(value.inner) }
