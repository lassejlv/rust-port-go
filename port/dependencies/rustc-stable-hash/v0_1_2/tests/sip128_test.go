// SPDX-License-Identifier: MIT OR Apache-2.0
// Mirrors src/sip128/tests.rs in source order; cfg-specific declarations are
// retained in sip128_arch_*_test.go at their corresponding source positions.
package stablehash_test

import (
	"strconv"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
)

func TestSip128_test_siphash_1_3_test_vector(t *testing.T) {
	k0 := uint64(0x_07_06_05_04_03_02_01_00)
	k1 := uint64(0x_0f_0e_0d_0c_0b_0a_09_08)
	input := []byte{}
	for i, v := range TEST_VECTOR {
		out := hash_with(newSip(k0, k1), Bytes(input[:]))
		expected := stablehash.SipHasher128Hash{
			uint64(v[0])<<0 | uint64(v[1])<<8 | uint64(v[2])<<16 | uint64(v[3])<<24 |
				uint64(v[4])<<32 | uint64(v[5])<<40 | uint64(v[6])<<48 | uint64(v[7])<<56,
			uint64(v[8])<<0 | uint64(v[9])<<8 | uint64(v[10])<<16 | uint64(v[11])<<24 |
				uint64(v[12])<<32 | uint64(v[13])<<40 | uint64(v[14])<<48 | uint64(v[15])<<56,
		}
		assert_eq(t, out, expected)
		input = append(input, uint8(i))
	}
}
func TestSip128_test_hash_idempotent(t *testing.T) {
	val64 := uint64(0xdeadbeef_deadbeef)
	assert_eq(t, sipHash(val64), sipHash(val64))
	val32 := uint32(0xdeadbeef)
	assert_eq(t, sipHash(val32), sipHash(val32))
}
func TestSip128_test_hash_no_bytes_dropped_64(t *testing.T) {
	val := uint64(0xdeadbeef_deadbeef)
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 0)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 1)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 2)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 3)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 4)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 5)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 6)))
	assert_true(t, sipHash(val) != sipHash(zero_byte64(t, val, 7)))
}
func zero_byte64(t *testing.T, val uint64, byte uint) uint64 {
	assert_true(t, byte < 8)
	return val & ^(uint64(0xff) << (byte * 8))
}
func TestSip128_test_hash_no_bytes_dropped_32(t *testing.T) {
	val := uint32(0xdeadbeef)
	assert_true(t, sipHash(val) != sipHash(zero_byte32(t, val, 0)))
	assert_true(t, sipHash(val) != sipHash(zero_byte32(t, val, 1)))
	assert_true(t, sipHash(val) != sipHash(zero_byte32(t, val, 2)))
	assert_true(t, sipHash(val) != sipHash(zero_byte32(t, val, 3)))
}
func zero_byte32(t *testing.T, val uint32, byte uint) uint32 {
	assert_true(t, byte < 4)
	return val & ^(uint32(0xff) << (byte * 8))
}
func TestSip128_test_hash_no_concat_alias(t *testing.T) {
	s := [2]string{"aa", "bb"}
	t0 := [2]string{"aabb", ""}
	u := [2]string{"a", "abb"}
	assert_true(t, s != t0 && t0 != u)
	assert_true(t, sipHash(s) != sipHash(t0) && sipHash(s) != sipHash(u))
	{
		u := [4]int32{1, 0, 0, 0}
		v := [3][]int32{u[:1], u[1:3], u[3:]}
		w := [3][]int32{u[:], u[4:4], u[4:4]}
		assert_true(t, !sliceTupleEqual(v, w))
		assert_true(t, sipHash(v) != sipHash(w))
	}
}
func TestSip128_test_short_write_works(t *testing.T) {
	test_u8 := uint8(0xFF)
	test_u16 := uint16(0x1122)
	test_u32 := uint32(0x22334455)
	test_u64 := uint64(0x33445566_778899AA)
	const test_u128 = 0x11223344_55667788_99AABBCC_DDEEFF77
	testU128 := stablehash.UInt128{uint64(test_u128 & low128Mask), uint64(test_u128 >> 64)}
	test_usize := uint(0xD0C0B0A0)
	test_i8 := int8(-1)
	test_i16 := int16(-2)
	test_i32 := int32(-3)
	test_i64 := int64(-4)
	test_i128 := stablehash.Int128FromInt64(-5)
	test_isize := int(-6)

	h1 := newSip(0, 0)
	h1.Write([]byte("bytes"))
	h1.Write([]byte("string"))
	h1.WriteU8(test_u8)
	h1.WriteU16(test_u16)
	h1.WriteU32(test_u32)
	h1.WriteU64(test_u64)
	h1.WriteU128(testU128)
	h1.WriteUsize(test_usize)
	h1.WriteI8(test_i8)
	h1.WriteI16(test_i16)
	h1.WriteI32(test_i32)
	h1.WriteI64(test_i64)
	h1.WriteI128(test_i128)
	h1.WriteIsize(test_isize)
	h2 := newSip(0, 0)
	h2.Write([]byte("bytes"))
	h2.Write([]byte("string"))
	h2.Write(nativeBytes(test_u8))
	h2.Write(nativeBytes(test_u16))
	h2.Write(nativeBytes(test_u32))
	h2.Write(nativeBytes(test_u64))
	h2.Write(nativeBytes(testU128))
	h2.Write(nativeBytes(test_usize))
	h2.Write(nativeBytes(test_i8))
	h2.Write(nativeBytes(test_i16))
	h2.Write(nativeBytes(test_i32))
	h2.Write(nativeBytes(test_i64))
	h2.Write(nativeBytes(test_i128))
	h2.Write(nativeBytes(test_isize))
	h1_hash := h1.Finish128()
	h2_hash := h2.Finish128()
	assert_eq(t, h1_hash, h2_hash)
}

