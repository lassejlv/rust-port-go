# Complete format-parser original-source oracle

This development harness compiles the complete, unchanged Rust files at
`rust-lang/rust@2d20b42b883dcfc014f3c2c56081eeb6091c2179`:

| Source | Git blob |
| --- | --- |
| `compiler/rustc_parse_format/src/lib.rs` | `90e04fe388ad6f592d5785e8c36964ce284a0da6` |
| `compiler/rustc_parse_format/src/tests.rs` | `a2e03a514fa5fcc8304933ef59e02aa4400c4181` |
| `compiler/rustc_lexer/src/lib.rs` | `0aa57e75889fa2d81deb1ec8bf5480b1c930fff1` |
| `compiler/rustc_lexer/src/tests.rs` | `a7357ba38c8e4484da814ef3c22bf33b75bdbeb2` |

`manifest.json` records their full SHA-256 hashes, the exact root lockfile blob,
dependency versions/checksums, wrapper/support-file hashes, deterministic request
builder hash, profile/case counts, original-result hashes and retained fixture
hash. The selected reference compiler is exactly
`rustc 1.99.1 (58cf8f9c3 2026-10-13)`, on a 64-bit host. This fixes the standard
library's Unicode/debug-escaping behavior as well as parser arithmetic.

The implementation being validated lives in
`port/rust/compiler/rustc_parse_format/src/lib.go`; its original translated tests
and Go corpus replayer live beside it. The Rust code in this tool is validation
code, never a production service or compiler fallback.

## Original code and dependency boundary

The generator reads each source from its pinned Git object, verifies both its
object ID and raw SHA-256, and copies its original bytes into a temporary Cargo
workspace. It never modifies the reference checkout, fetches absent Git blobs
implicitly, or substitutes a Go lexer or unescaper for the original parser's
dependencies.

The temporary manifests isolate these files from the compiler workspace. Exact
registry selections match the pinned root `Cargo.lock`, including:

- rustc-literal-escaper 0.0.8, archive SHA-256
  `bfe6f213fb658c8fb95baabd5420393438cf5a98d707f5dd701d9197c705f71e`;
- unicode-ident 1.0.24 and unicode-properties 0.1.4 (emoji feature only), both
  Unicode 17.0.0, and memchr 2.8.0 for the unchanged lexer;
- expect-test 1.5.1 and its upstream-locked dependencies for original lexer tests;
- serde_json 1.0.145 and its upstream-locked dependencies for this wrapper's
  JSON protocol. They are not parser implementation dependencies.

All 16 registry selections retain their original checksums. Before compilation,
the generator checks the actual package paths selected by Cargo metadata,
verifies each selected archive checksum, and compares every original archive
member with the expanded source Cargo will compile. An edited registry source
tree therefore cannot silently replace the reference implementation.

The original parser test configuration imports exactly one rustc_index macro:
`static_assert_size!(Piece<'_>, 16)`. The small validation-only macro crate here
performs that same compile-time type-size assertion. It implements no index
algorithms and receives no source-file completion credit. Both original parser
and lexer test modules compile unchanged and run in full: 28 and 17 tests.

## Complete observations

Each JSON request constructs a fresh parser with its mode, raw-string style,
optional source snippet, appended-newline flag and overflow profile. The
canonical response contains the constructor result, its initial state, state
after optional public-field initialization, and every scripted operation.

Every state includes all public fields: `errors`, `curarg`, `arg_places`,
`is_source_literal`, and `line_spans`, plus `peek` and `peek_ahead` observations.
Every `Next` result contains every argument/format/position/count field,
`Argument::is_identifier`, `Position::index`, and the original derived `Debug`
text. Characters use numeric Unicode scalars in structured fields; ranges use
two-element unsigned-integer arrays. No integer is rounded through JSON float64.

Scripts support `next`, `peek`, `peek_ahead`, replacement of each mutable public
field, error clearing, and insertion of a fixed fully populated synthetic
error. Calls continue after `Next` returns `None` or panics, preserving this
iterator's unfused error recovery and partial mutations. Constructor panics
produce no parser state; operation panics retain and report the live parser's
post-panic public state. Panic categories are `add_overflow`, `sub_overflow`,
`char_boundary`, `bounds`, or `other`; source paths and runtime panic wording
are not a compatibility claim.

Checked and wrapping requests use separate whole-workspace Cargo builds.
`CARGO_PROFILE_DEV_OVERFLOW_CHECKS` applies to the original parser and its
dependencies, rather than only to the protocol executable. Debug assertions
remain enabled in both profiles. All input/snippet strings are valid UTF-8, as
required by the original `&str`/`String` APIs.

`cases.jsonl.gz` contains `{ "request": ..., "response": ... }` records. Its
baseline is Go-only: no Rust, Cargo, Git, upstream checkout, network or registry
cache is needed to replay it. The nested `char-unicode` tool independently
produces the pinned scalar whitespace and Rust debug-escaping predicates used
by the Go port. It is not a substitute for this original-parser comparison.

## Reproduction and portability

After preparing the pinned reference cache, fetch the locked registry packages
once with network access:

```sh
python3 tools/port-inventory/inventory.py prepare
export CARGO_HOME="$PWD/.toolchain/port-cargo-home"
RUSTUP_TOOLCHAIN=1.99.1 cargo fetch --locked \
  --manifest-path tools/port-rust-parse-format/Cargo.toml
```

The tracked temporary-workspace manifests are sufficient for `cargo fetch`;
the generator materializes the verified Rust source files before compilation.
All comparison/test builds then run with `--locked --offline`:

```sh
go test ./port/rust/compiler/rustc_parse_format/src
FERRUGO_RUSTC=rustc FERRUGO_CARGO=cargo go run ./tools/port-rust-parse-format -check
```

Omit `-check` to regenerate the reference corpus and manifest. The generator asks
the Go comparison test for its deterministic request stream using
`FERRUGO_PARSE_FORMAT_DUMP_REQUESTS` and `TestDumpOracleRequests`; it does not
derive expected answers from the Go implementation. Run the Go replayer after
regeneration to establish parity. Rust's original tests are run on every live
generation/check.

`-root`, `-rust-source`, `-cargo-home`, `-rustc`, `-cargo`, and `-go` select paths
and tools. Flags override `FERRUGO_RUST_SOURCE`, `CARGO_HOME`, `FERRUGO_RUSTC`,
`FERRUGO_CARGO`, and `FERRUGO_GO`. Bare executable names resolve through PATH;
paths resolve against the repository root. Defaults use the repository-local
Rust toolchain, the currently running Go toolchain, and isolated Cargo cache.
Explicit empty, missing or incompatible selections fail without falling back.

Go 1.23 is supported. Regeneration/checking retains an existing valid gzip
stream whenever its decompressed original results are unchanged; compression
differences between Go versions do not rewrite evidence. Check mode also
preserves the manifest's original generation host/Go-version provenance and
reports the actual validating tools. Unit tests independently check PATH/flag
selection, registry tampering rejection and non-default gzip preservation.

This harness validates these complete parser/lexer files and their declared
dependencies. It does not establish completion of macro expansion, compiler
diagnostics integration, or production Rust compilation.
