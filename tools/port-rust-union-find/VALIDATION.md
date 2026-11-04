# Union-find validation

Validated with Go 1.23.12 and pinned
`rustc 1.99.1 (58cf8f9c3 2026-10-13)` on the 64-bit Linux host.

- Both original upstream Rust unit tests compile and pass unchanged.
- Both directly ported Go test functions pass.
- 1,894 requests run in each of the original Rust debug and release profiles,
  producing **3,788 original-source records** with no Go mismatches.
- Each record compares operation results, the complete parent/rank table's
  derived debug representation after every operation, and every `Idx::new`,
  `Idx::index`, `PartialEq::eq`, and `PartialEq::ne` callback in source order.
- Cases cover empty forests, every pair of valid/invalid keys for sizes 0..6,
  transitive/rank-balanced forests, same-set merges, exact path compression,
  snapshots and independently mutable snapshot storage, typed-index validation
  failures, and 256 deterministic random 48-operation sequences.
- Callback panics are injected independently at the first 120 calls of each
  of the four callback kinds. Operations continue after recovery, comparing
  partial mutation, callback traces, and subsequent forest behavior.
- The constructor and snapshot tests directly verify the upfront length-index
  check and ascending per-key construction order. The checked u32 adapter is
  tested at an index beyond `u32::MAX` before any forest allocation.
- Every oracle key contains static slice metadata; the Go key contains a
  `[]byte` field and is non-comparable. All requests are repeated with both an
  overridden `PartialEq::ne` and the actual Rust default `!PartialEq::eq`.
- Source pin/blob verification and `-check` fixture regeneration pass. The
  manifest records all source, oracle, request, response, and fixture hashes.
- Go package tests and `go vet` pass using the actual Go 1.23.12 binary.
- The two built-in adapters delegate to the separately validated typed-index
  component's `UsizeIdx`/`U32Idx` methods. Focused union-find oracle tests and
  both standard/nightly typed-index tests pass after integration.

The oracle's faithful, scoped `IndexVec` shim is detailed in `PORT.md`; it is
not a full port of the index container. Panic classes, mutation, and callback
order are compared; Rust and Go runtime panic message spelling is not equated.
