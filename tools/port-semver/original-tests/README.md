This directory preserves the tests and benchmark workloads from the official
`semver` 1.0.28 archive. The archive SHA-256 is
`8a7852d02fc848982e0c167ef163aaff9cd91dc640ba85e263cb1ce46fae51cd`.
The implementation and its independent semantic oracle are owned by the parent
`tools/port-semver` directory.

All six original test/support files (893 lines) map to the Go tests, and all
three workloads from `benches/parse.rs` (21 lines) map to Go benchmarks. The
audit checks the 34 original test names in declaration order, every original
string/character and integer literal occurrence, and all 71 assertion sites.
The version and requirement test bodies are generated from the original
statements, preserving case order and signed expressions. `audit.json` records
the source/target hashes and the exact file mapping.

Run from the repository root, with Go 1.23 or later:

```sh
python3 tools/port-semver/original-tests/mirror.py --check
FERRUGO_GO="$(command -v go)" python3 tools/port-semver/original-tests/audit.py --check
go test -race ./port/dependencies/semver/v1_0_28/tests ./port/dependencies/semver/v1_0_28/benches
go test -tags semver_no_std ./port/dependencies/semver/v1_0_28/tests
go test -tags semver_serde ./port/dependencies/semver/v1_0_28/tests
go test -tags 'semver_no_std semver_serde' ./port/dependencies/semver/v1_0_28/tests
go test -tags semver_miri ./port/dependencies/semver/v1_0_28/tests
go test -run '^$' -bench BenchmarkParse ./port/dependencies/semver/v1_0_28/benches
```

The default identifier test retains the original 280 lengths and 5.6 MB value.
`semver_miri` selects the original 40-iteration workload cap; it does not claim
that the Go program runs in Miri. The `semver_node` tag replaces only the test
requirement matcher with the exact external Node script. Requesting that profile
requires `node` and `require('semver')`; missing prerequisites fail explicitly.

```sh
go test -tags semver_node ./port/dependencies/semver/v1_0_28/tests
```

Go has no Rust `Send + Sync` bound. The exact original compiler witness remains
in `fixtures/tests/test_autotrait.rs` and compiles in the unchanged Rust suite.
The Go mirror adds read-only concurrent checks in the same seven-type order,
which are exercised under the race detector. The original nested
`DefaultHasher` helper is also retained and executed unchanged. Its Go assertion
uses caller-owned deterministic hashing to check the original repeatability
property; it does not claim to implement Rust's concrete `DefaultHasher`.

`run.py` extracts every archive file unchanged into an ignored, isolated Cargo
workspace. Its adapted manifest selects the original four test crates and pins
the optional serde dependency. It runs all 34 original tests in each of `std`,
`no_std`, `serde`, and `no_std + serde`; it separately runs all four original
Rust doc examples (three lib.rs examples and the parse.rs Error example) in
every selected profile. The 34/71 test-file audit remains separate. `--node` adds the original
`test_node_semver` configuration. Archive checksums and every expanded registry
source file, including normalized Cargo manifests, are verified before and
after execution. Symlinks, changed files, missing files, and unexpected files
are rejected. This is a reference test runner, not a production Rust fallback.

```sh
FERRUGO_CARGO="$(command -v cargo)" FERRUGO_RUSTC="$(command -v rustc)" \
  python3 tools/port-semver/original-tests/run.py --offline --node
FERRUGO_RUN_SEMVER_ORIGINAL_TESTS=1 \
  go test -run '^TestOriginalRustProfiles$' ./port/dependencies/semver/v1_0_28/tests
```

The default Cargo cache is `.toolchain/port-cargo-home`; `CARGO_HOME` overrides
both the reference dependencies' and the verified semver archive cache. `--fetch` explicitly allows
Cargo to obtain locked dependencies. The first-time maintainer operation
`--fetch --write-lock` records the adapted workspace lockfile. The local Rust
toolchain remains the default when no `FERRUGO_CARGO`/`FERRUGO_RUSTC` is supplied.

The recorded native validation used Go 1.23.12 and Rust 1.99.1 on amd64. Both Go
test and benchmark packages also cross-compiled for 386. The current kernel
rejects those ELF32 binaries with `exec format error`; 32-bit runtime behavior
is not validated. `validation.json` records that distinction and the completed
profile checks. Ignored Rust logs and detailed per-dependency source hashes are
in `.toolchain/oracles/semver-original-tests/evidence.json` after execution.

The upstream licenses and archive inventory are retained in
`port/dependencies/semver/v1_0_28`; fixture source bytes remain covered by those
licenses. No whole-project inventory credit is implied by this test audit.
