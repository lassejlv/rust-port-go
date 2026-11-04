// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete source test: compiler/rustc_data_structures/src/fingerprint/tests.rs.
// Git blob: e04af19abf2bd9a410f8b167680cd832d70dac1b.
package fingerprint_test

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"testing"
)

// Mirrors fingerprint/tests.rs in declaration order with all original values.
func TestCombineCommutativeIsOrderIndependent(t *testing.T) {
	a := datastructures.NewFingerprint(rustchashes.NewHash64(0xf6622fb349898b06), rustchashes.NewHash64(0x70be9377b2f9c610))
	b := datastructures.NewFingerprint(rustchashes.NewHash64(0xa9562bf5a2a5303c), rustchashes.NewHash64(0x67d9b6c82034f13d))
	c := datastructures.NewFingerprint(rustchashes.NewHash64(0x0d013a27811dbbc3), rustchashes.NewHash64(0x9a3f7b3d9142ec43))
	permutations := [][3]datastructures.Fingerprint{{a, b, c}, {a, c, b}, {b, a, c}, {b, c, a}, {c, a, b}, {c, b, a}}
	f := a.CombineCommutative(b).CombineCommutative(c)
	for _, p := range permutations {
		if f != p[0].CombineCommutative(p[1]).CombineCommutative(p[2]) {
			t.Fatal("original permutation assertion failed")
		}
	}
}
