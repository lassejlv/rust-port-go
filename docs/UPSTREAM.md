# Upstream scope and reference pins

Inspected on 2026-11-04:

- Rust reference: [`2d20b42b883dcfc014f3c2c56081eeb6091c2179`](https://github.com/rust-lang/rust/tree/2d20b42b883dcfc014f3c2c56081eeb6091c2179), including 80 directories under `compiler/`.
- Cargo reference: [`24de4721844c7a4d839072bab6fb66d41c9bc1f4`](https://github.com/rust-lang/cargo/tree/24de4721844c7a4d839072bab6fb66d41c9bc1f4), including compiler, config, context, diagnostics, ops, resolver, sources, util and workspace components.
- Executable comparison baseline: **rustc 1.99.1 (`58cf8f9c3`, 2026-10-13)**, edition 2021 with overflow checks enabled. This stable executable baseline differs from the source-reference pins above.

The machine-readable [inventory](upstream-inventory.json) records directory names and pins. It is an inventory of remaining scope, not a completion metric. No upstream compiler crate or Cargo component is fully ported. Ferrugo's current code is an independent Go bootstrap implementation informed by the upstream architecture and Rust behavior.

| Area | Upstream reference components | Ferrugo status |
| --- | --- | --- |
| Tokens, syntax and spans | rustc_lexer, rustc_parse, rustc_ast, rustc_span | Partial bootstrap subset with source locations |
| Name resolution and types | rustc_resolve, rustc_hir_typeck, rustc_infer, rustc_type_ir | Local function/binding resolution and primitive types only |
| Traits and ownership | rustc_trait_selection, rustc_next_trait_solver, rustc_borrowck | Unimplemented |
| Lowering and intermediate representations | rustc_ast_lowering, rustc_hir, rustc_mir_build, rustc_mir_transform | No equivalent typed IR/MIR pipeline yet |
| Constant evaluation and patterns | rustc_const_eval, rustc_pattern_analysis | Unimplemented |
| Macros and expansion | rustc_expand, rustc_builtin_macros, rustc_proc_macro | Only bounded print/println/assert builtins |
| Native code and target ABI | rustc_codegen_llvm, rustc_codegen_ssa, rustc_abi, rustc_target | Host C11 bootstrap backend; no Rust ABI |
| Standard library and runtime | library/core, library/alloc, library/std | Unimplemented; scalar/string helpers use libc |
| Metadata, incremental and queries | rustc_metadata, rustc_incremental, rustc_query_impl | Unimplemented |
| Diagnostics and tooling | rustc_errors, rustc_error_codes, rustc_driver | Minimal CLI diagnostics; no error-code compatibility |
| Cargo project commands | src/ops, src/workspace, src/compiler | Partial single-binary local build/check/run/new/init/clean |
| Cargo configuration and sources | src/config, src/sources, src/context | Strict local manifest subset only |
| Cargo resolution | src/resolver | Dependencies, features and lockfiles unimplemented |

The upstream Rust tree also includes its bootstrap system, libraries, tools and test suites; `compiler/` alone does not represent the whole repository. Cargo's ancillary crates, integration tests, registry protocols and documentation likewise add scope beyond its main source directories. Full compatibility needs the broader validation described in [ROADMAP.md](ROADMAP.md).

The committed fixture suite is authored for Ferrugo and is **not the upstream conformance suite**. Differential success on that suite establishes behavior only for those cases. See [CONFORMANCE.md](CONFORMANCE.md).
