# Partial BLAKE3 dependency adapter

This supports only the unkeyed `Hasher::new`, `Hasher::update`, borrowed
`Hasher::finalize` and `hash` operations used by rustc_span source-file hashing.
It is explicitly partial and grants no complete source-file or crate credit.
The source manifest inventories every official blake3 1.8.2 archive member and
records the remaining implementation/test/benchmark files as unported.

The pinned archive SHA-256 is
`3888aaa89e4b2a40fca9848e400f6a658a5a3978de7be858e209cafa8be9a4a0`,
from Rust revision `2d20b42b883dcfc014f3c2c56081eeb6091c2179`'s lockfile.
Original license texts and source metadata are retained verbatim.

`src/hash.go` translates portable.rs's wrapping 32-bit compression and the
lib.rs chunk/output/tree contracts into scalar Go. It processes chunks
serially and merges completed nodes before accepting the next chunk. This
replaces the original optimized subtree/SIMD scheduling; internal lazy-stack
layout and throughput are outside this scoped adapter. The 64-byte final block
and root flag remain distinct from ordinary chaining values. Finalize leaves
the hasher reusable; Update borrows input only during the call. Go's zero value
also initializes to the unkeyed state as a convenience. There is no Rust
fallback or production dependency on source files, fixtures or network access.

Keyed hashing, key derivation, extended output, arbitrary input offsets,
platform acceleration, IO traits, diagnostics and optional crate features
remain unported. The fixed stack covers the source's tree-depth domain; no
Rust ABI or allocator behavior is promised. Callers synchronize mutation.

The independent reference compiles the actual checksum-verified crate with its
`pure` feature. Every selected dependency version/checksum must match the Rust
lockfile, and all expanded registry files are verified before and after native
execution. No hash algorithm is substituted in that reference. Native records
cover block/chunk/tree boundaries through 2 MiB, different data bytes and update
partitions, empty updates, prefix finalization, repeated finalization and later
updates. Go replay also changes input bytes after each borrow expires.

Run `python3 tools/port-blake3-scalar/generate.py --fetch` for an explicit cold
fetch and first generation, or `--check` to compare offline without changing
retained observations. `FERRUGO_RUSTC`, `FERRUGO_CARGO`, `CARGO_HOME` and
`FERRUGO_RUST_SOURCE` select the recorded reference toolchain and caches.
Normal Go tests require none of those tools or upstream sources.
