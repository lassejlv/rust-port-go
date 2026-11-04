# Format-parser validation

Validated the complete pinned `rustc_parse_format/src/lib.rs` and `tests.rs`
against Rust `2d20b42b883dcfc014f3c2c56081eeb6091c2179`, using original
`rustc_lexer` source and the exact pinned registry dependencies. Source blobs,
SHA-256 hashes and the full dependency/checksum list are in `manifest.json`.
The standalone original build uses rustc 1.99.1 on Linux x86-64.

All **28 original parser tests** and **17 original lexer tests** pass without
editing their source. The parser test's sole rustc_index dependency is the
faithful compile-time `Piece` size assertion documented in `PORT.md`.

The Go port matches **4,684 original-source requests and 57,824 scripted
operations/state observations**:

| Profile or mode | Requests |
| --- | ---: |
| Overflow checked | 2,341 |
| Overflow wrapping | 2,343 |
| Format strings | 2,257 |
| Inline assembly | 1,241 |
| Diagnostic attributes | 1,186 |

Every applicable result compares the complete Piece/Argument/FormatSpec/Position/
Count data, identifier/index helper results, original derived Debug text,
accumulated errors and suggestions, current implicit argument, argument spans,
source-literal flag, line spans, and both lookaheads. Initial public-field changes
and subsequent scripted mutations are retained. Scripts continue after errors,
`None`, and panics to verify recovery and partially updated public state.

Constructor observations include 27 checked-add overflows, 100 checked-subtract
overflows, 340 range/bounds failures, and four UTF-8 character-boundary failures.
Six operation observations catch checked-add overflow with post-panic state.
No caught panic has an unclassified `other` category. Runtime panic wording and
source paths are normalized; parser diagnostic strings and suggestions compare
exactly, including Unicode/debug escapes and embedded controls.

Inputs include the upstream cases, deterministic short adversarial/random
strings, all modes, matched/mismatched/escaped/raw source snippets, Unicode,
NUL, appended newlines, raw delimiter styles, native-integer extremes, partial
consumption, seeded/replaced errors, and mutable public state. Malformed UTF-8
is excluded from this oracle because Rust accepts `&str`/`String`; Go's separate
input-contract tests reject it explicitly.

Completed checks:

- Generation and baseline replay pass under actual **Go 1.23.12**.
- Live original `-check` passes under **Go 1.27.3**, using bare PATH names for
  Cargo/rustc/Go and repository-relative source/cache overrides.
- Cross-Go checking preserves existing compressed bytes and generation
  provenance; original decoded results, all source/dependency/support hashes,
  request bytes and manifest still match.
- Tool unit tests pass on Go 1.23: non-default gzip retention, changed-evidence
  rejection, PATH/relative-path/flag selection, empty/missing executable
  rejection, expanded registry-source tampering and archive tampering.
- Tool `go vet` passes under Go 1.23.
- Seven end-to-end failed selections reject empty Cargo/Go/source overrides,
  a missing rustc, a wrong compiler, a wrong source revision and an unexpected
  positional argument. Corpus and manifest hashes remain unchanged after all
  failures.

Representative commands from the repository root:

```sh
go test ./tools/port-rust-parse-format
go test ./port/rust/compiler/rustc_parse_format/src -run '^TestOriginalParserOracle$' -count=1
FERRUGO_RUSTC=rustc FERRUGO_CARGO=cargo FERRUGO_GO=go \
  go run ./tools/port-rust-parse-format -check
go vet ./tools/port-rust-parse-format
```

The implementation owner additionally reports these completed checks:

- Actual Go 1.23.12 full parser-package tests and vet, plus Go 1.27 race tests.
- 84,150 executions of two-worker fuzzing without failure.
- Complete Unicode-scalar checks for char escape-debug, char Debug, standalone
  and non-leading string Debug, and all 25 whitespace scalars.
- Clone/equality/default/index helper tests and checked/wrapping public Curarg
  mutation tests.
- Instrumentation showing nonzero execution for every parser routine, including
  recovery, iterator, count, word, integer, format, assembly and diagnostic paths.

Those results are separate from the independent original-parser oracle and its
replay recorded above. Neither establishes completion of compiler macro
expansion or production Rust compilation.
