# Project goals

The overall goal is a complete file-for-file Go port of the pinned Rust compiler and Cargo sources that compiles production Rust. No bootstrap milestone completes that project.

## Active goal: complete upstream source-file ports

Map every pinned upstream file and required submodule, preserve source provenance and licenses, and port complete implementation files with their tests. Rust programs used as compiler test inputs stay Rust fixtures. Start with the entire `compiler/rustc_lexer/src/lib.rs` and Cargo's `src/util/edit_distance.rs`, validate them against the original source, then integrate validated components into the compiler/package manager.

Each completed file must have a recorded source revision and blob identity, a Go counterpart, all behavior implemented, and reproducible comparison evidence. The bootstrap does not count as a file-for-file port. Production readiness requires the complete frontend, ownership/borrow checking, traits/generics, lowering/code generation, standard libraries, crate/ABI/metadata support, and Cargo's registry, lockfile, feature and workspace behavior, tested against upstream suites and representative production crates.

## Implemented milestone: local Cargo library dependencies

Build and run a binary package whose supported Rust code depends on local library-only packages through `path` dependencies, including renamed and transitive dependencies and a shared dependency diamond.

Acceptance criteria:

- Parse the documented local dependency manifest subset and reject unsupported sources/settings.
- Preserve separate crate roots, external names, nominal type identity and visibility.
- Check libraries without a main entrypoint; require main only for the selected binary.
- Detect graph cycles, missing/mismatched packages and conflicting aliases/identities.
- Preserve source bytes and the logical source/manifest directory when symlink leaves are involved.
- Compare committed package fixtures with original Cargo offline, pass local checks, push on `main`, and verify hosted CI.

This milestone links supported library code into a bootstrap C executable. Rust rlib/ABI artifacts, registry/git dependencies, feature/version resolution, lockfiles and workspaces remain later Cargo work.

This implementation is pushed in checkpoint 8; local comparison and race checks pass, and hosted CI is pending for that checkpoint.

## Implemented milestone: host integer semantics

Compile signed and unsigned integers through 64 bits plus isize/usize, with exact bounds, bitwise operations, shifts, compound assignments and checked runtime failures. Compare supported numeric behavior with rustc and validate generated C for undefined behavior, then push and verify CI.

The integer implementation is pushed in checkpoint 9 and passed hosted Linux/macOS CI.

## Implemented milestone: integer casts

Compile `as` casts across supported integer widths and from bool to integer with Rust narrowing/sign-extension rules, evaluation order and literal inference boundaries. Compare the expanded fixtures against rustc and validate optimized generated C, then push and verify CI.

The cast implementation is pushed in checkpoint 10. Local comparisons and race tests pass; hosted CI is tracked in the progress log.

## Implemented milestone: fixed-size Copy arrays

Compile owned Copy-array types, literals/repeats, function values, indexing/mutation and length access. Preserve index place semantics, side effects and bounds panics, and handle zero-sized values without inflating storage. Compare the expanded fixtures with rustc and review generated C under UBSan, then push and verify CI.

This implementation is pushed in checkpoint 11. Local checks pass; hosted CI is tracked in the progress log.

## Implemented milestone: bounded for loops

Compile bounded integer ranges and owned Copy-array iteration with exact once-only header evaluation, scoped mutable/immutable bindings, nested break/continue and inclusive integer maxima. All 150 language conformance cases match original rustc. This finishes the bootstrap work already underway before the full-file port goal was made explicit.

## Implemented milestones

- Executable bootstrap: source parsing/type checking, native output, and single-package Cargo-style commands.
- Multi-file modules: nested inline/external modules, qualified calls, aliases and privacy; pushed in checkpoint 5 with reviewed output-path correction in checkpoint 6.
- Named Copy records: typed fields, copies, construction/mutation, function values and module visibility; pushed in checkpoint 7.

The module and Copy-record checkpoints passed hosted Linux/macOS CI with rustc comparisons. The [progress log](PROGRESS.md) records the exact implemented boundaries and checks.

Ownership, borrowing, generic trait selection, move-only values, enums, standard libraries and complete upstream conformance remain required for the overall compiler port.
