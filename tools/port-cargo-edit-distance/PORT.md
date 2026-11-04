# Cargo edit-distance source port

This is the complete port of `src/util/edit_distance.rs` from Cargo commit `24de4721844c7a4d839072bab6fb66d41c9bc1f4` to `port/cargo/src/util/edit_distance.go`, with the original test in `edit_distance_test.go`.

- Git source blob: `e67259956fc26a53b1be30efb371161249dfff8a`.
- Raw source SHA-256: `073eb1a086cd2d2e74d1d8ce98b87013a8756f0187313ea1b7340b63184c6407`.
- Source license: MIT OR Apache-2.0; preserved at `port/cargo/LICENSE-MIT` and `port/cargo/LICENSE-APACHE`.
- Generated Unicode data terms: `port/cargo/src/util/edit_distance_unicode_LICENSE`.

Every executable item in the file is covered:

| Rust item | Go item | Preserved behavior |
| --- | --- | --- |
| `edit_distance` | `EditDistance` | Rust lowercasing; scalar comparisons; shorter-column swap; length-limit early return; common prefix/suffix removal; three reused rows; restricted Damerau transposition; final limit check |
| `closest` | `Closest[T]` | Ordered iteration, one key lookup per candidate, distance-three cutoff, first minimum on ties, optional generic result |
| `closest_msg` | `ClosestMsg[T]` | Exact two-newline suggestion suffix, selected key lookup again, empty string when no candidate matches |
| `test_edit_distance` | `TestEditDistanceUpstream` | Entire Unicode-scalar identity loop with the same exclusive upper bound, plus all six multiline accented-string cases |

`EditDistance(a, b string, limit uint) (uint, bool)` maps Rust `usize` and `Option<usize>` to an unsigned native-size integer and an explicit presence flag. `Closest` and `ClosestMsg` take Go 1.23 `iter.Seq[T]` and `func(*T) string`; they are not restricted to string candidates. A false distance result returns zero in its ignored value slot. First-minimum tie behavior is confirmed by the original Rust oracle, rather than inferred from Go's ordering conventions.

Rust `&str` admits only valid UTF-8. These Go APIs reject invalid UTF-8 with an explicit panic, including callback strings and the message's kind. They never silently replace malformed input. Production code is entirely Go, uses compiled-in tables, and performs no file I/O, process execution, Rust delegation or network calls.

The distance algorithm remains the upstream restricted Damerau/optimal-string-alignment algorithm, rather than unrestricted Damerau distance. The limit does not introduce a new banded algorithm: apart from the upstream length-difference check, the remaining matrix is evaluated and then filtered by the limit. Time is O(m*n) after trimming; storage includes O(m+n) lowercased scalar buffers and three O(min(m,n)) rows.

## Pinned standard-library behavior

Cargo's function calls `std::str::to_lowercase`, so the Cargo source commit alone does not fix its Unicode behavior. This port explicitly selects **Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), Unicode 17.0.0**, matching the original Cargo validation toolchain and the Cargo package's Rust 1.99 minimum. It does not use the host Go Unicode tables.

The separately pinned rustc main repository (`2d20b42b883dcfc014f3c2c56081eeb6091c2179`) declares **Unicode 18.0.0** in `library/core/src/unicode/unicode_data.rs`. That is a different standard-library profile; this port does not claim its Unicode 17 tables reproduce that future profile. Supporting a different linked Rust standard library requires regenerating and validating a separate profile deliberately.

`edit_distance_unicode.go` contains 1,488 full lowercase mappings, including expansions, plus final-sigma context predicates. The generator queries original Rust for all 1,112,064 Unicode scalars. For each scalar `c`, `AΣc` identifies non-case-ignorable cased scalars, and `AcΣ` identifies non-case-ignorable uncased scalars; the complement is case-ignorable. Only the effective cased predicate consumed after ignoring case-ignorable scalars is stored. No broader Unicode-property API is claimed. Go lowering is checked against both complete context probes for every scalar, as well as standalone lowering.

## Reproduction and CI

Ordinary Go tests consume committed, checksummed Rust-oracle fixtures and require neither Rust nor an upstream checkout:

```sh
go test ./port/cargo/src/util
```

Set `FERRUGO_RUSTC` to opt into compiling and executing the original Rust source. An explicitly requested empty, unavailable or different-version compiler fails; it never silently skips. `FERRUGO_CARGO_SOURCE` optionally selects the pinned Cargo checkout, defaulting to `.toolchain/upstream-cargo` relative to the repository root.

```sh
RUSTUP_TOOLCHAIN=1.99.1 FERRUGO_RUSTC=rustc go test ./port/cargo/src/util
RUSTUP_TOOLCHAIN=1.99.1 FERRUGO_RUSTC=rustc go run ./tools/port-cargo-edit-distance -check
```

The generator also accepts `-root`, `-rustc` and `-cargo-source`. Compiler names are resolved on PATH; relative paths resolve against the repository root. Omitting `-check` regenerates deterministic tables, fixtures and the checksum manifest. Both generation and live tests compile/run the original upstream Rust test and compare the exact original-source oracle streams. This tool uses only Rust's standard library, so it needs no Cargo dependency download. `FERRUGO_CARGO` and `CARGO_HOME` may remain set by the source-port CI job but are not needed by this component.

The checked-in checksum manifest is `port/cargo/src/util/edit_distance_unicode_manifest.json`. Validation results and scope are recorded in `VALIDATION.md` beside this report.
