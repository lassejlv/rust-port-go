# Cargo restricted names: complete source-file port

Source: `rust-lang/cargo@24de4721844c7a4d839072bab6fb66d41c9bc1f4`,
`crates/cargo-util-schemas/src/restricted_names.rs` (263 lines), Git blob
`dfe44bfb92bd1f485091476be02a96c6ee836ce3`, raw SHA-256
`6d3869b64e61f4de029e5ac69f4257c34b3dc6010e4cf39554880b13d15b1950`.
The exact mirrored target is
`port/cargo/crates/cargo-util-schemas/src/restricted_names.go`, package
`cargoutilschemas`. Cargo's MIT/Apache-2.0 licenses are in `port/cargo`.

All eight source functions are implemented:

| Rust | Go |
| --- | --- |
| `validate_package_name` | `ValidatePackageName` |
| `validate_registry_name` | `ValidateRegistryName` |
| `validate_name` | `ValidateName` |
| `sanitize_package_name` | `SanitizePackageName` |
| `sanitize_name` | `SanitizeName` |
| `validate_profile_name` | `ValidateProfileName` |
| `validate_feature_name` | `ValidateFeatureName` |
| `validate_path_base_name` | `ValidatePathBaseName` |

These six validators return Go `error` (nil for Rust `Ok(())`), with failures
represented by the public `*NameValidationError` and private `errorKind`.
The two sanitizers return strings. Functions are exported to cross the Go
package boundaries introduced by the mirrored module layout; the original
functions have crate visibility. The Rust `Result<T>` alias requires no
additional Go type.

`Error()` preserves the exact `thiserror` Display text; `RustDebug()` preserves
derived Debug, including tuple/variant names, field order, and Rust character
and string escaping. `Source()` delegates to the private inner error's source,
which is nil for every variant. There is deliberately no `Unwrap()` method:
Rust's transparent Error implementation does not add the private error as a
source-chain node. The original error has no Clone, equality, or public
variant-inspection API, and the port adds none.

`restricted_names_format.go` completes the streaming trait contracts. Both
the public error and its private kind implement
`WriteDisplayFormatter(*rustfmt.Formatter)` and
`WriteDebugFormatter(*rustfmt.Formatter)`. Display delegates transparently to
the inner kind and streams the original literal/argument pieces; it ignores
outer width, precision, fill and numeric flags. Debug uses the real nested
tuple/struct shapes and neutral Unicode-17 character/string witnesses.
Compact fields retain the mutable formatter, while pretty fields receive
copied children and indentation adapters. The first writer error stops later
writes; actual empty writes, independent character writes, accepted prefixes,
formatter state and reuse remain observable. `WriteDisplay`/`WriteDebug`
provide writer/options conveniences, and `Error`/`RustDebug` use the same
streaming implementation. The Go `fmt.Formatter` bridge prevents Go from
adding outer padding/truncation; `%#v` selects compact Rust Debug. Full source
flags use the primary formatter API.

Writer arguments are temporary borrowed operations, never owned or destroyed
by the error. Translated source panic sites use the explicit formatter's
`Context.Panic`; arbitrary raw Go panics retain Go-local cleanup timing. The
neutral helpers are supporting runtime code, with no additional source-file
completion credit.

The validation order, first offending character, component splitting, reserved
profile names, and sanitizer prefix-skipping behavior are retained. In
particular:

- `ValidateProfileName("")` succeeds, as the source does.
- The initial digit-specific diagnostic uses Rust `char::is_digit(10)`, which
  recognizes ASCII `0` through `9`; Arabic digits get the XID-start diagnostic.
- Features reject `dep:` before checking slashes, and slashes before examining
  the initial character.
- Package names validate each `::` component independently, including empty
  components. Sanitizers replace wholly invalid components with `package`.
- Any Unicode scalar is a valid sanitizer placeholder, including NUL, slash,
  or `?`. The source does not guarantee that arbitrary placeholders yield
  valid package names; the port makes no stronger guarantee.

All string parameters must be valid UTF-8, reflecting Rust `&str` and
`String`. Placeholders must be Unicode scalars, reflecting Rust `char`.
Invalid Go byte strings, negative runes, surrogates, and out-of-range runes
panic explicitly. Go does not reproduce Rust's static string lifetime or
ownership restrictions; immutable Go string values preserve the required
contents. Validators and sanitizers accept no callbacks or mutable internal
state; formatting accepts the source's fallible writer witness.

## Exact Unicode profile and dependencies

The original source is pinned independently from its standard-library runtime.
The oracle uses exactly
`rustc 1.99.1 (58cf8f9c3 2026-10-13)`, whose standard library uses Unicode
17.0.0. This is not a claim that a different Rust compiler or the separately
pinned Rust repository's Unicode 18 standard library has identical behavior.

XID behavior comes from the already ported lexer predicates, with exact
`unicode-ident 1.0.24` / Unicode 17 tables. The full scalar oracle verifies
those predicates directly against the Cargo source's locked dependency,
including the lexer helper's intentional addition of underscore to XID_Start.
Alphanumeric and retained Debug-escaping profile tables are generated from the
reference standard library. Streaming Debug reuses the independently validated
neutral Rust-1.99.1/Unicode-17 helpers. Production never uses host Go Unicode
classification or escaping tables.

