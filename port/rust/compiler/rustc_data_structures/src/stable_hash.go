// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL foundation from rustc_data_structures/src/stable_hash.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. This file supplies the scoped
// RawSpan/RawDefId identities, concrete hasher alias and context bridge used by
// the span source ports. The surrounding stable-hashing file remains partial.
package datastructures

import (
	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// RawSpan is the source's u64 identity wrapper used to avoid a dependency on
// rustc_span. It supplies no hashing algorithm, codec, or Span implementation.
type RawSpan struct{ Value uint64 }

// RawDefId preserves the source tuple's order: crate number, then definition
// index. It avoids a dependency from data structures to rustc_span.
type RawDefId struct{ Crate, Index uint32 }

// StableHasher is the source's reexport of the actual locked stable Sip hasher.
// It is an instantiated (non-generic) alias, supported by Go 1.23.
type StableHasher = stablehash.StableSipHasher128

func NewStableHasher() *StableHasher { return stablehash.NewStableSipHasher128() }

// StableHashControls is the source context key for cached stable hashes.
type StableHashControls struct{ HashSpans bool }

func (value StableHashControls) Clone() StableHashControls              { return value }
func (value *StableHashControls) CloneFrom(source StableHashControls)   { *value = source.Clone() }
func (value StableHashControls) Equal(other StableHashControls) bool    { return value == other }
func (value StableHashControls) NotEqual(other StableHashControls) bool { return !value.Equal(other) }
func (value StableHashControls) Hash(hasher interface{ WriteU8(uint8) }) {
	if value.HashSpans {
		hasher.WriteU8(1)
	} else {
		hasher.WriteU8(0)
	}
}

// HashWithHasher retains bool Hash's independent write_u8 default/override
// dispatch through the complete required Hasher contract.
func (value StableHashControls) HashWithHasher(hasher stablehash.Hasher) {
	var bit uint8
	if value.HashSpans {
		bit = 1
	}
	stablehash.HashU8(hasher, bit)
}
func StableHashControlsHashSliceWithHasher(values []StableHashControls, hasher stablehash.Hasher) {
	for _, value := range values {
		value.HashWithHasher(hasher)
	}
}
func (value StableHashControls) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return rustfmt.WriteStruct(f.Writer, "StableHashControls", f.Options.Alternate, []rustfmt.Field{{Name: "hash_spans", Value: f.Value(f.Options.Alternate, func(field *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(value.HashSpans, field) })}})
}

// StableHashCtxt retains every required operation of the source trait. Context
// callbacks borrow the actual caller's state; implementations must preserve the
// shared/mutable source borrow and lifetime of that state. DefPathHash is a
// shared callback and may update only source interior-mutable fields.
type StableHashCtxt interface {
	StableHashSpan(RawSpan, *StableHasher)
	DefPathHash(RawDefId) Fingerprint
	StableHashControls() StableHashControls
	AssertDefaultStableHashControls(string)
}
