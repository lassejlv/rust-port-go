// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL foundation from compiler/rustc_span/src/hygiene.rs and its actual
// newtype_index! expansion at 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// Only SyntaxContext's concrete identity, checked conversions, and root blocks
// used by span_encoding are implemented. Expansion/mark allocation, hygiene
// resolution, macro-generated operators/codecs and the rest of this file are
// unported; no full-file completion is claimed.
package rustcspan

const SyntaxContextMaxAsU32 uint32 = 0x1fffffff

type SyntaxContext struct{ value uint32 }

func SyntaxContextFromU32(value uint32) SyntaxContext {
	if value > SyntaxContextMaxAsU32 {
		panic("SyntaxContext: value exceeds MAX")
	}
	return SyntaxContext{value}
}
func SyntaxContextFromUsize(value uint) SyntaxContext {
	if value > uint(SyntaxContextMaxAsU32) {
		panic("SyntaxContext: value exceeds MAX")
	}
	return SyntaxContext{uint32(value)}
}
func SyntaxContextFromU16(value uint16) SyntaxContext      { return SyntaxContextFromU32(uint32(value)) }
func RootSyntaxContext() SyntaxContext                     { return SyntaxContext{} }
func (value SyntaxContext) IsRoot() bool                   { return value.value == 0 }
func (value SyntaxContext) AsU32() uint32                  { return value.value }
func (value SyntaxContext) AsUsize() uint                  { return uint(value.value) }
func (value SyntaxContext) Index() uint                    { return value.AsUsize() }
func (value SyntaxContext) Clone() SyntaxContext           { return value }
func (value SyntaxContext) Equal(other SyntaxContext) bool { return value == other }
func (value SyntaxContext) Hash(h spanU32Hasher)           { h.WriteU32(value.value) }
