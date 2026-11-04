# Project goals

The overall goal remains a complete Rust compiler and Cargo implementation in Go. The executable bootstrap was the first milestone, and does not complete that project.

## Implemented milestone: multi-file Rust packages

Compile and run one binary package with nested inline and external modules, qualified function calls, simple `use` aliases and visibility checks, through both `ferrugo` and `ferrugo-cargo`.

Acceptance criteria:

- Resolve `name.rs` and `name/mod.rs`, including modules nested inside inline declarations.
- Resolve local, `self`, `super` and `crate` paths and simple function/module aliases.
- Enforce item privacy and reject duplicate definitions, missing/ambiguous files, unresolved imports and cycles.
- Keep source filenames in diagnostics and protect every participating source file from output replacement.
- Make check, native builds and C emission use the same crate loading and analysis path.
- Compare committed positive and rejection cases against rustc, pass CI, and push the implementation to `main`.

The module implementation is pushed in checkpoint 5, with a reviewed output-path correction in checkpoint 6. Local differential/race checks pass; their hosted CI runs are queued.

## Active goal: named Copy records

Compile and run brace-style records deriving Copy and Clone with primitive/nested fields, field access/mutation, typed function parameters/results and module/type-import privacy. Validate constructor evaluation order, copies, nested field writes and invalid-program rejection against rustc, then push the implementation and verify CI.

Move-only records and borrowing are deliberately deferred until their ownership analysis is implemented. Cargo's next major work includes local library crates and path dependency graphs before registry resolution. Ownership, borrowing, generic trait selection, standard libraries and complete upstream conformance remain required for the overall goal.
