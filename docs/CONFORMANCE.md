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

The suite contains 68 cases:

| Category | Cases | Coverage |
| --- | --- | --- |
| Single-file runs | 25 | Arithmetic and integer boundaries, recursion, lexical scopes, loop control, short-circuiting and evaluation order, UTF-8/NUL strings, formatting, unit/if/block results, runtime failures, and Copy records |
| Module runs | 11 | Both external-file layouts, nested inline/external modules, `crate`/`self`/`super`, function/module/struct import aliases, duplicate function basenames, recursive imports, private helper access, and records crossing module boundaries |
| Single-file compile failures | 16 | Type/arity/return errors, immutable assignment, branch/condition errors, lifetimes and divergent arithmetic, missing/duplicate/unknown record fields, field type mismatches, immutable field mutation, distinct record types, and recursive value layouts |
| Module failures | 11 | Missing/ambiguous files, duplicate declarations/imports, private functions/modules/types/fields, unresolved imports, root-level `super` and import cycles |
| Valid unsupported Rust | 5 | Arrays, traits, generics, raw strings and plain structs without the supported Copy/Clone derive |

All implemented record fixtures use brace structs with `#[derive(Copy, Clone)]` and scalar or nested Copy-record fields. Positive cases verify field-expression evaluation in written order, by-value copies, nested field mutation, field access on call/if/block results, construction shorthand, record function results, defining-module access to private fields, and imported struct aliases. The unsupported plain-struct fixture deliberately omits the derive; it records a current implementation boundary, rather than treating valid move-only Rust as invalid.

Passing these examples establishes behavior for these cases on the current host backend. Ownership, borrowing, traits, the standard library, crates.io dependencies, cross-target ABI behavior and the rest of the Rust/Cargo roadmap need separate implementation and validation.
