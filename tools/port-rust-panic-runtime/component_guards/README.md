# Component panic-operation integration

This independently compares real original source callbacks with the explicit
Go runtime protocol. The source algorithms are the unchanged whole pinned
lexer file, the checksum-verified literal-escaper 0.0.8 crate, and the full
original data-structures/index crates from the retained span reference graph.
There are no replacement source algorithms or dummy dependency branches.

The 182 isolated scripts cover:

- Cursor owned predicates and tokenizer `for_each`/`fold` callbacks.
- Literal-escaper owned string callbacks.
- Fingerprint owned component conversions/destruction.
- Binary-search owned extraction, temporary key operators/destruction.
- Interval cloned/original owned range getters/destruction.
- The actual borrowed StableHashCtxt callback bridge.
- A real lexer UTF-8 continuation-boundary failure during panic processing.

Normal, inherited-caught, inherited-escaping and processing contexts compare
ordinary hook calls, resumed payloads, real compiler cleanup diagnostics,
callback-local destruction, field glue and operation state. All 107 aborts are
actual native SIGABRT. Other native failures are rejected. Go exit134 represents
that abort class; no signal ABI identity is claimed.

User callback and Drop implementations are authored complete witnesses. User
body panics emit ordinary hooks. The actual compiler field/parameter cleanup
boundary emits the cleanup diagnostic exactly once, before later outer cleanup
can run. Expected fixture events are never consulted to synthesize hook calls.
The Fold cases also retain the initialized final accumulator return-place leak
when closure destruction panics.

Construction occurs under runtime1, invocation under explicitly supplied
runtime2. Calls, hooks and cleanup must retain runtime2. An additional real
caller switch replaces observerA with a silent observer/runtime0 before a
predicate panic; the Go operation supplies an explicit empty Context and must
not retain construction-time A. This is a supplied-context information-flow
check, not an assumption about unspecified Go TLS.

The native observation client uses matching official nightly-2026-11-04
(1.101.0 / 708c94ef0), with the checked/debug profile. Exact dependency versions
and archive checksums are locked. The client is compiled with an explicit
`--remap-path-prefix` for its own generated cache path, making its actual
compiled location stable across workspaces. Its observed source locations are
retained as explicit caller profile inputs; the Go runtime does no path
normalization or host-stack inference. The original compiler crate bodies and
their locations are unmodified.

```sh
python3 tools/port-rust-span-reference/install.py --fetch
python3 tools/port-rust-span-reference/prepare.py --fetch-source --fetch-deps
python3 tools/port-rust-panic-runtime/component_guards/generate.py --fetch-deps
python3 tools/port-rust-panic-runtime/component_guards/generate.py --prepared --check
```

Ordinary generation is offline. Tool selection honors `--rustc`/`--cargo` and
`FERRUGO_RUSTC`/`FERRUGO_CARGO`, preserving Rustup executable basenames.
`FERRUGO_RUST_SOURCE` and `CARGO_HOME` select verified caches. The output cache
has an ownership marker; no unowned directory is overwritten. Compilations and
all native/Go children have bounded timeouts and native core dumps are disabled.
Existing gzip bytes are preserved whenever the uncompressed corpus is equal.

The five `component_guard_runtime_test.go` files replay the baseline without
Rust or network access. Their shared caller witness is in `validation/`.
The manifest binds all seven migrated guard implementations, lexer context
propagation, the neutral runtime, callback witnesses, wrappers and original
source/dependency provenance. No standard-library or dependency file completion
credit is granted by this scoped support proof.
