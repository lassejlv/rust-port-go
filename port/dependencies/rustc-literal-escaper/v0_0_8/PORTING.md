# Pinned literal-escaper dependency

This ports `rustc-literal-escaper 0.0.8`, selected by the pinned Rust lockfile. The official crate archive SHA-256 is `bfe6f213fb658c8fb95baabd5420393438cf5a98d707f5dd701d9197c705f71e`; its published source revision is `ccd015ab0b388fb5e1c0c1085307aedd8210c7c8`.

`source-manifest.json` accounts for every one of the archive's 14 files, with original hashes, roles, targets and evidence. Its sole 687-line implementation maps to `src/lib.go`, the complete nine-test file maps to `tests/test_test.go`, and all 27 benchmark workloads map to `benches/benches_test.go`. Licenses and published metadata are retained verbatim. Adapted build metadata uses the parent Go module; no production code compiles or delegates to Rust.

Completion review found that derived Debug currently covers default text only,
and callback adapters do not yet expose owned closure destruction. The
implementation record is therefore partial until its full mutable formatter,
fallible output and owned callback contracts are completed. Existing escaping,
constructor, scalar and source-test comparisons remain valid evidence for those
implemented operations. See `docs/SOURCE-CONTRACT-AUDIT.md` at the repository root.

The escaping algorithms cover all 23 errors and their severity, all eight literal modes, raw checks, normal/byte/C-string unescaping, continuation warnings, exact UTF-8 byte ranges, nonzero unit restrictions, conversions and both public `MixedUnit` variants. `MixedByte` models the normalized `From<NonZero<u8>>` conversion; `MixedHighByte` models direct construction of the public HighByte variant even for ASCII values. Private `CheckRaw` and `Unescape` traits are specialized internally; the public callback adapters still need owned destruction behavior.

Go inputs follow Rust's valid-UTF-8 and scalar preconditions. Results use an explicit `NoError` value, and callers can use `Result.IsOK`. Callback ranges retain their byte offsets. The bootstrap string adapter normalizes CRLF before calling this component and maps diagnostic ranges back to the original bytes; nonfatal warnings currently have no bootstrap warning channel.

Validation compiles the complete unchanged original library and all nine original tests. All 121,536 retained original cases match callback values, errors, severity and ranges. Every one of the 1,112,064 Unicode scalars also matches across 16 direct, escaped and continuation operations: 302,482,110 reference event bytes, SHA-256 `3d37900f0331e0071d790a7335984975df4b15fb62403d1355822213b1a26263`. Direct variant construction, tags, payloads and equality have additional original-source checks. All 27 Go benchmark workloads run; no cross-runtime performance equivalence is claimed.

```sh
go test ./port/dependencies/rustc-literal-escaper/v0_0_8/...
FERRUGO_CARGO=cargo FERRUGO_RUSTC=rustc \
  go run ./tools/port-literal-escaper -check -offline
```

The live command requires the locked development dependencies cached first. Actual Go 1.23.12 and Go 1.27.3 tests, regeneration and vet pass. Existing valid compressed references are retained only after exact decompressed comparison, keeping checks reproducible across Go compression versions. These dependency records are separate from the two top-level upstream Git inventories and do not add completion credit to their source counts.
