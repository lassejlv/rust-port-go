# Strict version hash

This ports all of `compiler/rustc_data_structures/src/svh.rs` at Rust
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` (Git blob
`0b80c655d8a5aa83edeb91767e02867c1fd0c2ca`) to `src/svh.go` in the mirrored
data-structures package. Rust's MIT OR Apache-2.0 notices remain in `port/rust`.
There are no original test declarations or child test files for this module.

`Svh` preserves the concrete Fingerprint field and source Copy, Clone/default
clone_from, PartialEq/default ne, Hash/default hash_slice, Debug, no-context
Encodable/Decodable and StableHash implementations. It adds no Ord or Default
trait and does not calculate crate hashes: that algorithm belongs elsewhere.

`NewSvh`, `AsU128`, `AsFingerprint` and `ToHex` correspond directly to the four
original methods. ToHex uses all 32 lowercase hexadecimal digits, with the high
limb first. Display calls the original string-padding protocol, including width,
precision, alignment, fill, writer failures and mutable formatter state. Debug
borrows a real child formatter and delegates to Fingerprint's tuple Debug.
The additive neutral Fingerprint formatter entrypoint retains the older numeric
API without changing its fixtures or behavior.

Hash borrows the caller's complete required Hasher. Independent typed overrides
and inherited raw-write defaults remain distinct. The concrete dictionary API
retains exact `H=Unhasher` specialization; a reference or distinct wrapper takes
the generic two-write path. Slice hashing performs the inherited element loop
without a length prefix. No state is copied, finished, consumed or dropped.

The codec adapters delegate exactly one 16-byte Fingerprint operation. Their
resolved method witnesses borrow the actual caller-owned encoder/decoder state;
byte inputs are temporary read-only borrows. StableHash uses the real stable
Sip hasher and Fingerprint's two-limb operation; its context is unused here.
These trait protocols compose the accepted complete Fingerprint contract and
introduce no owned callback argument, destructor or additional cleanup boundary.
Callback-local Go panic/defer timing remains host behavior; translated callback
panic sites use the active source context supplied by their operation witness.

This mapping grants one source-file record, not completion credit for the
surrounding compiler or standard library. Go storage and formatting buffers do
not claim a Rust ABI or custom allocator interface.