Profile lowercasing calls `util.Lowercase17`, a small compatibility wrapper
around the existing complete lowercase implementation used by Cargo's
`edit_distance` port. It retains multi-scalar expansions and contextual final
sigma. Its complete scalar mappings are compared again here, with contextual
cases in the request corpus. The wrapper is reuse support, not another source
file completion. Generated data retains the Unicode license in
`restricted_names_unicode_LICENSE`; existing lexer dependency licenses remain
with that source port.

The production package uses the Go standard library and these Go ports only.
It never launches Rust or Cargo. Validation scans names in source order;
predicate lookups use the existing pinned tables. Sanitization uses linear
scanning with storage for its output. Lowercasing allocates linear scalar and
result buffers, as documented by the shared helper. Allocation patterns and
out-of-memory behavior are not a cross-language compatibility claim.

## Original-source oracle and reproduction

`main.go` reads the complete source and original root `Cargo.lock` from their
pinned Git objects and verifies both blob IDs and raw SHA-256 values before
copying the unchanged source into a temporary standalone Cargo project.
`oracle.rs` calls all eight actual functions and the original error's Display,
Debug, and `std::error::Error::source`. The original inline test module also
compiles and runs unchanged; its 18 assertions are mirrored, in order, in
`restricted_names_extra_test.go` without separate source-file credit.
The separate `format_oracle.rs` calls the unchanged error's generated
Display/Debug implementations through real `std::fmt::Formatter` values.

The standalone lockfile preserves the original versions and checksums of all
six required registry packages: thiserror 2.0.18, thiserror-impl 2.0.18,
unicode-ident 1.0.24, proc-macro2 1.0.106, quote 1.0.46, and syn 2.0.118.
Before building, the tool checks Cargo's selected package paths, verifies each
archive's original checksum, and compares every archived member against the
actual expanded source that Cargo will compile. Registry-cache edits cannot
silently change the oracle.

After the pinned Cargo Git cache is prepared, fetch those packages once:

```sh
export CARGO_HOME="$PWD/.toolchain/port-cargo-home"
RUSTC="$PWD/.toolchain/rust/bin/rustc" .toolchain/rust/bin/cargo fetch --locked \
  --manifest-path tools/port-cargo-restricted-names/Cargo.toml
go run ./tools/port-cargo-restricted-names
go test ./port/cargo/crates/cargo-util-schemas/src
```

Every subsequent reference build uses `--locked --offline`. Git reads disable
lazy fetching; missing pinned objects fail explicitly. Reference source files
are never edited. The generated baseline fixtures replay in Go without Rust,
Cargo, Git, network access, or an upstream source checkout.

`-root`, `-cargo-source`, `-cargo-home`, `-cargo`, and `-rustc` select paths and
tools. Tool flags override `FERRUGO_CARGO`/`FERRUGO_RUSTC`; directory flags
override `FERRUGO_CARGO_SOURCE`/`CARGO_HOME`. Bare executable names resolve
through PATH; relative paths resolve against the repository root. Defaults use
the local pinned toolchain, Cargo cache, and source cache. Explicit empty,
missing, or incompatible selections fail rather than falling back.

`go run ./tools/port-cargo-restricted-names -check` regenerates the original
results and checks all retained output without changing files. Compatible
existing gzip streams are retained only after bounded decompression, CRC
validation, and exact raw equality, so Go 1.23 and later compressor differences
do not rewrite fixtures. Check mode preserves original generation-host and
Go-version metadata. `manifest.json` binds the complete source/lock pins,
dependency checksums, wrapper/manifests/generator, request and response hashes,
generated tables, raw fixtures, and compressed fixture bytes.

The binary corpus contains requests and original answers for 32,779 cases.
Inputs cover all eight functions, the original examples, profile reservations
and Unicode lowercase counterexamples, short exhaustive combinations,
deterministic random strings, separator and error precedence, NUL/control and
non-BMP characters, invalid prefixes, arbitrary contexts, and varied scalar
placeholders. The separate scalar profile observes all 1,112,064 scalar
values for XID_Start, XID_Continue, standard alphanumeric, ASCII digit, lowercase,
and character/string Debug-escape predicates. Surrogates are explicitly marked
as non-scalars. The Rust wrapper also checks the special escape mappings and
first/following string and character Debug behavior across that whole domain.

`restricted_names_formats.bin.gz` retains 31,664 original formatter scripts
with 659,866 attempted writes and 28,016 failing scripts. Sixteen error inputs
reach every variant and exercise empty payloads, Unicode, combining marks,
NUL/control characters, quotes, backslashes and multiline text. Nineteen
format patterns cover pretty/compact Debug, both hexadecimal Debug flags,
width/precision/alignment, signs, zero padding, and ASCII/Unicode/newline/NUL
fill. Each input runs with default and independently overridden `write_char`.
The reference discovers successful write counts, then independently runs every
failure cut and cuts beyond the final write. It deliberately reuses the same
formatter for an integer after the first success/error, recording both results,
all writer attempts/accepted bytes, and fill/alignment/width/precision/public
flag state before, after and after reuse. Expected values always come from
original Rust execution, never from the Go implementation.

This validates the complete restricted-names file and its dependency profile.
It adds no accepted syntax or Unicode naming claims to the bootstrap Cargo
manifest parser and does not establish completion of Cargo or rustc.
