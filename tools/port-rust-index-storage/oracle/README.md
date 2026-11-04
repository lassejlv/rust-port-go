# Original index storage comparison

This development oracle executes the unchanged pinned Rust `slice.rs`,
`vec.rs`, and `vec/tests.rs` through the reference workspace assembled by
[`../reference-build/prepare.py`](../reference-build/prepare.py). The complete
original `idx.rs`, index macro crate, and nightly serialization/hash source
dependencies are also present. None of the validation wrappers earns Go source
port credit.

The source pin is `2d20b42b883dcfc014f3c2c56081eeb6091c2179` (Rust 1.101.0
development sources). The recorded development compiler is Rust/Cargo 1.99.1.
`RUSTC_BOOTSTRAP=1` enables the existing source gates; the nightly build adds
only `-Zcrate-attr=feature(never_type)`. These settings are limited to reference
child builds. No original source body is rewritten.

From the repository root, prepare a cold cache with:

```sh
python3 tools/port-rust-index-storage/reference-build/prepare.py --fetch --validate
cargo fetch --locked --manifest-path tools/port-rust-index-storage/oracle/Cargo.toml
go run ./tools/port-rust-index-storage/oracle -check
```

The upstream cache must contain the pinned root lock and version/stage0 files,
`compiler/rustc_index`, `compiler/rustc_index_macros`,
`compiler/rustc_serialize`, and `compiler/rustc_hashes`. The inventory preparation
step hydrates these files in CI. After hydration/fetching, both preparation and
comparison can run without a network:

```sh
python3 tools/port-rust-index-storage/reference-build/prepare.py --offline --validate
go run ./tools/port-rust-index-storage/oracle -check
go test ./port/rust/compiler/rustc_index/src/vec
go test -tags=ferrugo_rustc_nightly ./port/rust/compiler/rustc_index/src/vec
```

`FERRUGO_GO`, `FERRUGO_CARGO`, and `FERRUGO_RUSTC` select explicit tool binaries.
Bare names use `PATH`; relative paths resolve from the repository root. Empty
explicit overrides fail. The default Go binary comes from the executing Go
runtime's GOROOT, so Go 1.23 CI does not need a repository-local Go installation.
Rust/Cargo default to `.toolchain/rust/bin`. `CARGO_HOME` is respected; its default
is `.toolchain/port-cargo-home`. `-root`, `-go`, `-cargo`, and `-rustc` provide
command-line overrides. Go 1.23 is the minimum supported generator version.

The optional test `FERRUGO_STORAGE_ORACLE=1 go test
./port/rust/compiler/rustc_index/src/vec -run '^TestOriginalStorageOracleLive$'`
executes the same `-check` command. Ordinary tests need only the committed
corpus and Go. They never replace missing original Rust output with Go output.

The generator verifies Git object bytes, root-lock dependency versions and
checksums, registry archives, and every selected expanded registry source file
before compiling. It builds eight independent whole-workspace profiles:
normal/nightly features × checked/wrapping arithmetic × enabled/disabled debug
assertions. The nightly driver uses the actual original serialization traits,
including independent `[T]` encoder and `Vec<T>` decoder specializations.

The committed snapshot contains 150,068 reference records and 189,668 steps.
See [VALIDATION.md](VALIDATION.md) for the completed toolchain, race, integrity,
and failure checks.

Requests are JSON lines produced by `TestDumpStorageOracleRequests`; each
committed line is `{ "request": ..., "response": ... }`. Responses preserve
return values, vector contents/capacities, callback order, callback panic
state, and each caught panic. Panic payload text is not a cross-runtime
contract. Abort records execute in isolated Rust/Go processes and compare all
events before termination. Go exit 134 represents the Rust abort class; it
does not claim SIGABRT signal identity.

The corpus includes custom typed indices, independent element trait methods,
default/overridden clone-from, partial clones and cleanup, owned factories and
ranges, custom slice-index conversion, option maps, trusted iterator fold and
for-each overrides, owned allocation reuse, full-width zero-sized vectors, and
nightly codec/extend operations. The scoped Copy tests use actual Copy element
types in Rust. Pointer-identity regressions live in the implementation owner's
focused tests; this corpus compares observable callback/state traces rather
than numeric cross-language addresses.

Run without `-check` to regenerate. `manifest.json` binds the source provenance,
registry provenance, wrapper/request-builder inputs, tool versions, profile
set, record counts, and compressed/decoded hashes. `-check` compares decoded
records and retains the committed compressed bytes and historical Go version,
allowing Go 1.23 and later Go gzip implementations to verify the same artifact.
