// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_data_structures/src/svh.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179, blob
// 0b80c655d8a5aa83edeb91767e02867c1fd0c2ca.
package datastructures

import (
	"fmt"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Svh is a crate's strict version hash. As in the source, this is a Copy value;
// it adds no ordering implementation or hash-calculation algorithm.
type Svh struct{ hash Fingerprint }

func NewSvh(hash Fingerprint) Svh             { return Svh{hash: hash} }
func (value Svh) AsU128() rustchashes.Uint128 { return value.hash.asU128() }
func (value Svh) AsFingerprint() Fingerprint  { return value.hash }
func (value Svh) Clone() Svh                  { return value }
func (value *Svh) CloneFrom(other Svh)        { *value = other }
func (value Svh) Equal(other Svh) bool        { return value == other }
func (value Svh) NotEqual(other Svh) bool     { return !value.Equal(other) }
func (value Svh) ToHex() string {
	return fmt.Sprintf("%016x%016x", value.hash.second, value.hash.first)
}
func (value Svh) Hash(h stablehash.Hasher)           { value.hash.HashWithHasher(h) }
func (value Svh) HashWithHasher(h stablehash.Hasher) { value.Hash(h) }

// SvhHashWithHasherRef preserves the underlying Fingerprint specialization for
// the actual concrete H, including the distinction from a reference-wrapper H.
func SvhHashWithHasherRef[H, Output any](value Svh, h stablehash.HasherRef[H, Output]) {
	FingerprintHashWithHasherRef(value.hash, h)
}
func SvhHashSlice(values []Svh, h stablehash.Hasher) {
	for _, value := range values {
		value.Hash(h)
	}
}
func SvhHashSliceWithHasherRef[H, Output any](values []Svh, h stablehash.HasherRef[H, Output]) {
	for _, value := range values {
		SvhHashWithHasherRef(value, h)
	}
}
func (value Svh) StableHash(context StableHashCtxt, hasher *StableHasher) {
	value.hash.StableHash(context, hasher)
}
func (value Svh) Encode(encoder FingerprintEncoder) { value.hash.Encode(encoder) }
func DecodeSvh(decoder FingerprintDecoder) Svh {
	return DecodeSvhWithContext(rustfmt.Context{}, decoder)
}
func DecodeSvhWithContext(context rustfmt.Context, decoder FingerprintDecoder) Svh {
	return NewSvh(DecodeFingerprintWithContext(context, decoder))
}

func (value Svh) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "Svh", pretty)
	b.Field("hash", f.Value(pretty, func(child *rustfmt.Formatter) error { return value.hash.WriteDebugFormatter(child) }))
	return b.Finish()
}
func (value Svh) WriteDisplayFormatter(f *rustfmt.Formatter) error { return f.PadString(value.ToHex()) }
func (value Svh) DebugString() string                              { return rustfmt.String(value.WriteDebugFormatter) }
func (value Svh) String() string                                   { return rustfmt.String(value.WriteDisplayFormatter) }
