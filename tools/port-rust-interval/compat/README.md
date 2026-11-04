# Original interval ownership and formatting comparison

This additional oracle retains 22,000 original-source records and 106 isolated
abort comparisons. It preserves the existing 40,380-record interval corpus
and its compressed bytes. All four overflow/debug-assertion combinations are
compiled independently, including the actual original index storage and
SmallVec dependency.

```sh
go test ./port/rust/compiler/rustc_index/src/interval
go test -tags=ferrugo_rustc_nightly ./port/rust/compiler/rustc_index/src/interval
go run ./tools/port-rust-interval/compat -check
```

Omit `-check` to regenerate original output. Go 1.23 is the minimum generator
and test version. `-root` selects the repository; `-rustc` or
`FERRUGO_RUSTC` selects the compiler, including a bare name on `PATH`.
`FERRUGO_GO` optionally selects the Go compiler for the isolated live witness.
Ordinary tests require no Rust, Git, registry, or network access.

Prepare the pinned SmallVec archive with the original tool's explicit
`-fetch-dependency` option when needed. The additional tool reads
`.toolchain/port-rust-interval-deps/smallvec-1.15.1.crate`, verifies its exact
pinned root Cargo.lock checksum, and extracts the actual crate without
substituting implementations. It copies unchanged full Git-object bytes for
`idx.rs`, `slice.rs`, `vec.rs`, the 464-line `interval.rs`, and the 199-line
`interval/tests.rs` from Rust pin
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`. All seven original tests pass in
each profile. The source 1.101.0 development version and available reference
compiler 1.99.1 are explicitly recorded; `RUSTC_BOOTSTRAP=1` accesses the
existing step/formatting-options feature gates only in child builds.

Formatter cases compare primitive and metadata-backed index types, empty and
populated interval lists, full-width domains, fifteen styles, every relevant
fallible writer position, independently overridden character/string writes,
callback panic cleanup, reused mutable formatter state and canonical
PhantomData names. No key Debug implementation is invoked for PhantomData.
Numeric pair/domain formatting carries caller radix, width, precision, fill,
alignment, sign and zero padding through the original tuple/list/struct
builder behavior.

Owned range cases compare clone, cloned start, index conversion, cloned range
destruction, original end, end conversion and original destruction separately.
The five set methods and two matrix methods preserve that sequence, including
empty bounds, distinct clone values, failed clones/reads/conversions/destructors,
matrix row growth and failure after partial growth. The custom native range
owns its destructor; no fake generic range or patched source algorithm is
used. Its index keys are lawful Copy types with slice-backed metadata.

Primary context-aware range callbacks receive `IntervalPanicContext` and the
context-first `*WithContext` entrypoints pass inherited source panic state.
The neutral context distinguishes ordinary unwinding from processing a panic
message. Native cases execute both from actual Rust destructor unwinding and
from an actual panic hook. Ambient unwinding alone permits an internal catch;
a panic escaping cleanup for another local panic aborts. Translated panic
sites call `context.Panic` to preserve immediate processing-panic abort timing
before callback locals are destroyed. Default/plain callbacks represent normal
context conveniences and retain Go local panic/defer behavior.

Internal arithmetic/storage failures are intercepted by component boundary
guards, not contextual panic calls at every internal site. For the pinned call
graph, source review establishes that no observable inner cleanup occurs before
the guard, and native processing-panic edge cases verify that owned ranges are
not destroyed. A new inner destructor, lock release or other observable cleanup
would require a fresh audit of this boundary.

All 106 required aborts execute isolated Rust and Go children; no intentional
abort runs in the main test process. The Rust child's core limit is disabled
locally. Go exit 134 normalizes to the native abort class; no signal-number,
core-dump, or Rust ABI promise is made. Every event before termination must
agree, including events omitted after an early abort.

`manifest.json` binds source hashes, exact dependency bytes, compiler profiles,
request/replay/witness inputs, normal corpus hashes and abort records. `-check`
rebuilds the originals and compares fresh source output plus normal and
isolated Go replay. Existing gzip bytes and historical Go version metadata are
retained when decoded records match, allowing later Go versions to check
artifacts produced with Go 1.23. The small shared formatting/runtime adapters
and the dependency reference builds earn no extra upstream source-file credit.
