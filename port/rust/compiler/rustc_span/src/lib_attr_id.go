// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual newtype_index! identity used for AttrId in pinned rustc_span/lib.rs.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

const AttrIdMaxAsU32 uint32 = 0xffffff00

type AttrId struct{ value uint32 }

func NewAttrId(value uint) AttrId       { return AttrIdFromUsize(value) }
func AttrIdFromU32(value uint32) AttrId { return AttrIdFromU32WithContext(value, rustfmt.Context{}) }
func AttrIdFromU32WithContext(value uint32, context rustfmt.Context) AttrId {
	if value > AttrIdMaxAsU32 {
		spanStaticPanicAt(context, spanLibLocation(1370, 1), "assertion failed: value <= 0xFFFF_FF00")
	}
	return AttrId{value: value}
}
func AttrIdFromUsize(value uint) AttrId { return AttrIdFromUsizeWithContext(value, rustfmt.Context{}) }
func AttrIdFromUsizeWithContext(value uint, context rustfmt.Context) AttrId {
	if value > uint(AttrIdMaxAsU32) {
		spanStaticPanicAt(context, spanLibLocation(1370, 1), "assertion failed: value <= (0xFFFF_FF00 as usize)")
	}
	return AttrId{value: uint32(value)}
}
func AttrIdFromU16(value uint16) AttrId          { return AttrIdFromU32(uint32(value)) }
func AttrIdFromU32Unchecked(value uint32) AttrId { return AttrIdFromU32(value) }
func AttrIdZero() AttrId                         { return AttrId{} }
func AttrIdMax() AttrId                          { return AttrId{value: AttrIdMaxAsU32} }
func (value AttrId) AsU32() uint32               { return value.value }
func (value AttrId) AsUsize() uint               { return uint(value.value) }
func (value AttrId) Index() uint                 { return value.AsUsize() }
func (value AttrId) Clone() AttrId               { return value }
func (value *AttrId) CloneFrom(other AttrId)     { *value = other }
func (value AttrId) Equal(other AttrId) bool     { return value == other }
func (value AttrId) Compare(other AttrId) int    { return compareDefIndexValues(value.value, other.value) }
func (value AttrId) Hash(h spanU32Hasher)        { h.WriteU32(value.value) }
func (value AttrId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if err := f.WriteString("AttrId("); err != nil {
		return err
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	if err := rustfmt.FormatDebugUint(uint64(value.value), 32, child); err != nil {
		return err
	}
	return f.WriteString(")")
}
