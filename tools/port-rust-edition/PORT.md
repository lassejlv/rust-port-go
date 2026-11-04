# Complete rustc edition module port

`compiler/rustc_span/src/edition.rs` at Rust revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` maps to
`port/rust/compiler/rustc_span/src/edition.go` (`rustcspan`). Original Git blob:
`d4f3f0e208e34401cf6d02af5a8606a3980a0171`; complete source SHA-256:
`a1dd82ce1a5b22b27e3360bcf8bd606fd4fa6669f6de6305536d6436d63ac7ae`.
MIT OR Apache-2.0 license texts are retained under `port/rust/`.

The port includes all five ordered variants, both edition-name lists, default
and latest-stable editions, lint names, string conversion, stability and edition
predicates, exact `FromStr`, and every derived contract. `AllEditions` returns a
fresh ordered slice so a caller cannot modify the immutable original list.
`EditionFromStr` adapts `Result<Edition, ()>` to `(Edition, bool)`.

| Original contract | Go implementation |
| --- | --- |
| `Clone`, `Copy` | Value assignment, `Clone`, `CloneFrom` |
| `PartialEq`, `Eq`, `PartialOrd` | Native equality/ordering for valid variants and `Compare` |
| `Display` | `AsStr`, `String`, `DisplayTo`, formatter `%s`/`%v` |
| `Debug` | `DebugString`, `DebugTo`, formatter `%#v` |
| `Hash` | `Hash(EditionHasher)` calls the caller's typed `WriteIsize` once |
| `StableHash` | `StableHash(context, EditionStableHasher)` calls `WriteU8` once with syntactic variant order |
| `Encodable` | `Encode(EditionSpanEncoder)` calls `EmitU8` once |
| `BlobDecodable` | `DecodeEdition(EditionBlobDecoder)` reads one byte and rejects unknown tags with the original panic message |

Hashers and codecs retain caller-owned state. The required interfaces expose
the exact operations used by this enum, rather than replacing typed writes
with raw bytes or returning precomputed hash values. The opaque stable-hash
context is unused: the original primitive implementation and fieldless enum
never call it. `EditionSpanEncoder` and `EditionBlobDecoder` are operation
subsets for this type; they do not claim serialization of other span types.
Callback side effects, delayed errors owned by encoders, and panics propagate.

The concrete memory adapters implement the real edition byte protocol and
decoder footer/cursor behavior. `Finish` transfers encoder data without adding
the caller-owned `rust-end-file` footer. A decoder requires that footer, permits
an initial position at the end, reads exactly one byte per edition, and panics
on exhaustion. Its Go input snapshot enforces Rust's immutable-borrow behavior
without exposing a mutable alias. These scoped adapters grant no whole-file
credit to the surrounding serialization implementation.

The original `Display` forwards its string with `write!(f, "{s}")`, so outer
width, fill, alignment, sign and precision have no effect. The derived Debug
unit variants also ignore those options. Fifteen actual Rust formatting forms
verify this; the Go formatter preserves it and rejects unsupported verbs with
Go's bad-verb notation. `DisplayTo`/`DebugTo` propagate writer errors; a Go-only
short write without an error becomes `io.ErrShortWrite`. Go casts can construct
invalid discriminants that safe Rust cannot; methods reject those inputs.

## Original-code reference boundary

`reference-build/` compiles the complete, byte-verified original
`rustc_macros`, `rustc_serialize`, and `rustc_hashes` implementation crates,
plus the actual locked `rustc-stable-hash` algorithm. The complete edition file
is unchanged, including every derive. The full expanded result and exact
dependency/compiler provenance remain in the ignored reference cache and are
bound by hashes in the retained manifest.
Every compiled registry source member is checked against its checksum-verified
archive before compilation and after proof, including normalized manifests;
changed/missing/extra members and symlinks are rejected.

The surrounding `rustc_span` and `rustc_data_structures` packages are bounded
development facades. They retain the exact relevant original trait definitions,
dummy stable-hash context and primitive `u8` implementation. Only unused
span/symbol/definition-ID/fingerprint interfaces use opaque types and fail-fast
sentinels. They return no dummy values. Every successful and failing decode
path is exercised, and the unused-interface call counter must stay zero.
Actual codec and hashing operations always execute original implementations.
This validates Edition's complete used contract; it does not claim a complete
build or port of either facade crate, a full compiler runtime, or compiler ABI
compatibility. The facades and dependency builds receive no source-port credit.

Source revision 1.101.0 is validated with installed rustc 1.99.1 under the
recorded development-only `RUSTC_BOOTSTRAP=1` and
`-Zcrate-attr=feature(never_type)` profile. This enables an older compiler's
feature gate without changing any original implementation body. Production Go
code uses only the Go standard library and never invokes Rust or Cargo.
