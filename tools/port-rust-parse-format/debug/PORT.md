# Format-parser derived Debug proof

This restores the complete derived `Debug` contracts in the already counted
`compiler/rustc_parse_format/src/lib.rs`; it adds no source-file completion
credit. The nine source types are ParseMode, Piece, Argument, FormatSpec,
Position, Alignment, Sign, DebugHex and Count. ParseError, Suggestion and Parser
have no source Debug implementation, so this does not add one for them.

`port/rust/compiler/rustc_parse_format/src/debug_format.go` provides the primary
`WriteDebugFormatter(*DebugFormatter) error` methods. `DebugFormatter`,
`DebugOptions` and `DebugContext` are aliases for the neutral shared rustfmt
adapter. Use `DefaultDebugOptions()` for ordinary Rust defaults; a zero Fill
field explicitly represents a NUL fill. `WriteDebug`, `RustDebugWith` and the
existing `RustDebug` methods are fallible-writer, explicit-options and ordinary
string conveniences over that same implementation.

Compact tuple/struct/Option fields share the caller's actual mutable formatter.
Pretty fields use copied options and a nested indentation adapter. Numeric
zero-padding therefore leaks its changed fill/alignment on a compact writer
error exactly as native Rust does, while pretty child changes stay local.
Both successful and failed formatting can be followed by another numeric
Debug operation using the same formatter. All returned writer errors preserve
the caller's error value. Sinks can override `WriteChar(rune) error` independently
from string writes; otherwise the default is a UTF-8 string write. Source std
Range fields stream start, `..`, and end with the inherited numeric options.
Strings/chars use the shared Rust 1.99.1 / Unicode 17 escape profile, independent
of Go's Unicode tables, including empty writes around escaped substrings.

The shared helper is a scoped adapter for the contracts this file delegates to
std. It grants no standard-library source-file credit. The parser algorithms,
original source fixtures and prior parser/lexer original-test corpus are not
changed by this restoration. Go enum integers outside source variants, malformed
Go strings, and invalid runes remain outside the Rust source value domain.

The development wrapper in `oracle.rs` links the entire unchanged pinned parser
and lexer. The generator obtains all four original implementation/test files by
Git object at revision `2d20b42b883dcfc014f3c2c56081eeb6091c2179`, verifies both
Git blob and SHA-256, and compiles them with the existing locked reference
workspace's manifests. The parent test-only rustc_index size assertion facade
is retained unchanged; it does not service formatting or grant dependency source
credit. Each selected registry archive and every expanded file Cargo compiles
are verified before and after build, including normalized Cargo manifests;
symlinks, missing files, extra files and changed files fail.

The corpus has 156,460 native records: 40 values exercise every enum variant,
all FormatSpec option/payload fields, empty and escaped Unicode strings/chars,
u16/usize boundaries, Range fields, Box delegation and nested Option/struct/
tuple forms. Nineteen actual Rust format patterns cover default/pretty,
lower/upper hex, plus/minus, zero padding, alignment, fill (including Unicode,
newline and NUL) and precision. Each is tested with default and overridden
character sinks, every failing write plus a no-failure sentinel, and both direct
return and formatter reuse. Records include operation kinds/text, accepted
prefix, exact success/failure results and fill/alignment before, after and after
reuse. The successful default text also continues to match the existing
57,824-step parser corpus. This proof uses rustc 1.99.1 on Linux x86-64 for the
source version 1.101.0 pin; native usize fixtures are host-width specific.

Run from the repository root:

```sh
python3 tools/port-rust-parse-format/debug/generate.py -check
python3 -m unittest discover -s tools/port-rust-parse-format/debug -p 'test_*.py'
go test -race ./port/rust/compiler/rustc_parse_format/src
go vet ./port/rust/compiler/rustc_parse_format/src
```

Without `-check`, the generator refreshes native evidence. Existing gzip bytes
are preserved whenever the complete decompressed output matches. `-check`
never rewrites artifacts. Both modes replay native evidence through the Go port.
`FERRUGO_RUSTC`, `FERRUGO_CARGO` and `FERRUGO_GO` accept PATH names or paths;
explicitly empty/missing selections fail. `FERRUGO_RUST_SOURCE` selects the
read-only pinned Git cache; `CARGO_HOME` selects the already fetched registry
cache. The default uses this project's existing local caches and toolchains.
It runs Cargo offline, sets `GIT_NO_LAZY_FETCH=1`, and never fetches source or
provides a production Rust fallback. Bare-PATH selections work in CI.
