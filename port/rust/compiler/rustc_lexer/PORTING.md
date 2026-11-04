# rustc_lexer source port

This package ports both implementation and test files from `rust-lang/rust` at `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.

| Upstream source | Git blob | Go counterpart |
| --- | --- | --- |
| `compiler/rustc_lexer/src/lib.rs` | `0aa57e75889fa2d81deb1ec8bf5480b1c930fff1` | `src/lib.go` |
| `compiler/rustc_lexer/src/tests.rs` | `a7357ba38c8e4484da814ef3c22bf33b75bdbeb2` | `src/tests_test.go` |

The implementation covers all token/literal variants and metadata, the UTF-8 byte cursor, comments, identifiers and prefixes, numeric/string/character/lifetime lexing, raw-string error metadata, shebang handling, guarded strings and frontmatter. All 17 upstream tests are preserved, including 14 shebang assertions and nine exact token snapshots. Additional tests exercise cursor and error boundaries.

Go represents data-carrying enum variants as a tag and comparable payload fields; unused payload fields are zero. Optional values use an explicit boolean. `Tokenize` returns a slice without EOF; callers needing incremental consumption use `Cursor.AdvanceToken`. String entrypoints reject invalid UTF-8 because Rust's `&str` excludes it. Cursor `Prev` follows the upstream debug-build cursor behavior. Raw error comparisons use structured fields; the extra `RustDebug` diagnostic convenience is not a general implementation of Rust's Unicode debug-character formatter.

The Go standard library implements byte search in place of `memchr`. Unicode predicates use generated data from the exact upstream lockfile dependencies, `unicode-ident 1.0.24` and `unicode-properties 0.1.4`, both Unicode 17.0.0. Their tables, checksums, complete scalar reference and notices are recorded in `unicode_manifest.json`, `src/unicode_tables.go`, `src/testdata/unicode_oracle.bin.gz` and `dependencies/`. The generator is `tools/port-lexer-unicode`; it checks official crate archive hashes and retains dependency notices. These lexer dependencies have their own Unicode version, distinct from the pinned core library's Unicode 18.0.0.

The frontend adapter in `internal/syntax/raw_lexer_adapter.go` uses the port's cursor for actual source boundaries. It joins supported adjacent punctuation and decodes supported string values for the bootstrap parser. Parser and backend boundaries remain explicit: ASCII identifiers, supported integer/string literals and the documented subset. Doc comments retain the bootstrap's treatment as skipped comments; attribute expansion is not implemented. Adapting this lexer does not complete parsing, macro expansion or compiler compatibility.

The original MIT/Apache licenses and Rust copyright notice are retained under `port/rust`. Unicode dependency licenses and notices are retained in this component's `dependencies/` directory.

Validation commands and original-source comparison evidence are recorded separately. Passing this component's tests does not establish production Rust compilation.
