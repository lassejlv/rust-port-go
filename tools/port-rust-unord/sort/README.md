This validates a scoped supporting dependency, `port/support/rustsort`, against
the actual official Rust 1.101 nightly SDK matching the pinned compiler source.
It grants no standard-library file completion credit. The full `unord.rs` port
remains pending its other contracts.

`rustsort.Stable` implements the normal, non-`optimize_for_size` 32/64-bit
driftsort strategy. Its `Type` dictionary carries the real Rust size, alignment,
`Freeze` and `Copy` metadata; Go layout is irrelevant. `Less` temporarily borrows
the actual elements and caller's comparison state. The caller retains the owned
comparison environment and its destruction, including early/empty cases. No
additional destructible Go closure captures are permitted.

The source algorithm's scratch length matters: the complete 4096-byte stack
buffer can exceed the requested length; the 8 MB scaling threshold changes
logical versus physical merges. Stable small sort selects its `Freeze` strategy
independently of `Copy`. Hole guards restore source values after comparison
panics; the sort never destroys elements. The helper models storage in Go and
does not expose a Rust ABI or allocator protocol.

The native observer calls actual `slice::sort_by`, with no source rewrites or
replacement sort. It covers 98,720 requests and 7,978,963 comparisons, including
89,414 caught panics. Ten real type shapes include borrowed pointers, borrowed
tuples, `Cell` interior mutation, nontrivial destructors, source sizes from zero
to 4096 bytes, and different `Copy`/`Freeze` combinations. Seven distributions
and broken comparator relations exercise small sorts, pivots, recursive
partitions, sorted runs, eager fallback, physical merges and the large-buffer
threshold. Failure positions cover every comparison for small inputs and
selected positions for larger inputs.

For each request, the observer emits every compared element identity, interior
touch count, argument pointer equality, complete post-call slot state, and
destructor sequence. Python retains independent SHA256 commitments to those
original byte streams, plus their byte counts. The Go replay must reproduce all
commitments, callback counts, panic messages and type metadata. This avoids
retaining hundreds of megabytes of repetitive full traces. The manifest also
records the original native stdout digest, SDK library hashes, source-audit
hashes, and driver/generator/Go helper hashes.

`Context` retains caller-provided ambient/processing panic metadata. Translated
callback failures must call `Context.Panic` at their source site. A raw Go panic
is routed at the callback boundary before sort-local restoration, but Go-local
callback defers have already run. This helper does not implement Rust's global
panic-hook runtime. Owned comparison cleanup belongs to its source caller.

Run:

```
FERRUGO_GO=/path/to/go python3 tools/port-rust-unord/sort/generate.py --check
```

The default original compiler is `.toolchain/rust-nightly-2026-11-04/bin/rustc`;
`FERRUGO_RUSTC_NIGHTLY` may select it by PATH name or path. Explicit empty or
unavailable overrides fail. Valid existing gzip bytes are preserved. `--check`
does not rewrite fixtures or evidence; `--skip-go` only rebuilds/checks the
original reference commitments.

Verified with actual Go 1.23.12: native generation/replay, race detection
(25.507 seconds), and vet. Unstable sorting and cached-key ownership/permutation
are not implemented by this stable helper yet.