// The source macro is mirrored once and invoked in exactly the same type order.
func test_fill_buffer(t *testing.T, size int, write func(*stablehash.SipHasher128, stablehash.UInt128), bytes func(stablehash.UInt128) []byte) {
	input := make([]byte, stablehash.BufferSize)
	for index := range input {
		input[index] = 42
	}
	const original_x = 0x01234567_89ABCDEF_76543210_FEDCBA98
	x := stablehash.UInt128{uint64(original_x & low128Mask), uint64(original_x >> 64)}
	x_bytes := bytes(x)
	for i := 1; i <= size; i++ {
		s := input[:stablehash.BufferSize-i]
		h1 := newSip(7, 13)
		h1.Write(s)
		write(h1, x)
		h2 := newSip(7, 13)
		h2.Write(s)
		h2.Write(x_bytes)
		h1_hash := h1.Finish128()
		h2_hash := h2.Finish128()
		assert_eq(t, h1_hash, h2_hash)
	}
}
func TestSip128_test_fill_buffer(t *testing.T) {
	test_fill_buffer(t, 1, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteU8(uint8(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(uint8(x[0])) })
	test_fill_buffer(t, 2, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteU16(uint16(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(uint16(x[0])) })
	test_fill_buffer(t, 4, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteU32(uint32(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(uint32(x[0])) })
	test_fill_buffer(t, 8, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteU64(x[0]) }, func(x stablehash.UInt128) []byte { return nativeBytes(x[0]) })
	test_fill_buffer(t, 16, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteU128(x) }, func(x stablehash.UInt128) []byte { return nativeBytes(x) })
	test_fill_buffer(t, strconv.IntSize/8, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteUsize(uint(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(uint(x[0])) })
	test_fill_buffer(t, 1, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteI8(int8(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(int8(x[0])) })
	test_fill_buffer(t, 2, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteI16(int16(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(int16(x[0])) })
	test_fill_buffer(t, 4, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteI32(int32(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(int32(x[0])) })
	test_fill_buffer(t, 8, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteI64(int64(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(int64(x[0])) })
	test_fill_buffer(t, 16, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteI128(stablehash.Int128(x)) }, func(x stablehash.UInt128) []byte { return nativeBytes(stablehash.Int128(x)) })
	test_fill_buffer(t, strconv.IntSize/8, func(h *stablehash.SipHasher128, x stablehash.UInt128) { h.WriteIsize(int(x[0])) }, func(x stablehash.UInt128) []byte { return nativeBytes(int(x[0])) })
}
func TestSip128_test_finish(t *testing.T) {
	hash := func(h *stablehash.SipHasher128) uint64 { return h.Finish64() }
	hasher := newSip(0, 0)
	hasher.WriteIsize(0xF0)
	hasher.WriteIsize(0xF0010)
	assert_eq(t, hash(hasher), hash(hasher))
}
