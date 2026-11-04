// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL foundation from compiler/rustc_span/src/def_id.rs and the actual
// rustc_index_macros/src/newtype.rs expansion at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Implements the DefIndex identity,
// checked conversions, and LocalDefId identity used by span_encoding only.
// The rest of def_id.rs, macro operators/Step/codecs and its context-sensitive
// Debug implementation remain unported; this file earns no full-file credit.
package rustcspan

const DefIndexMaxAsU32 uint32 = 0xffffff00

type DefIndex struct{ value uint32 }

func DefIndexFromU32(value uint32) DefIndex {
	if value > DefIndexMaxAsU32 {
		panic("DefIndex: value exceeds MAX")
	}
	return DefIndex{value}
}
func DefIndexFromUsize(value uint) DefIndex {
	if value > uint(DefIndexMaxAsU32) {
		panic("DefIndex: value exceeds MAX")
	}
	return DefIndex{uint32(value)}
}
func DefIndexFromU16(value uint16) DefIndex      { return DefIndexFromU32(uint32(value)) }
func (value DefIndex) AsU32() uint32             { return value.value }
func (value DefIndex) AsUsize() uint             { return uint(value.value) }
func (value DefIndex) Index() uint               { return value.AsUsize() }
func (value DefIndex) Clone() DefIndex           { return value }
func (value DefIndex) Equal(other DefIndex) bool { return value == other }

// DefIndex is orderable; LocalDefId intentionally has no ordering methods.
func (value DefIndex) Compare(other DefIndex) int {
	if value.value < other.value {
		return -1
	}
	if value.value > other.value {
		return 1
	}
	return 0
}

type LocalDefId struct{ LocalDefIndex DefIndex }

func (value LocalDefId) Clone() LocalDefId           { return value }
func (value LocalDefId) Equal(other LocalDefId) bool { return value == other }

type spanU32Hasher interface{ WriteU32(uint32) }

func (value DefIndex) Hash(h spanU32Hasher)   { h.WriteU32(value.value) }
func (value LocalDefId) Hash(h spanU32Hasher) { value.LocalDefIndex.Hash(h) }
