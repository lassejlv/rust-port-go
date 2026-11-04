//go:build 386

// SPDX-License-Identifier: MIT OR Apache-2.0
// Exact cfg(target_arch = "x86") declaration from sip128/tests.rs.
package stablehash_test

import "testing"

func TestSip128_test_hash_usize(t *testing.T) {
	val := uint64(0xdeadbeef_deadbeef)
	assert_true(t, sipHash(val) != sipHash(uint(val)))
	assert_eq(t, sipHash(uint32(val)), sipHash(uint(val)))
}
