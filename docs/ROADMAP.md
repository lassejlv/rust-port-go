# Compatibility roadmap

The goal is a Go implementation of Rust compilation and Cargo project management. A bootstrap subset is a starting point, not completion of that goal. Completion needs a pinned language/toolchain target, the upstream conformance suites, a compatible standard library and target runtime, and real-world crate validation.

## 1. Executable bootstrap (current)

- Source spans, lexer, parser, precise errors.
- Typed functions, local variables and scopes, integer/boolean/string primitives.
- Arithmetic, comparisons, calls, recursion and structured control flow.
- Explicitly bounded print and assertion builtins.
- Native executables through a C11 backend, without rustc.
- Cargo-style single-package new/init/check/build/run/clean.
- End-to-end success and rejection tests; CI; documented compatibility boundary.

## 2. Language front end

Rust tokens and grammar across a pinned edition, Unicode identifier correctness, attributes, modules and name resolution, item visibility, pattern matching, structs and enums, tuples/arrays/slices, const evaluation, richer diagnostics and rustc UI-test harness integration.

## 3. Type system and ownership

Generic parameters, inference and coercion, trait selection, associated items/types, closures, region inference, moves and initialization, borrowing and lifetimes, drop elaboration, unsafe validation, object safety and dynamic dispatch. These need explicit intermediate representations and dedicated conformance suites before being advertised as Rust compatible.

## 4. Runtime and code generation

Typed high-level IR and MIR, monomorphization, layout and ABI, unwinding and panic strategies, FFI, atomics, inline assembly, multiple targets, debugging metadata, incremental builds, optimization, and a backend strategy beyond the initial C emitter. Port or integrate a compatible core/alloc/std library and validate behavior against upstream rustc.

## 5. Macros and compiler tooling

Declarative and procedural macros, build-script integration, compiler plugins where relevant, metadata, Cargo/rustc protocol compatibility, rustdoc/tooling integration, formatter and diagnostics interfaces as separately scoped projects.

## 6. Cargo ecosystem

Full TOML manifests, workspaces/targets, path/git/registry sources, semantic-version and feature resolution, lockfiles, sparse indexes, downloads/checksums, offline/cache behavior, build graph and scheduling, profiles, target triples, cross-compilation, packaging/publishing, credentials and configuration. Validate Cargo's upstream tests and representative real projects.

## 7. Completion evidence

Document the pinned Rust and Cargo revisions and compatibility target. Run applicable upstream suites, core/alloc/std tests, cross-target tests, a substantial published crate corpus, and Cargo integration tests. Track failures publicly. A parser accepting a construct is insufficient: type checking, ownership, runtime behavior, diagnostics, and tooling contracts must agree with the target.
