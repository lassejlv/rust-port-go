// SPDX-License-Identifier: MIT OR Apache-2.0
// The three source doctests live in stable_hasher.rs. Examples 1 and 3 are
// interface/implementation compile witnesses; example2 executes a typed finish.
package stablehash_test

import (
	"math/bits"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
)

type docBogusHasher struct{ Value stablehash.UInt128 }

func docBogusWrite(ref stablehash.Ref[docBogusHasher], a []byte) {
	profile, _ := sourceProfile()
	sum := stablehash.UInt128{}
	for _, a := range a {
		low, carry := bits.Add64(sum[0], uint64(a), 0)
		high, overflow := bits.Add64(sum[1], 0, carry)
		if profile.OverflowChecks && overflow != 0 {
			ref.Context.Panic("attempt to add with overflow")
		}
		sum = stablehash.UInt128{low, high}
	}
	low, carry := bits.Add64(sum[0], ref.Value.Value[0], 0)
	high, overflow := bits.Add64(sum[1], ref.Value.Value[1], carry)
	if profile.OverflowChecks && overflow != 0 {
		ref.Context.Panic("attempt to add with overflow")
	}
	ref.Value.Value = stablehash.UInt128{low, high}
}
func docBogusOps() stablehash.ExtendedHasherOps[docBogusHasher, stablehash.UInt128] {
	return stablehash.ExtendedHasherOps[docBogusHasher, stablehash.UInt128]{
		Write:      docBogusWrite,
		Finish64:   func(ref stablehash.Ref[docBogusHasher]) uint64 { return ref.Value.Value[0] },
		ShortWrite: func(ref stablehash.Ref[docBogusHasher], length uint, bytes []byte) { docBogusWrite(ref, bytes) },
		Finish: func(owned *stablehash.Owned[docBogusHasher]) *stablehash.Owned[stablehash.UInt128] {
			state := owned.Take()
			return stablehash.Own(state.Value)
		},
	}
}
func TestDoc_ExtendedHasher(t *testing.T) {
	// The original example declares the implementation without instantiating it.
	_ = docBogusOps()
}

type docPairHash struct{ Value [2]uint64 }

func docPairFromStableHash() stablehash.FromStableHash[stablehash.SipHasher128Hash, docPairHash] {
	return stablehash.FromStableHash[stablehash.SipHasher128Hash, docPairHash]{
		From: func(hash *stablehash.Owned[stablehash.SipHasher128Hash]) *stablehash.Owned[docPairHash] {
			return stablehash.Own(docPairHash{[2]uint64(hash.Take())})
		},
	}
}
func TestDoc_StableHasher(t *testing.T) {
	hasher := newStable()
	hasher.WriteUsize(0xFA)
	hash := stablehash.FinishInto(hasher, docPairFromStableHash()).Take()
	_ = hash
}

type docWideHash struct{ Value stablehash.UInt128 }

func docWideFromStableHash() stablehash.FromStableHash[[2]uint64, docWideHash] {
	return stablehash.FromStableHash[[2]uint64, docWideHash]{
		From: func(owned *stablehash.Owned[[2]uint64]) *stablehash.Owned[docWideHash] {
			hash := owned.Take()
			upper := hash[0]
			lower := hash[1]
			// Source (upper << 64) | lower maps to these two explicit limbs.
			return stablehash.Own(docWideHash{stablehash.UInt128{lower, upper}})
		},
	}
}
func TestDoc_FromStableHash(t *testing.T) {
	// The original example declares the converter without calling it.
	_ = docWideFromStableHash()
}
