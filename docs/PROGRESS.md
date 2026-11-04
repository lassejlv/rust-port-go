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

## 2026-11-04 — checkpoint 4: review corrections

- Independent review found no backend mismatch across 102 additional evaluation-order programs compared with rustc.
- Corrected CRLF normalization and escaped line continuations in Rust strings while retaining original source offsets.
- Rejected leading underscores in Unicode escapes, bare carriage returns in strings and invalid UTF-8 anywhere in source, including comments.
- Tightened the manifest parser to reject invalid UTF-8, raw controls, lone carriage returns and non-TOML whitespace before comment removal or escape decoding.
- Added source/string regressions and 156 manifest syntax cases checked against Python's TOML/UTF-8 parsers.
- Passed the full local race-enabled suite with rustc differential comparisons and vet after these corrections.
- Verified hosted Linux/macOS CI, including rustc comparisons, for checkpoint 3: https://github.com/lassejlv/rust-port-go/actions/runs/41828174561.

The full port remains incomplete. Current compatibility and the remaining scope are documented separately; successful bootstrap checks do not establish Rust ecosystem compatibility.

## 2026-11-04 — checkpoint 5: multi-file modules

- Added inline/external modules, public/private visibility, qualified calls and simple function/module import aliases.
- Added canonical name resolution with `crate`, `self`, repeated `super`, import-chain and privacy checks while preserving lexical variable shadowing.
- Added a crate file loader for both `name.rs` and `name/mod.rs`, including nested inline/external bases, ambiguity/missing-file errors and physical file-cycle checks.
- Unified CLI/Cargo checks, native builds and C emission around file-based crate analysis. Output protection includes all participating module sources and hard-link/symlink aliases.
- Preserved original source filenames and positions in diagnostics.
- Supported root `main` aliases through an independent compiler-generated wrapper after review against rustc.
- Added eight multi-file success cases and nine rejection cases; all 47 committed conformance cases pass against rustc 1.99.1. Resolver cases and loader/CLI/Cargo regression tests pass.
- Passed the full local race suite and vet; updated the Cargo example to import Fibonacci from a second source file.

The multi-file goal is implemented locally; hosted CI will validate the pushed checkpoint. Named structures and field access are the next language milestone. The complete compiler and Cargo port remains the overall goal.

## 2026-11-04 — checkpoint 6: consistent C output paths

- Fixed a reviewed path mismatch in C-file emission: source protection checked a normalized output path while publication used the original spelling. A symlink followed by `..` could otherwise target a loaded module source.
- C emission now uses one normalized absolute output path for protection, temporary-file creation and publication.
- Added a regression that reproduces the symlink/parent traversal case and verifies source bytes are preserved; focused C-emission tests pass.

## 2026-11-04 — checkpoint 7: typed Copy records

- Added named record declarations deriving Copy and Clone, primitive/nested field types, constructors with shorthand fields, projections and mutable local field assignments.
- Resolved record types and import aliases across modules into canonical type identities while preserving source type spelling in parsed signatures.
- Added typed function parameters/results and branch values containing records, field/type privacy checks, and cycle detection for impossible recursive by-value layouts.
- Added topologically ordered C record definitions, collision-free record identifiers, by-value copies and constructor evaluation in written field order.
- Preserved mutable assignment places instead of mutating temporary copies; RHS evaluation precedes field writes.
- Added 21 fixtures; all 68 committed conformance cases pass against rustc 1.99.1, including construction/copies/mutation/order/privacy failures and documented unsupported move-only records.
- Independent review found and corrected primitive-name record import shadowing; function/module aliases named like primitive types remain allowed.
- Updated the multi-file Cargo demo to return Fibonacci results in records. Local full race tests, vet and the demo pass.

General ownership, borrowing, generic traits, move-only records, enums and standard libraries are still required for the overall port. The next Cargo milestone is local library crates and path dependency graphs.
