# Span-encoding validation

The complete original `compiler/rustc_span/src/span_encoding.rs` is compiled
unchanged at Git blob `3947b9095f0c2cca407b096d1c352cc827296fce`, SHA-256
`6f5efe1fef5a093cf9046ec794813ae5a9e3f56e9a524d4f7e3276a35ffa13d0`.
The source version is1.101.0; the recorded native witness is Rust1.99.1 on Linux
x86-64. PORT.md describes the exact surrounding dependency boundary and why
this establishes no complete surrounding span/session/source-map crate port.

Completed checks:

- **15,264 native requests**, with **142,168 completed steps** and **64 actual
  native abort cases**, match the Go replay. Each of the four independent
  debug-assertion/ordinary-overflow profiles executes3,816 requests. The step
  count includes the64 successfully completed raw-slot preludes in the abort
  records; the Go logger separately reports142,104 nonabort completed steps.
- All four packing forms, interner deduplication/order, no-session fast/failure
  paths, query/tracking/reentry order, parent/context changes, typed Hash,
  Clone/default replacement, location-only ordering, raw invalid inputs and
  post-catch state are compared with actual Rust. No native panic is left
  unclassified.
- Span/OrdSpan/SpanData formatting compares19 native patterns, independent C/S
  dispatch/default fallback, failing writes/accepted prefixes, getter side
  effects, source-map dependency calls, post-error state and formatter reuse.
- Real custom FnOnce/capture/Drop and field glue prove mutated-cell lifetime,
  moved fields, destruction before reconstruction, decode/call/constructor
  failures, inherited unwinding and panic-hook contexts. Double cleanup panic
  and panic-in-hook cases actually terminate by native SIGABRT. Only observed
  pre-abort state is retained; no postmortem cleanup is manufactured.
- **Go1.23.12** full span package **race tests passed** on the final behavior
  (37.571s); package **vet passed**. This includes the actual full native
  replayer, its64 isolated Go abort children, existing Edition proof, profile
  capture guards and ownership/concurrency unit tests.
- **Go1.27.3** portable live `generate.py -check` passed with bare PATH names
  for Go/rustc/Cargo and relative source/cache selections. All native profiles
  rebuilt/replayed; gzip and manifest bytes remained unchanged.
- **14 isolated integrity regressions passed**: scoped source extraction,
  static callback closure completeness, diagnostic/comment brace handling,
  original-lock checks, local dependency substitution, exact archives and
  normalized expanded source, missing/extra/modified files, symlink roots,
  staged input freezing and actual-abort observation parsing.
- **18 original files**, **20 exact selected fragments**, **53 staged reference
  Rust/Cargo inputs**, **26 pinned registry archives** and **1,273 expanded
  registry files** are verified. Staged/tool/request-builder bytes are frozen
  before compilation and checked again afterward; no used dependency algorithm
  is replaced by a stub. Reference-only dependency interfaces remain explicit.
- A Go1.23 **386** test artifact cross-compiled. This kernel cannot execute
  native ELF32; no 32-bit runtime comparison is claimed. The retained usize/
  formatting reference profile is64-bit.

The implementation owner additionally reports a final modern-Go full package
run and360,145 roundtrip fuzz executions in10 seconds with two workers. Those
self-consistency checks complement, rather than generate, native expectations.

Retained raw corpus SHA-256:
`8878558ae3a0dfac67116269653ea2723817f97ae0f96587bec56b54831c22e6`.
Preserved gzip SHA-256:
`c1448d919c7496e7be2bc190480c32a60099a8cf3087ba3b73ea3e26492eb27b`
(1,101,505 bytes).
Frozen request-builder SHA-256:
`b747ccd57ff654f7a833f8144900e8ad5be5e5735f1d7f4ace3acc96563549dd`.

manifest.json binds the evidence to exact Git objects/fragment offsets,
compiled input hashes, original-lock registry sources, retained tool sources,
request-builder bytes, native responses and explicit build profiles. Merely
preparing the reference does not constitute behavioral validation. Production
code never calls this driver, Rust or Cargo.

For a cold registry, `python3 tools/port-rust-span-encoding/fetch.py` first
stages the exact source reference, explicitly fetches its locked dependencies,
then verifies every expanded registry source. It does not run behavioral
comparisons. `generate.py -check` and the ambient-debug companion remain
offline; hosted CI executes both after this preparation.
