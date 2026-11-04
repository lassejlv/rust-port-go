# Project goals

The overall goal remains a complete Rust compiler and Cargo implementation in Go. The executable bootstrap was the first milestone, and does not complete that project.

## Active goal: multi-file Rust packages

Compile and run one binary package with nested inline and external modules, qualified function calls, simple `use` aliases and visibility checks, through both `ferrugo` and `ferrugo-cargo`.

Acceptance criteria:

- Resolve `name.rs` and `name/mod.rs`, including modules nested inside inline declarations.
- Resolve local, `self`, `super` and `crate` paths and simple function/module aliases.
- Enforce item privacy and reject duplicate definitions, missing/ambiguous files, unresolved imports and cycles.
- Keep source filenames in diagnostics and protect every participating source file from output replacement.
- Make check, native builds and C emission use the same crate loading and analysis path.
- Compare committed positive and rejection cases against rustc, pass CI, and push the implementation to `main`.

The next language milestone after modules is named structures with field access and explicit types. Cargo's next major work includes local library crates and path dependency graphs before registry resolution. Ownership, borrowing, generic trait selection, standard libraries and complete upstream conformance remain required for the overall goal.
