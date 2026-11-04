// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete std::hash::Hasher paths for fingerprint.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The legacy typed methods remain
// compatibility conveniences; these preserve required/default Hasher dispatch.
package datastructures

import stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"

// SetRuntimeContext supplies ambient source panic state. This host metadata is
// not part of the source Unhasher value or its Hash output. Translated source
// panic sites use this context before callback-local cleanup can run.
func (hasher *Unhasher) SetRuntimeContext(context FingerprintDropContext) {
	hasher.context = context
}

// HashWithHasher preserves the exact concrete Unhasher specialization. A
// distinct wrapper or reference-wrapper Hasher uses two independent write_u64
// dispatches. Missing typed overrides use Hasher's native-endian Write default.
// Hash does not call Finish64 or destroy the borrowed state.
//
// Use FingerprintHashWithHasherRef for an explicit concrete-type dictionary:
// interface erasure otherwise makes the dictionary adapter itself the Hasher
// type, rather than its underlying H. The dictionary primary also supports
// arbitrary non-Copy state and source panic context at actual callback sites.
func (value Fingerprint) HashWithHasher(hasher stablehash.Hasher) {
	if _, exact := hasher.(*Unhasher); exact {
		stablehash.HashU64(hasher, value.first+value.second)
		return
	}
	stablehash.HashU64(hasher, value.first)
	stablehash.HashU64(hasher, value.second)
}

// FingerprintHashWithHasherRef is the concrete-H primary for the source generic
// Hash operation. Ref.Value is the actual exclusive borrowed state cell. Its
// lifetime ends with this call; callbacks must obey the source borrow contract
// and the dictionary must represent H's genuine Hasher implementation.
//
// H=Unhasher selects the source specialization. H=*Unhasher represents the
// distinct source reference-wrapper implementation and uses the generic path.
// No state is copied, consumed or destroyed, and Ref.Context is retained.
func FingerprintHashWithHasherRef[H, Output any](value Fingerprint, hasher stablehash.HasherRef[H, Output]) {
	if _, exact := any(hasher.Ref.Value).(*Unhasher); exact {
		hasher.WriteU64(value.first + value.second)
		return
	}
	hasher.WriteU64(value.first)
	hasher.WriteU64(value.second)
}

func (value PackedFingerprint) HashWithHasher(hasher stablehash.Hasher) {
	value.Fingerprint().HashWithHasher(hasher)
}

func PackedFingerprintHashWithHasherRef[H, Output any](value PackedFingerprint, hasher stablehash.HasherRef[H, Output]) {
	FingerprintHashWithHasherRef(value.Fingerprint(), hasher)
}

// The Slice helpers expose the inherited Hash::hash_slice default, which hashes
// each element without a length prefix. An empty slice performs no callbacks.
func FingerprintSliceHashWithHasher(values []Fingerprint, hasher stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(hasher)
	}
}

func PackedFingerprintSliceHashWithHasher(values []PackedFingerprint, hasher stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(hasher)
	}
}

func FingerprintSliceHashWithHasherRef[H, Output any](values []Fingerprint, hasher stablehash.HasherRef[H, Output]) {
	for _, value := range values {
		FingerprintHashWithHasherRef(value, hasher)
	}
}

func PackedFingerprintSliceHashWithHasherRef[H, Output any](values []PackedFingerprint, hasher stablehash.HasherRef[H, Output]) {
	for _, value := range values {
		PackedFingerprintHashWithHasherRef(value, hasher)
	}
}
