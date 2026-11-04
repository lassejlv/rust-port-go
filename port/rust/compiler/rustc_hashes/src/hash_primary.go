// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete std::hash::Hasher entrypoints for the Hash implementations in
// compiler/rustc_hashes/src/lib.rs. These supplement the typed compatibility
// helpers in lib.go; they do not introduce a different hashing algorithm.
package rustchashes

import stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"

// HashWithHasher accepts the complete required source Hasher contract. Its
// write_u64 override is dispatched independently; when omitted, the original
// default writes one native-endian eight-byte borrow through Write. Finish64
// is required by Hasher but is not called by Hash.
//
// A stablehash.HasherRef supplies arbitrary, non-Copy concrete state through
// its actual borrowed cell and carries the ambient panic context to callback
// sites. The callback dictionary must describe that concrete Rust impl. Raw Go
// callbacks retain Go-local panic timing; use the dictionary's Ref.Context.Panic
// at translated source panic sites. Hash neither copies nor destroys the state.
func (value Hash64) HashWithHasher(hasher stablehash.Hasher) {
	stablehash.HashU64(hasher, value.inner)
}

// HashWithHasher preserves Hash128's deliberate low-half-only source Hash.
func (value Hash128) HashWithHasher(hasher stablehash.Hasher) {
	stablehash.HashU64(hasher, value.Truncate().AsU64())
}

// Hash64SliceWithHasher is Hash::hash_slice's inherited default for these
// custom types. It hashes successive elements and writes no length prefix.
// The values and state are temporary borrows; neither is consumed or dropped.
func Hash64SliceWithHasher(values []Hash64, hasher stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(hasher)
	}
}

func Hash128SliceWithHasher(values []Hash128, hasher stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(hasher)
	}
}
