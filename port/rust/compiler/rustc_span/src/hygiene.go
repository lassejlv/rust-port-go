// SPDX-License-Identifier: MIT OR Apache-2.0
// Translation of compiler/rustc_span/src/hygiene.rs and its actual
// newtype_index! expansion at 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// The source's concrete types, allocation, resolution, remapping, codecs,
// hashing and lifecycle implementations are split across the hygiene*.go files.
// Original whole-source differential validation is retained under
// tools/port-rust-hygiene; full-file acceptance is tracked independently.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

const SyntaxContextMaxAsU32 uint32 = 0x1fffffff

type SyntaxContext struct{ value uint32 }

func SyntaxContextFromU32(value uint32) SyntaxContext {
	return SyntaxContextFromU32WithContext(value, rustfmt.Context{})
}
func SyntaxContextFromUsize(value uint) SyntaxContext {
	return SyntaxContextFromUsizeWithContext(value, rustfmt.Context{})
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
