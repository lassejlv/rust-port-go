# Fingerprint decode panic metadata

This independent companion proof leaves the existing 55,292-row fingerprint
corpus unchanged. It links the complete unchanged pinned Fingerprint module
through the existing verified `fingerprint-reference` graph and its real
codec/hash dependencies. No source body is replaced.

The caller-defined safe Decoder returns a slice of length 0, 15, 16 or 17 from
`read_raw_bytes(16)`. Fingerprint and PackedFingerprint are each called in
ordinary, inherited-unwind and actual panic-hook contexts: 24 records and six
real native aborts. The proof records the original hook payload's dynamic Any
kind and exact bytes, can_unwind, force_no_backtrace, caller file/line/column,
thread panic state, read calls, and caller-owned Decoder destruction.

The observed failure is an OwnedString payload:

```
called `Result::unwrap()` on an `Err` value: TryFromSliceError(())
```

The original location is fingerprint.rs:180:68. `manifest.json` declares the
explicit portable reference filename profile; every hook also retains its raw
native filename. The oracle itself is compiled using a repository-relative
filename so that fresh checkouts reproduce the same records. Production does
not infer any filename: the Go comparison installs the named profile through
Context.PanicOperations.Locations, and a separate test verifies a missing
profile remains unlocated. Abort records contain events only, with no invented
post-abort state.

The source pin is `2d20b42b883dcfc014f3c2c56081eeb6091c2179`. This existing
reference uses development rustc 1.99.1, commit
`58cf8f9c35b355199727627d07ae34a4e2846d67`, to compile the unmodified pinned
source and dependencies. The manifest binds source provenance, locked registry
dependencies, helper sources, the production branch and the Go replay.

Run from the repository root:

```
python3 tools/port-rust-fingerprint/decode-metadata/generate.py --offline --check
go test ./port/rust/compiler/rustc_data_structures/src -run 'TestFingerprint(OriginalDecodeMetadata|DecodeMetadataWithoutLocation)$'
```

Use explicit `--rustc`/`--cargo` or FERRUGO_RUSTC/FERRUGO_CARGO to select tools.
Empty overrides and a different compiler are errors. Native generation reruns
source/dependency verification and the original source test before linking.
Gzip reuse requires bounded decompression, complete CRC validation and exact
newly observed bytes; corrupted or symlink fixtures fail check mode.
