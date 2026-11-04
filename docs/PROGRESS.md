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

## 2026-11-04 — checkpoint 3: expression semantics and differential validation

- Added value-producing if/else and block expressions, unit expressions, branch inference and control-transfer analysis.
- Tightened expression-statement grammar against rustc and rejected unsupported value-less divergent arithmetic rather than accepting invalid Rust.
- Enforced the accepted &str return lifetime-elision boundary.
- Added 30 committed fixtures: 18 executable cases, eight invalid-Rust rejections and four valid-but-unsupported boundaries. All match their expected behavior against rustc 1.99.1 on Linux.
- Added 12 more native expression/control-transfer regressions, manually cross-checked with rustc, and expanded fuzzing without observed parser/emitter panics.
- Passed the full local race-enabled Go suite, vet and the package demo.
- Fixed a macOS test path-alias assumption found by hosted CI; added rustc differential checks to Linux/macOS CI.
- Pinned the inspected Rust/Cargo source revisions and recorded a component inventory. No upstream component is fully ported.

The initial executable-bootstrap goal is achieved. The overall compiler/Cargo port is still incomplete; ownership, traits/generics, user-defined types, modules, standard libraries, crate linking and dependency resolution remain substantial next work.
