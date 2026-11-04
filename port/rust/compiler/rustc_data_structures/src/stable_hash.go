// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL foundation from rustc_data_structures/src/stable_hash.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Only the actual RawSpan identity
// block is implemented here. The surrounding stable-hashing file is unported.
package datastructures

// RawSpan is the source's u64 identity wrapper used to avoid a dependency on
// rustc_span. It supplies no hashing algorithm, codec, or Span implementation.
type RawSpan struct{ Value uint64 }
