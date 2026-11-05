# Pinned source-file analyzer

This maps the complete 298-line `compiler/rustc_span/src/analyze_source_file.rs`
at Rust `2d20b42b883dcfc014f3c2c56081eeb6091c2179` to
`port/rust/compiler/rustc_span/src/analyze_source_file.go`. The twelve declarations
in its 109-line test module map to `src/analyze_source_file/tests_test.go`.

The public Go entry points are `AnalyzeSourceFile`, `AnalyzeSourceFileWithConfig`
and `AnalyzeSourceFileWithContext`. They require valid UTF-8, as Rust `&str`
does. Configuration independently selects source debug assertions, ordinary
integer overflow checks and the source host's architecture/CPU-feature witness.
The metadata uses the actual shared `RelativeBytePos` and `MultiByteChar` types.

The generic decoder, SIMD chunk/tail/fallback cursors, all sixteen mask bits and
cross-chunk UTF-8 overflow are retained. SSE2 and LSX vector operations are
represented by portable Go byte/mask operations. This promises the source state
machine, not SIMD code generation or SIMD performance. The native default uses
the amd64 SSE2 baseline; other hosts conservatively use generic decoding unless
the caller supplies a truthful explicit feature witness. No 386 SSE2 or loong64
LSX feature is assumed from architecture alone.

`generate.py` compiles the entire unchanged original file with the actual
`rustc_span` position types and their real operators, using the complete pinned
compiler graph built by `tools/port-rust-span-reference/prepare.py`. It does not
replace operators or containers with shims. Rustc is matching nightly
2026-11-04 (commit `708c94ef093ecc42fb8851e17d8383d999727abc`). Four independent
debug/overflow profiles produce 30,780 records. The replay checks values, vector
capacities, returned scan overflow and partially appended vectors after caught
panics. Cases include every scalar length/chunk boundary, seeded input,
prepopulated capacities and output offsets near `u32::MAX`.

Native execution covers x86_64/SSE2 and generic decoding. Portable Go LSX is
compared with the identical native SSE2/generic mask/control contract; native
LSX hardware has not been executed. This is recorded in the manifest rather
than presented as a native loong64 validation.

```sh
python3 tools/port-rust-analyze-source-file/generate.py --check
GOCACHE=/tmp/ferrugo-go123-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test ./port/rust/compiler/rustc_span/src ./port/rust/compiler/rustc_span/src/analyze_source_file -run 'TestAnalyzeSourceFileNativeStateOracle|TestOriginalAnalyzeCases'
```

The source and test SHA-256 identities, fixture identities and native wrapper
identity are retained in `src/testdata/analyze_source_file_manifest.json`.
Standard Vec growth for these two fixed-size Copy metadata types is scoped
support, not a claim to port a standard-library Vec source file. Explicit host
panic context and ordinary allocation/runtime representation follow the shared
span runtime contract; no Rust/Go ABI, signal-number or allocator-timing identity
is claimed.

Every generator run revalidates the complete reference graph through the shared
verified helper, including original Git/archive bytes, original tests and
linkage. `--rustc` and `FERRUGO_RUSTC` are validated against the exact selected
SDK; executable dispatch identity is preserved. The included original analyzer
comes from the verified copied source tree, not a mutable upstream working
file. Compiles and reference subprocesses have finite timeouts. RelativeBytePos
arithmetic delegates to its actual lib-owned operator implementation, retaining
ordinary hook dispatch and explicit selected-profile source locations.
