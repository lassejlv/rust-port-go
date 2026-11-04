# Bootstrap regression and differential fixtures

`internal/conformance` compiles committed Rust fixtures through Ferrugo's Go compiler and C backend, runs the resulting executables, and compares exact stdout bytes and process exit status with committed goldens. This is a small regression suite for the implemented language subset. It is not Rust's upstream compiler test suite, a language conformance certification, or evidence that arbitrary Rust programs work.

Run the baseline suite with a Go toolchain and a C11 compiler available:

```sh
go test -count=1 ./internal/conformance
```

The baseline needs no Rust installation and makes no network requests. Set `CC` to one C compiler executable if `cc` is not the intended compiler. Test builds and executables live in temporary directories; the harness does not generate or update files in the repository.

For differential validation, set `FERRUGO_RUSTC` to an installed rustc executable:

```sh
FERRUGO_RUSTC=/absolute/path/to/rustc go test -count=1 -v ./internal/conformance
```

The harness validates the selected executable with `--version`, then compiles each original run fixture with both compilers. It compares each executable against the same goldens and compares Ferrugo with rustc. If the variable is set but the executable is unavailable, invalid, or fails to run, the suite fails. When it is unset, the suite logs that the Rust comparison is disabled. No automatic Rust download or silent fallback occurs.

Rust reference builds use edition 2021 and explicit overflow checks. Runtime panics are compared by exit status (currently 101), while stderr wording is intentionally excluded because rustc and Ferrugo have different diagnostics. Stdout is compared as bytes, including embedded NUL and UTF-8. Executables have a three-second timeout and rustc invocations a fifteen-second timeout. At most four fixture subprocess groups run concurrently.

## Fixture layout

| Directory | Expected Ferrugo behavior | Optional rustc behavior |
| --- | --- | --- |
| `testdata/conformance/run` | Build, run, match `.stdout` and optional `.status` | Build the identical source, run, match goldens and Ferrugo |
| `testdata/conformance/modules/<case>` | Load `main.rs` and supporting modules; run and match `output.stdout` and optional `output.status` | Build the identical root and module tree; run and compare |
| `testdata/conformance/compile-fail` | Reject during file checking | Reject during compilation |
| `testdata/conformance/module-fail/<case>` | Reject the `main.rs` module tree during file checking | Reject the same module tree during compilation |
| `testdata/conformance/unsupported` | Reject during checking | Accept valid Rust outside Ferrugo's current subset |

Each single-file run fixture requires a sibling `.stdout` file. Each multi-file module case is a directory containing `main.rs`, supporting Rust files, and `output.stdout`. Both compilers receive the original root source path and discover the same module tree themselves. A sibling `.status` file (or `output.status` in a module case) supplies a decimal status from 0 to 255; when absent, the expected status is zero. Empty stdout uses an empty golden file. The harness never rewrites goldens. Invalid-Rust fixtures assert rejection, without requiring identical diagnostic wording or error codes. Valid unsupported fixtures are tested separately to keep intentional compatibility boundaries distinct from invalid Rust.

The suite contains 112 cases:

| Category | Cases | Coverage |
| --- | --- | --- |
| Single-file runs | 53 | Arithmetic and integer boundaries, recursion, lexical scopes, loop control, short-circuiting and evaluation order, UTF-8/NUL strings, formatting, unit/if/block results, runtime failures, Copy records, signed/unsigned widths, bitwise/shift operators, eager boolean logic and numeric casts |
| Module runs | 11 | Both external-file layouts, nested inline/external modules, `crate`/`self`/`super`, function/module/struct import aliases, duplicate function basenames, recursive imports, private helper access, and records crossing module boundaries |
| Single-file compile failures | 30 | Type/arity/return errors, immutable assignment, branch/condition errors, lifetimes and divergent arithmetic, missing/duplicate/unknown record fields, field type mismatches, immutable field mutation, distinct record types, recursive value layouts, incompatible integer types, unsigned negation, invalid bitwise/shift operand types and invalid casts |
| Module failures | 11 | Missing/ambiguous files, duplicate declarations/imports, private functions/modules/types/fields, unresolved imports, root-level `super` and import cycles |
| Valid unsupported Rust | 7 | Arrays, traits, generics, raw strings, 128-bit integers, character casts and plain structs without the supported Copy/Clone derive |

All implemented record fixtures use brace structs with `#[derive(Copy, Clone)]` and scalar or nested Copy-record fields. Positive cases verify field-expression evaluation in written order, by-value copies, nested field mutation, field access on call/if/block results, construction shorthand, record function results, defining-module access to private fields, and imported struct aliases. The unsupported plain-struct fixture deliberately omits the derive; it records a current implementation boundary, rather than treating valid move-only Rust as invalid.

Numeric fixtures exercise signed `i8`/`i16`/`i32`/`i64`/`isize` and unsigned `u8`/`u16`/`u32`/`u64`/`usize`, including the unsigned 64-bit maximum, signed minima, unsuffixed contextual inference, bitwise operators, Rust operator precedence, arithmetic right shifts, left shifts that discard high bits, differently typed shift counts, and scalar/record-field compound assignments. Eager boolean `&`/`|`/`^` cases use side-effect markers to distinguish them from short-circuiting operators. Six numeric panic fixtures check unsigned addition/subtraction/multiplication overflow, negative or out-of-range shift counts, and division by zero. Their operands cross function boundaries to avoid rustc's compile-time constant-arithmetic lint; panic stdout and status 101 are compared, while diagnostics remain compiler-specific. Pointer-integer examples use values valid on both 32-bit and 64-bit hosts, without claiming cross-compilation support.

Ten successful numeric-cast fixtures cover signed widening, unsigned-to-signed reinterpretation, narrowing/truncation, sign extension before wider unsigned conversion, pointer-sized integers, boolean-to-integer conversion, cast chains and precedence, record fields and function returns. Side-effect markers verify that both integer and boolean cast operands evaluate once. These conversions match Rust wrapping/truncation behavior without arithmetic-overflow panics. Eight invalid-cast fixtures require rejection of integer-to-boolean, string/unit/record-to-integer and integer-to-record conversions, plus out-of-range literals at cast-inference boundaries. A valid character-cast fixture remains in the unsupported category. Cast-inference regressions distinguish independently inferred conditional/arithmetic/shift operands from direct literals and literal block tails: `(256 + 0) as u8` wraps to zero, while `256 as u8` is rejected as an out-of-range inferred `u8` literal. The positive literal-block case retains the full unsigned 64-bit maximum.

Passing these examples establishes behavior for these cases on the current host backend. Ownership, borrowing, traits, the standard library, crates.io dependencies, cross-target ABI behavior and the rest of the Rust/Cargo roadmap need separate implementation and validation.
