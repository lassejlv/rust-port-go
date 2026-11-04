# Complete span-encoding native reference

This development tool evaluates the complete unchanged
`compiler/rustc_span/src/span_encoding.rs` at Rust revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` (source version1.101.0). Production
Go span code never invokes Rust. The surrounding lib/identity/hygiene/session
runtime is explicitly partial and earns no complete-file credit here.

## Actual source and dependency boundary

The reference compiles the original entire span_encoding, idx, fx,
int_overflow, atomic_ref and sync/lock files, plus both implementation files of
the complete real rustc_index_macros proc-macro crate. The DefIndex and
SyntaxContext definitions use their actual original newtype_index! invocations
and real macro expansion. It does not handwrite their range checks/Hash/Debug.
The lock uses the original Lock/LockGuard algorithms and the exact sync::mode
block, with its actual initially uninitialized mode conservatively selecting
synchronization. The exact DynSend/DynSync marker declarations are sufficient
for this scoped reference; no marker runtime operations or broader sync build
is claimed.

Exact byte-selected fragments provide BytePos's declaration/derive, SpanData's
declaration/derive/complete impl, LocalDefId, RawSpan, SyntaxContext root/is_root,
Span lo/hi/lo_hi, actual Span/SpanData Debug, SPAN_TRACK, scoped TLS and
with_session_globals. Fragment byte ranges and hashes identify their original
whole files. All whole originals are retained under the ignored reference
workspace; none of their bodies are patched. Additional inspection adapters are
separate files included after the original module, allowing actual interner
snapshots without exposing or replacing its implementation.

SessionGlobals is a development dependency facade containing just its real
span-interner lock and optional source-map interface. Symbols, hygiene
allocation and other session fields are absent, with no fabricated values.
The recording source-map adapter implements exactly the method called by Span
Debug, logs the original Span argument and returns caller-supplied text (or
panics). Those strings are explicit test inputs; they are not a replacement
filename/location algorithm and prove no complete SourceMap implementation.
The Go production API has the same explicit dependency operation.

The four independent compiler profiles set debug assertions and ordinary
integer overflow checks separately for the whole reference workspace. The
original DebugStrictAdd implementation consequently sees the real profile
combination. Rust1.99.1, Linux x86-64 is the recorded formatter/compiler witness
for the1.101 source. RUSTC_BOOTSTRAP enables only features needed to compile
these unchanged internal types/macros and the development custom FnOnce probe;
no original source feature/body edits are made. This is not a full pinned
rustc_span or rustc_data_structures crate build.

All selected registry archives match the original Rust Cargo.lock. Every
expanded source file Cargo compiles is compared with its exact archive before
and after builds, including normalized manifests; changed, missing, extra and
symlink files are rejected. The driver also freezes/hashes all staged reference
inputs and all tool/request-builder inputs before compilation and checks them
again after execution. All fetching is separate from the offline checker.

## Observable contracts

The independently generated Go request builder supplies inputs only. Every
expected result, panic, callback/write trace, interner row and raw encoded bit
comes from actual Rust execution. The schema is retained in SCHEMA.md. Source
and protocol errors are separate: an unclassified native panic fails generation
instead of being accepted as an expected span result.

The retained requests cover all four packing formats, boundary/invalid raw
spans, swapped/ordered constructors, absent TLS, interner insertion/deduplication
and context-free keys, tracked/untracked queries, parent replacement, context
mapping, dummy/expansion predicates, clone/default clone_from, typed Hash and
hasher panics, location-only OrdSpan equality/order and partial results. Tracking
reentry actually creates spans during the original callback, exercising lock
release and insertion order. Caught failures retain the actual partially
updated interner and slot state and subsequent operations continue.

Native Span/OrdSpan/SpanData Debug is observed with19 actual format patterns,
independent write_char overrides/default fallback, sink failures, parent query
side effects after earlier writer errors and formatter reuse. Both attempted
write kinds/text and accepted prefixes are retained, with fill/alignment state
before/after/reuse. A source-map operation is called before writes and preserves
its own panic/result order.

Owned mapping passes a real custom Rust FnOnce containing one boxed mutable
cell and a Token field. Actual call, user Drop and compiler field-drop glue
establish mutation/identity, moved capture behavior and destruction before
reconstruction. Actual outer destructor/catch and panic-hook contexts supply
std::thread::panicking observations. Double cleanup panics and panic-in-hook
failures run alone in native subprocesses and really abort. The driver retains
flushed observations, reports the native abort class and labels its last
pre-operation state as last_observed; it never fabricates postmortem cleanup or
interner state. The abort class corresponds to Rust SIGABRT and Go exit134,
without claiming equal process signal numbers.

The source's negative Ord/PartialOrd/Hash implementations remain compiled
unchanged. Span retains size8/alignment4 in the reference. This is evidence for
its logical bit/trait contracts, not a Go/Rust FFI or target-endian ABI claim.

## Commands

```sh
python3 tools/port-rust-span-encoding/generate.py -check
python3 -m unittest discover -s tools/port-rust-span-encoding -p 'test_*.py'
go test -race ./port/rust/compiler/rustc_span/src
go vet ./port/rust/compiler/rustc_span/src
```

Generation omits -check. Both modes build/run all four profiles offline and
execute the actual Go replayer. Existing gzip bytes are preserved whenever their
complete decompressed contents match. Check mode rewrites neither corpus nor
manifest. Registry expansion and source-selection integrity tests are retained
separately from the native expected observations.

FERRUGO_RUSTC/FERRUGO_CARGO/FERRUGO_GO select paths or bare PATH executables;
explicit empty/missing selections fail. CARGO_HOME and FERRUGO_RUST_SOURCE select
the already prepared registry/Git caches. Local cached toolchains are defaults,
then PATH. GIT_NO_LAZY_FETCH=1 and Cargo --locked --offline forbid implicit
network fetches. prepare.py --offline stages/verifies the reference without
claiming that merely preparing it validates span behavior. A first dependency
lock update uses --write-lock and still verifies every version/checksum against
the pinned original lock.
