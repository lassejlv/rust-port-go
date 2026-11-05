// SPDX-License-Identifier: MIT OR Apache-2.0
// Full required/default std Hasher entrypoints for def_id.rs and its actual
// newtype_index expansions. Existing typed methods remain conveniences.
package rustcspan

import (
	"strconv"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

// These primaries borrow the caller's complete Hasher. Typed defaults and
// independent overrides are dispatched by the validated trait bridge. Passing
// a HasherRef retains arbitrary actual state cells and their source context;
// no operation copies, finishes, consumes or destroys that state.
func (value CrateNum) HashWithHasher(h stablehash.Hasher) {
	stablehash.HashU32(h, value.AsU32())
}
func (value DefIndex) HashWithHasher(h stablehash.Hasher) {
	stablehash.HashU32(h, value.AsU32())
}
func (value DefId) HashWithHasher(h stablehash.Hasher) {
	if strconv.IntSize == 64 {
		stablehash.HashU64(h, uint64(value.Krate.AsU32())<<32|uint64(value.Index.AsU32()))
	} else {
		value.Index.HashWithHasher(h)
		value.Krate.HashWithHasher(h)
	}
}
func (value LocalDefId) HashWithHasher(h stablehash.Hasher) {
	value.LocalDefIndex.HashWithHasher(h)
}
func (value ModId) HashWithHasher(h stablehash.Hasher)         { value.defId.HashWithHasher(h) }
func (value LocalModId) HashWithHasher(h stablehash.Hasher)    { value.localDefId.HashWithHasher(h) }
func (value StableCrateId) HashWithHasher(h stablehash.Hasher) { value.hash.HashWithHasher(h) }
func (value DefPathHash) HashWithHasher(h stablehash.Hasher)   { value.Fingerprint.HashWithHasher(h) }

// DefPathHashHashWithHasherRef preserves Fingerprint's exact concrete-H
// specialization across this derived wrapper. The dictionary must describe
// H's genuine implementation and obey its temporary source borrow lifetimes.
func DefPathHashHashWithHasherRef[H, Output any](value DefPathHash, h stablehash.HasherRef[H, Output]) {
	datastructures.FingerprintHashWithHasherRef(value.Fingerprint, h)
}

func defIdHashSlice[T interface{ HashWithHasher(stablehash.Hasher) }](values []T, h stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(h)
	}
}

// The Slice helpers are the inherited Hash::hash_slice defaults. They borrow
// elements in order, write no prefix, and perform no callbacks for empty input.
func CrateNumHashSliceWithHasher(values []CrateNum, h stablehash.Hasher) { defIdHashSlice(values, h) }
func DefIndexHashSliceWithHasher(values []DefIndex, h stablehash.Hasher) { defIdHashSlice(values, h) }
func DefIdHashSliceWithHasher(values []DefId, h stablehash.Hasher)       { defIdHashSlice(values, h) }
func LocalDefIdHashSliceWithHasher(values []LocalDefId, h stablehash.Hasher) {
	defIdHashSlice(values, h)
}
func ModIdHashSliceWithHasher(values []ModId, h stablehash.Hasher) { defIdHashSlice(values, h) }
func LocalModIdHashSliceWithHasher(values []LocalModId, h stablehash.Hasher) {
	defIdHashSlice(values, h)
}
func StableCrateIdHashSliceWithHasher(values []StableCrateId, h stablehash.Hasher) {
	defIdHashSlice(values, h)
}
func DefPathHashHashSliceWithHasher(values []DefPathHash, h stablehash.Hasher) {
	defIdHashSlice(values, h)
}
func DefPathHashHashSliceWithHasherRef[H, Output any](values []DefPathHash, h stablehash.HasherRef[H, Output]) {
	for _, value := range values {
		DefPathHashHashWithHasherRef(value, h)
	}
}
