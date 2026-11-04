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

The initial 18 run fixtures cover arithmetic precedence and signed division/remainder, integer bases and boundaries, typed and mutually recursive functions, lexical scopes and shadowing, nested loops with break/continue, boolean short-circuiting, operand/argument evaluation order, UTF-8 and embedded-NUL strings, equality, escaped format braces, unit returns, value-producing if and nested block expressions, and assertion/overflow runtime failures. Eight compile-fail fixtures cover type mismatches, immutable assignment, wrong arity, missing return values, mismatched branch types, non-boolean conditions, missing return-reference lifetimes, and arithmetic on an unconstrained divergent block. Eight additional module run cases cover sibling `name.rs`, directory `name/mod.rs`, nested inline/external modules, `crate`/`self`/`super` paths, function and module import aliases, separate functions with the same basename, mutually recursive imports, and access through private parent helpers. Nine module-fail cases cover missing/ambiguous files, duplicate declarations/imports, private functions/modules, unknown imports, root-level `super` escapes and import cycles. Four unsupported fixtures record arrays, traits, generics and raw-string boundaries. Together these categories contain 47 cases.

Passing these examples establishes behavior for these cases on the current host backend. Ownership, borrowing, traits, the standard library, crates.io dependencies, cross-target ABI behavior and the rest of the Rust/Cargo roadmap need separate implementation and validation.
