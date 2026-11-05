// SPDX-License-Identifier: MIT OR Apache-2.0
// The public SyntaxContextKey alias and returned mark tuples retain their
// concrete tuple traits. They have no Ord/PartialOrd or Default implementation.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

func (k SyntaxContextKey) Clone() SyntaxContextKey              { return k }
func (k *SyntaxContextKey) CloneFrom(other SyntaxContextKey)    { *k = other }
func (k SyntaxContextKey) Equal(other SyntaxContextKey) bool    { return k == other }
func (k SyntaxContextKey) NotEqual(other SyntaxContextKey) bool { return !k.Equal(other) }
func (k SyntaxContextKey) Hash(hasher hashmap.Hasher) {
	k.Parent.HashWithHasher(hasher)
	k.Expansion.Hash(hasher)
	k.Transparency.Hash(hasher)
}
func (k SyntaxContextKey) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	return rustfmt.WriteTuple(f.Writer, "", pretty, []rustfmt.Value{f.Value(pretty, k.Parent.WriteDebugFormatter), f.Value(pretty, k.Expansion.WriteDebugFormatter), f.Value(pretty, k.Transparency.WriteDebugFormatter)})
}
func (k SyntaxContextKey) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	k.StableHashWithContext(session, hcx, h, rustfmt.Context{})
}
func (k SyntaxContextKey) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	k.Parent.StableHashWithContext(session, hcx, h, ctx)
	k.Expansion.StableHashWithContext(session, hcx, h, ctx)
	k.Transparency.StableHash(hcx, h)
}
func (k SyntaxContextKey) Encode(e SpanEncoder) {
	k.Parent.Encode(e)
	k.Expansion.Encode(e)
	k.Transparency.Encode(e)
}
func DecodeSyntaxContextKey(d SpanDecoder) SyntaxContextKey {
	return SyntaxContextKey{DecodeSyntaxContext(d), DecodeExpnId(d), DecodeTransparency(d)}
}
func (k HygieneMark) Clone() HygieneMark              { return k }
func (k *HygieneMark) CloneFrom(other HygieneMark)    { *k = other }
func (k HygieneMark) Equal(other HygieneMark) bool    { return k == other }
func (k HygieneMark) NotEqual(other HygieneMark) bool { return !k.Equal(other) }
func (k HygieneMark) Hash(hasher hashmap.Hasher) {
	k.Expansion.Hash(hasher)
	k.Transparency.Hash(hasher)
}
func (k HygieneMark) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	return rustfmt.WriteTuple(f.Writer, "", pretty, []rustfmt.Value{f.Value(pretty, k.Expansion.WriteDebugFormatter), f.Value(pretty, k.Transparency.WriteDebugFormatter)})
}
func (k HygieneMark) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	k.StableHashWithContext(session, hcx, h, rustfmt.Context{})
}
func (k HygieneMark) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	k.Expansion.StableHashWithContext(session, hcx, h, ctx)
	k.Transparency.StableHash(hcx, h)
}
func (k HygieneMark) Encode(e SpanEncoder) { k.Expansion.Encode(e); k.Transparency.Encode(e) }
func DecodeHygieneMark(d SpanDecoder) HygieneMark {
	return HygieneMark{DecodeExpnId(d), DecodeTransparency(d)}
}
