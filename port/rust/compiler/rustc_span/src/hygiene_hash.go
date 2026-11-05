// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"

// HashWithHasher is the complete std::hash::Hash dictionary for this newtype.
// Hash's older typed-write operation adapter is retained for the partial Span
// foundation while its owner upgrades that surrounding public trait surface.
func (v SyntaxContext) HashWithHasher(hasher hashmap.Hasher) { hashmap.HashU32(hasher, v.value) }
