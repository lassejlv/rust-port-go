# Validation

The independent native driver calls the actual unchanged Svh, Fingerprint,
macros, memory codecs and stable hasher from the complete original compiler
reference. It uses official nightly2026-11-04, Rust1.101.0-nightly commit
`708c94ef093ecc42fb8851e17d8383d999727abc`. The reference itself verifies all
pinned sources/dependencies and runs all43 original span tests.

The retained57,540 original-source records cover:

- 136 boundary/random fingerprints, all projections and string forms, Clone
  and default clone_from, equality/default ne, actual encoded bytes, original
  memory decoding and a seeded real stable hash.
- Raw-write-only and independently typed Hasher implementations; default
  hash_slice over zero, one and two elements; seeded non-Copy caller state.
- 4,896 exact/reference/wrapped Unhasher requests across both assertion
  configurations, seeded states and wrapping limb sums. Every Hash request is
  replayed through the direct and concrete-state dictionary Go entrypoints.
- 1,184 successful formatter baselines and50,508 failure cuts, derived from
  every actual attempted native write. Twenty-three real format patterns cover
  Display precision/padding and nested Debug flags, independent character/string
  writers, NUL/Unicode fill, exact byte chunks and reused mutable formatters.
  Direct Fingerprint Debug is included to validate the neutral entrypoint.

The module has no arithmetic or debug-dependent source branch. Ordinary native
operations use the checked reference profile; the underlying Unhasher's actual
debug assertion is independently exercised in checked and release configurations.
No pointer-width/allocator ABI is inferred from these scalar observations.

Reproduce with the matching installed compiler and minimum Go1.23:

```sh
python3 tools/port-rust-svh/generate.py --check
go test -race ./port/rust/compiler/rustc_data_structures/src -run '^(TestOriginalSvh|TestFullHasherContract)$'
go vet ./port/rust/compiler/rustc_data_structures/src
```

`--prepared` explicitly reuses both already prepared matching native profiles.
The generator honors the matching-nightly environment selections through the
shared native-profile helper; explicit unsupported compilers fail. It preserves
existing compressed bytes when the retained raw records match. Tests verify the
fixture SHA-256 and exact count. The manifest binds source, drivers, Go code,
format patterns and both native assertion profiles. Rust is only a development
reference; production Svh operations run in Go.

Independent source/API review found no omissions. Minimum-Go replay passes;
final isolated race/vet/native gates are recorded in the parent progress log.
