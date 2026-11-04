# rustc thousands formatting port

Reference: rust-lang/rust `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.
The complete implementation and original four-test module map as follows:

| Original source | Go destination |
| --- | --- |
| `compiler/rustc_data_structures/src/thousands/mod.rs` | `port/rust/compiler/rustc_data_structures/src/thousands/mod.go` |
| `compiler/rustc_data_structures/src/thousands/tests.rs` | `port/rust/compiler/rustc_data_structures/src/thousands/tests_test.go` |

The `thousands` package exposes `UsizeWithUnderscores(uint)`,
`IsizeWithUnderscores(int)` and `F64p1WithUnderscores(float64)`; private
`formatWithUnderscores(string)` corresponds to the original private helper.
There are no omitted declarations, traits or callbacks. The original tests'
full 35 assertions are retained in their four corresponding Go test functions.

The helper finds the ASCII digit run after one optional leading minus sign.
It groups only that run and preserves decimal fractions, exponent suffixes,
Unicode characters, NULs and other trailing text. Rust `char::is_digit(10)`
recognizes ASCII digits; Unicode decimal numerals are not separators' inputs.
Rust `String` is always valid UTF-8, so the Go private helper explicitly rejects
invalid UTF-8 rather than accepting an extra input domain. A single Go builder
replaces repeated insertion while preserving results.

`uint` and `int` preserve Rust's host-sized unsigned/signed APIs, including
their extremes without taking an absolute value that could overflow.
`strconv.FormatFloat(value, 'f', 1, 64)` supplies fixed one-decimal-place
rounding, including ties to even and the sign of negative zero. Infinity
spellings are explicitly adapted to `inf` and `-inf`. All NaNs, regardless of
payload or sign, format as `NaN`. Original-source reference outputs verify
these decisions rather than relying on presumed similarity between formatters.

Production code uses only the Go standard library and never invokes Rust or an
external process. No locale behavior or numeric API beyond the source module
is introduced. MIT OR Apache-2.0 source licensing is retained; license texts
are `port/rust/LICENSE-MIT` and `port/rust/LICENSE-APACHE`.

## Development reference harness

The runner reads the exact two pinned Git objects with lazy fetching disabled
and verifies both Git blob IDs and complete SHA-256 hashes. The original
module bytes are embedded verbatim in an enclosing Rust module; a separate
test-only visibility adapter calls the private helper. This wrapper is needed
to cover arbitrary strings without changing the private function's visibility
or rewriting the source's inner documentation. The original `tests.rs` bytes
are copied unchanged to their normal module location. The complete wrapped
module compiles as an executable and as a Rust test harness; all four original
tests execute. `oracle.rs` and its visibility adapter are development code and
are never included in the production Go package.

The reference fixture contains only the original Rust results and requests;
the Go package's tests compare every applicable record with the actual port.
The manifest binds source/test blob IDs and hashes, compiler version, wrapper
hash, request hash, original-output hash, case counts and compressed-fixture
hash. Ordinary Go tests need neither Rust nor upstream sources.

On 32-bit hosts, ordinary fixture tests compare every float/string case and
integer inputs within that host's API domain; native 32-bit integer extrema
also have explicit tests. Regeneration uses the pinned 64-bit reference profile.
Live comparison is opt-in through `FERRUGO_RUSTC` and supports a bare executable
name on `PATH`. `FERRUGO_RUST_SOURCE` or `-rust-source` selects the pinned source
checkout, and `-root` permits invoking the tool from another directory. Invalid
explicit compiler/source overrides fail without selecting another reference.
