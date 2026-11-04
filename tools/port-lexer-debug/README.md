# Lexer source contracts

The production port is `port/rust/compiler/rustc_lexer/src`. This tool compiles
the entire unchanged lexer `lib.rs` and its 17 original tests from Rust revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`. Git object IDs and SHA-256 digests
are checked before compilation. Registry dependencies use exact upstream lock
versions; both the official archives and every expanded compiled file are
checked before and after the build. No reference implementation is substituted.

The same validation client calls unchanged `rustc-literal-escaper 0.0.8` and
retains its separate fixtures under `tools/port-literal-contracts`.

Normal Go tests replay committed observations without Rust, Cargo, downloads or
the upstream checkout. Reproduce the observations with Python 3.11 or newer:

```sh
CARGO_HOME="$PWD/.toolchain/port-cargo-home" \
  .toolchain/rust/bin/cargo fetch --locked \
  --manifest-path tools/port-lexer-debug/Cargo.toml
python3 tools/port-lexer-debug/generate.py --check
python3 -m unittest discover -s tools/port-lexer-debug -p 'test_*.py'
```

`fetch` prepares a cold registry; generation and comparison always run offline.
The source cache defaults to `.toolchain/upstream-rust`; the necessary objects
are `Cargo.lock`, `compiler/rustc_lexer/src/lib.rs` and `tests.rs` at the pin.
`FERRUGO_RUST_SOURCE`, `FERRUGO_RUSTC`, `FERRUGO_CARGO` and `CARGO_HOME` may
override defaults. Relative paths use the repository root; bare executables
use `PATH`. Explicit empty or invalid overrides fail. The reference profile is
Rust/Cargo 1.99.1 on a 64-bit host. `RUSTC_BOOTSTRAP=1` enables custom FnMut
validation clients only; the original stable lexer bytes are unchanged.

The default scope checks all fixtures. `--scope` selects `lexer`, `literal`,
`order`, `owned-lexer`, `owned-literal`, `owned-iterator`, `processing-lexer`,
`processing-literal`, `processing-iterator`, `profiles`, `fold-next` or
`internal-context`. Running without
`--check` retains new observations and their source/adapter hashes. Equivalent
gzip bytes are preserved across Go versions. Separate primitive proofs run as:

```sh
.toolchain/go/bin/go run ./tools/port-lexer-debug/char -check
.toolchain/go/bin/go run ./tools/port-lexer-debug/string -check
```

Those clients invoke native standard Debug on every Unicode scalar. Character
classification and every write operation are compared in blocks of 256. String
proofs cover each scalar alone and between adjacent escapes and Unicode text.
They pin Unicode 17.0.0 and the compiler identity. They support the adapter and
grant no additional standard-library source-file completion credit.

See `PORT.md` for the full source/API mapping and `VALIDATION.md` for retained
coverage and runtime boundaries.
