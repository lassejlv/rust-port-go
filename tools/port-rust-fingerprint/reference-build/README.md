`prepare.py` assembles an ignored standalone Cargo workspace from verified Git
blobs at `2d20b42b883dcfc014f3c2c56081eeb6091c2179`. It copies complete original
`rustc_serialize` and `rustc_hashes` crate sources and complete unchanged
`fingerprint.rs`, `fingerprint/tests.rs`, and `unhash.rs`. The latter's real
`indexmap` dependency makes all original aliases compile in the Rust reference;
the Go helper still ports only the Unhasher block.

The selected `stable_hash.rs` traits, context types, dummy-context panic
implementations, stable-compare blanket implementation, and trivial-type macro
are extracted without body edits. Each fragment has original line bounds and a
hash in the source provenance. The complete data-structures crate and the
remaining stable-hashing file are not being built or credited as ported.

Only standalone Cargo manifests, crate-root wiring, imports and feature gates
are adapted. The reference compiler is Rust1.99.1 while the pinned source
declares1.101.0. `RUSTC_BOOTSTRAP=1` enables the source's internal features;
`-Zcrate-attr=feature(never_type)` supplies a missing gate needed by actual
serialization code. `-Cdebug-assertions=true/false` selects the two source
profiles; the original fingerprint permutation test runs in both.

```sh
python3 tools/port-rust-fingerprint/reference-build/prepare.py --offline
python3 tools/port-rust-fingerprint/reference-build/prepare.py --offline --release-assertions
```

Every registry version/checksum must match the original pinned root Cargo.lock.
The original archives and every expanded source file, including Cargo-normalized
manifests, are checked before and after compilation. Symlink, nonregular,
missing, changed and unexpected files are rejected. The bounded verifier and
two standalone crate manifests are reused from the retained Edition reference
helper; their exact file hashes are recorded as helper dependencies.

`--offline` also disables Git lazy fetching. `--fetch` permits explicit locked
registry fetches, and first-time `--write-lock` records the adapted lockfile.
No production code invokes this build. Ignored build-profile metadata exposes
the actual Cargo-reported library and link arguments, without filename guesses.
