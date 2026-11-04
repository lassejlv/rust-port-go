# Complete Hasher dispatch for hash wrappers

This adds no source-file credit. It closes the complete `std::hash::Hasher`
entrypoints for the already ported `Hash64`, `Hash128`, `Fingerprint` and
`PackedFingerprint` implementations and their inherited `Hash::hash_slice`
defaults. The old typed-only convenience APIs and large fixtures are unchanged.

`HashWithHasher` accepts the complete required `Write` + `Finish64` interface.
Individual `WriteU64` overrides dispatch independently; without one, the actual
standard-library default writes a native-endian eight-byte borrow. Hash does
not call finish, prepend a slice length, consume the state, or destroy it.

The explicit concrete-state API for Fingerprint is
`FingerprintHashWithHasherRef` (and the corresponding packed/slice functions).
It borrows the real `Ref.Value` cell using the validated stable-hash trait
dictionary, whose callbacks describe the genuine concrete Rust implementation.
This retains the source's exact `H=Unhasher` specialization. `H=*Unhasher`
represents the distinct source `&mut Unhasher` Hasher implementation and takes
the generic two-write path. Erasing a dictionary adapter into the plain Hasher
interface instead describes that adapter as a wrapper type; it cannot expose
the concrete-H specialization. Ordinary `*Unhasher` values retain their exact
dynamic identity through `HashWithHasher`.

`Ref.Context` supplies ambient unwind/panic-processing state at callback sites.
Translated panic sites call `Context.Panic` before local cleanup. Raw Go
callbacks retain Go-local `panic`/`defer` behavior; no wrapper can undo callback
effects that already ran or intercept a callback-local recovered Go panic.
Write payloads are temporary read-only source borrows. To retain their data a
callback must copy it. Hasher state is exclusively borrowed for the operation;
its temporary state-cell pointer cannot escape the source borrow lifetime.

The development oracle links the unchanged pinned complete `fingerprint.rs`,
`unhash.rs` and `rustc_hashes` source through the existing def-id reference
builder. Preparation restores copied local bodies from the verified pin; it
does not report a prior modified copy as a failure. After preparation, exact
local inventories/bytes are verified before and after Cargo, and injected local
modules/build inputs are rejected. The builder also verifies original Git blobs,
pinned registry archives and their expanded sources, and uses one coherent dependency artifact directory
per profile. Its other def-id support code receives no additional credit here.
The recorded compiler is Rust1.99.1 on Linux x86-64, compiling source revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` (source version1.101.0) with the
required development-only bootstrap features.

The 2,808 native cases, replayed through both Go API paths for 5,616 comparisons,
cover both debug-assertion profiles, exact versus
wrapper/reference-wrapper Unhasher, default raw writes, independent typed
overrides, seeded state, noncomparable concrete cells, empty and multi-element
inherited hash-slice defaults, callback order, state retention after caught
panics, and callback/caller destruction timing. All three source panic contexts
are exercised. The 380 native processing-panic aborts are each replayed through
both Go API paths (760 abort replays), retaining exact event prefixes;
exit134 represents the unrecoverable class, with no SIGABRT/core-file ABI claim.
Core dumps are disabled for these child processes. Existing 85,483 hash-wrapper
and 55,292 Fingerprint fixture records remain unchanged and are replayed by their
original tests.

```sh
python3 tools/port-rust-fingerprint/hasher-contract/generate.py --offline --check
go test -race ./port/rust/compiler/rustc_hashes/src ./port/rust/compiler/rustc_data_structures/src
```

The driver accepts `--cargo`, `--rustc`, `--go`, or `FERRUGO_CARGO`,
`FERRUGO_RUSTC`, `FERRUGO_GO`. Explicit invalid/empty overrides fail. Bare
executable names resolve through PATH. Without overrides it uses installed
workspace toolchains when present, then PATH. `CARGO_HOME` is honored;
`--fetch` forwards dependency fetching to the shared reference builder. Rust is
used only by this development evidence generator; production Go hashing does
not invoke Rust.
