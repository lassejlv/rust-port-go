// SPDX-License-Identifier: MIT OR Apache-2.0
// Full required/default std Hasher dictionaries and inherited hash_slice for
// the concrete source hygiene types that actually derive/implement Hash.
package rustcspan

import (
	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

func (v ExpnIndex) HashWithHasher(h stablehash.Hasher)            { v.Hash(h) }
func (v LocalExpnId) HashWithHasher(h stablehash.Hasher)          { v.Hash(h) }
func (v DecodedSyntaxContext) HashWithHasher(h stablehash.Hasher) { v.Hash(h) }
func (v ExpnId) HashWithHasher(h stablehash.Hasher)               { v.Hash(h) }
func (v ExpnHash) HashWithHasher(h stablehash.Hasher)             { v.Hash(h) }
func (v Transparency) HashWithHasher(h stablehash.Hasher)         { v.Hash(h) }
func (v MacroKind) HashWithHasher(h stablehash.Hasher)            { v.Hash(h) }
func (v SyntaxContextKey) HashWithHasher(h stablehash.Hasher)     { v.Hash(h) }
func (v HygieneMark) HashWithHasher(h stablehash.Hasher)          { v.Hash(h) }

// ExpnHashHashWithHasherRef is the concrete-H primary for the source's generic
// derive. Passing an erased dictionary to Hash cannot reveal whether the actual
// H was Unhasher or its distinct source &mut Unhasher reference wrapper.
func ExpnHashHashWithHasherRef[H, Output any](v ExpnHash, h stablehash.HasherRef[H, Output]) {
	datastructures.FingerprintHashWithHasherRef(v.fingerprint, h)
}

// HygieneHashSlice exposes Hash::hash_slice's inherited default for the concrete
// source Hash types. It hashes each element, without adding a length prefix.
// AstPass, DesugaringKind, ExpnKind and ExpnData do not implement source Hash.
func HygieneHashSlice[T interface{ HashWithHasher(stablehash.Hasher) }](values []T, h stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(h)
	}
}

func ExpnHashHashSliceWithHasherRef[H, Output any](values []ExpnHash, h stablehash.HasherRef[H, Output]) {
	for _, value := range values {
		ExpnHashHashWithHasherRef(value, h)
	}
}
