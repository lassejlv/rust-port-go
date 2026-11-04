# Local package graph regression and differential fixtures

`internal/cargoconformance` exercises local Rust package graphs through Ferrugo's Go Cargo command API. The committed `testdata/packages` fixture trees use path dependencies only. This is a bounded regression suite for the implemented package subset, not Cargo's upstream integration suite or evidence of registry, workspace, feature or general ecosystem compatibility.

Run Ferrugo's package fixtures with Go and a host C11 compiler:

```sh
go test -count=1 -v ./internal/cargoconformance
```

Every case copies its complete fixture tree into a fresh temporary directory before calling `cargo.Run` with an absolute `--manifest-path`. The tests never change the process working directory, build inside committed fixture directories, or update fixture goldens. Positive packages first pass `check`; Ferrugo's check must leave no target directory. Binary cases then run and compare exact stdout bytes and successful exit status with `output.stdout`.

Optional upstream comparison uses an explicitly selected installed Cargo:

```sh
FERRUGO_CARGO=/absolute/path/to/cargo \
FERRUGO_RUSTC=/absolute/path/to/rustc \
go test -count=1 -v ./internal/cargoconformance
```

When `FERRUGO_CARGO` is set, the harness validates that executable with `--version`. If `FERRUGO_RUSTC` is set, its value must explicitly select a working Rust compiler, including when an empty value was supplied. Otherwise the harness looks for a `rustc` executable beside the selected Cargo. It validates and logs both versions; an unavailable or invalid requested toolchain fails the test instead of silently falling back. When `FERRUGO_CARGO` is unset, upstream package comparison is disabled, regardless of compiler-only `FERRUGO_RUSTC` settings used by the separate language suite.

Upstream Cargo receives a separate fixture copy, a fresh `CARGO_HOME`, a fresh `CARGO_TARGET_DIR`, an explicit `RUSTC`, and `--offline`. Wrapper and Rust-flag overrides are removed from that subprocess environment to keep the reference builds predictable. No dependencies are fetched and no network setup or toolchain download occurs in these tests. Cargo may create lockfiles and metadata only in its temporary fixture copy and target directory. Ferrugo and upstream Cargo each check the same original package graph; binary cases also run and compare against the same goldens.

Positive commands must succeed. Negative fixtures assert rejection and a diagnostic from both implementations; failure status numbers and diagnostic text are not compared because Cargo conventionally returns 101 and Ferrugo may return 1. Runtime panic behavior is covered by the separate language conformance suite.

At most three cases execute concurrently. Upstream subprocesses have a 45-second context timeout and use one build job. Ferrugo's in-process API has a 30-second bounded test wait for these trusted, finite fixtures; its API does not expose hard cancellation, and the timeout is not presented as a child-process cancellation guarantee.

## Coverage

The suite has eight positive cases (seven binaries and one library check) and eight rejection cases.

| Fixture | Behavior |
| --- | --- |
| `run/direct` | Direct local library dependency |
| `run/transitive` | Binary → library → library calls |
| `run/renamed` | Dependency alias with a different package name |
| `run/diamond` | Shared path-package identity across a dependency diamond; Copy-record values cross both branches |
| `run/crate_isolation` | Each crate's `crate::`/`super::` and private helpers resolve within that crate |
| `run/opaque_record` | Public library functions exchange an inferred public Copy record with private fields |
| `run/external_modules` | External module files and imports in binary and library crates |
| `check/library_root` | Library package checking without a `main` function |
| `reject/missing_path` | Missing local dependency path |
| `reject/cycle` | Cyclic library package graph |
| `reject/nonlibrary_dependency` | A binary-only dependency cannot provide a library crate |
| `reject/wrong_package` | Path dependency's actual package name disagrees with `package = ...` |
| `reject/private_function` | Cross-crate private function access |
| `reject/private_type` | Cross-crate use of a private return type |
| `reject/crosscrate_parent` | A library's `super::` cannot escape into its dependent crate |
| `reject/std_alias` | A custom dependency alias cannot replace Rust's implicitly injected standard prelude |

All sixteen original fixture graphs were independently validated with Cargo 1.99.1 (`5f94df478`) and rustc 1.99.1 (`58cf8f9c3`) on Linux. Future coverage additions should preserve independent original-Cargo validation and the temporary-copy rule.
