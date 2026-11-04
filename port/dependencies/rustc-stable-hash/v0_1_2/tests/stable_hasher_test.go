// SPDX-License-Identifier: MIT OR Apache-2.0
// Mirrors all declarations/assertions in src/stable_hasher/tests.rs.
package stablehash_test

import (
	"math"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
)

func TestStableHasher_test_hash_integers(t *testing.T) {
	test_u8 := uint8(0xAB)
	test_u16 := uint16(0xFFEE)
	test_u32 := uint32(0x445577AA)
	test_u64 := uint64(0x01234567_13243546)
	const test_u128 = 0x22114433_66557788_99AACCBB_EEDDFF77
	testU128 := stablehash.UInt128{uint64(test_u128 & low128Mask), uint64(test_u128 >> 64)}
	test_usize := uint(0xD0C0B0A0)
	test_i8 := int8(-100)
	test_i16 := int16(-200)
	test_i32 := int32(-300)
	test_i64 := int64(-400)
	test_i128 := stablehash.Int128FromInt64(-500)
	test_isize := int(-600)
	h := newStable()
	writeHashInput(h, test_u8)
	writeHashInput(h, test_u16)
	writeHashInput(h, test_u32)
	writeHashInput(h, test_u64)
	writeHashInput(h, testU128)
	writeHashInput(h, test_usize)
	writeHashInput(h, test_i8)
	writeHashInput(h, test_i16)
	writeHashInput(h, test_i32)
	writeHashInput(h, test_i64)
	writeHashInput(h, test_i128)
	writeHashInput(h, test_isize)
	expected := TestHash{stablehash.SipHasher128Hash{13997337031081104755, 6178945012502239489}}
	assert_eq(t, expected, TestHash{h.FinishHash()})
}
func TestStableHasher_test_hash_usize(t *testing.T) {
	test_usize := uint(0xABCDEF01)
	h := newStable()
	writeHashInput(h, test_usize)
	expected := TestHash{stablehash.SipHasher128Hash{12037165114281468837, 3094087741167521712}}
	assert_eq(t, expected, TestHash{h.FinishHash()})
}
func TestStableHasher_test_hash_isize(t *testing.T) {
	test_isize := int(-7)
	h := newStable()
	writeHashInput(h, test_isize)
	expected := TestHash{stablehash.SipHasher128Hash{3979067582695659080, 2322428596355037273}}
	assert_eq(t, expected, TestHash{h.FinishHash()})
}
func TestStableHasher_test_isize_compression(t *testing.T) {
	check_hash := func(a, b uint64) {
		hash_a := stableHash([2]int{int(a), int(b)})
		hash_b := stableHash([2]int{int(b), int(a)})
		assert_ne(t, hash_a, hash_b, "The hash stayed the same when permuting values \x60{a}\x60 and \x60{b}\x60!", a, b)
	}
	check_hash(0xAA, 0xAAAA)
	check_hash(0xFF, 0xFFFF)
	check_hash(0xAAAA, 0xAAAAAA)
	check_hash(0xAAAAAA, 0xAAAAAAAA)
	check_hash(0xFF, 0xFFFFFFFFFFFFFFFF)
	check_hash(math.MaxUint64 /* -1 */, 1)
}
func TestStableHasher_test_cloned_hasher_output(t *testing.T) {
	test_u8 := uint8(0xAB)
	test_u16 := uint16(0xFFEE)
	test_u32 := uint32(0x445577AA)
	h1 := newStable()
	writeHashInput(h1, test_u8)
	writeHashInput(h1, test_u16)
	h2 := h1.Clone()
	h3 := h1.Clone()
	writeHashInput(h3, test_u32)
	h1_hash := TestHash{h1.FinishHash()}
	assert_eq(t, h1_hash, TestHash{h2.FinishHash()})
	assert_ne(t, h1_hash, TestHash{h3.FinishHash()})
}
func TestStableHasher_test_hash_trait_finish(t *testing.T) {
	hash := func(h stablehash.Hasher) uint64 { return h.Finish64() }
	test_u8 := uint8(0xAB)
	test_u16 := uint16(0xFFEE)
	test_u32 := uint32(0x445577AA)
	h1 := newStable()
	writeHashInput(h1, test_u8)
	writeHashInput(h1, test_u16)
	writeHashInput(h1, test_u32)
	assert_eq(t, hash(h1), hash(h1))
	assert_eq(t, hash(h1), uint64(13655241286414701638))
}
