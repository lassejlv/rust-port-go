# Progress

## 2026-11-04 — checkpoint 1: project foundation

- Named the project Ferrugo.
- Initialized the empty repository on `main`; all implementation work stays there.
- Defined shared syntax/compiler/package-manager interfaces for parallel implementation.
- Set the executable-bootstrap goal and recorded the remaining compiler and Cargo scope.
- Installed a verified Go toolchain locally for development; toolchain binaries are not tracked.

The full rustc/Cargo port remains incomplete. This log records tested milestones rather than an estimated percentage.

## 2026-11-04 — checkpoint 2: executable compiler and Cargo bootstrap

- Implemented Go lexer/parser, source spans and syntax validation.
- Added type checking, scopes/shadowing, mutability, calls/recursion, return validation, contextual integer types and literal bounds.
- Added C11 lowering with ordered evaluation, short-circuiting, length-carrying strings and checked signed arithmetic.
- Added `ferrugo` CLI with check, native output and C emission.
- Added Go Cargo-style new/init/check/build/run/clean commands and a strict manifest parser.
- Added regression tests and a working Fibonacci Cargo example; all current package tests pass and `make demo` runs successfully on Linux/GCC.
- Added Linux/macOS CI configuration. Hosted CI results are not yet verified.

Remaining scope is listed in ROADMAP.md and COMPATIBILITY.md. This is an executable bootstrap, not a complete rustc/Cargo port.
