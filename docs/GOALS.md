# Project goals

The overall goal remains a complete Rust compiler and Cargo implementation in Go. No bootstrap milestone completes that project.

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

## Active goal: host integer semantics

Compile signed and unsigned integers through 64 bits plus isize/usize, with exact bounds, bitwise operations, shifts, compound assignments and checked runtime failures. Compare supported numeric behavior with rustc and validate generated C for undefined behavior, then push and verify CI.

The next expression milestone is integer casts, followed by fixed-size arrays and usize indexing.

## Implemented milestones

- Executable bootstrap: source parsing/type checking, native output, and single-package Cargo-style commands.
- Multi-file modules: nested inline/external modules, qualified calls, aliases and privacy; pushed in checkpoint 5 with reviewed output-path correction in checkpoint 6.
- Named Copy records: typed fields, copies, construction/mutation, function values and module visibility; pushed in checkpoint 7.

The module and Copy-record checkpoints passed hosted Linux/macOS CI with rustc comparisons. The [progress log](PROGRESS.md) records the exact implemented boundaries and checks.

Ownership, borrowing, generic trait selection, move-only values, enums, standard libraries and complete upstream conformance remain required for the overall compiler port.
