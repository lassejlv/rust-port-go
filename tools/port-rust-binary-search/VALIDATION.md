# rustc binary-search utility validation

Validated the complete `binary_search_util/mod.rs` implementation and `tests.rs` test file at Rust commit `2d20b42b883dcfc014f3c2c56081eeb6091c2179`. Exact file mappings, source blobs and SHA-256 hashes are recorded in `PORT.md` and the target package's `oracle_manifest.json`.

The original module and original upstream test compile directly under Rust 1.99.1 (`58cf8f9c3`, 2026-10-13). No Rust dependency, implementation shim or replacement algorithm is used by the oracle.

Completed validation:

- **1,558 additional native owned cases** match the primary storage and slice APIs: 526 successful, 376 catchable panics and 656 isolated aborts. These compare actual extractor/query/key cells, temporary destruction, inherited/processing panic state, mutable borrowed results and full `usize::MAX` zero-sized storage. The slice projection retains the original embedded zero-sized array address. See [owned/VALIDATION.json](owned/VALIDATION.json) and [owned/README.md](owned/README.md).
- Every original upstream test assertion passes in Rust and Go.
- **41,835 original-source operator cases** match Go ranges and exact extractor/comparison/operator traces. The comparator convenience adapter separately matches 34,536 applicable cases.
- The cases include 22,176 exhaustive short sorted-slice/target cases across ascending, descending, absolute-value and modulo-based orders; 5,460 unsorted/cyclic/stateful-order probes; 4,864 callback-panic configurations; 2,000 deterministic larger random cases; 35 signed-extreme/custom-order cases; 7,296 independent operator-panic configurations; and four explicit independent-trait trace/panic regressions.
- **5,378 callback panics**, including **1,952 operator-specific panics**, match both panic identity and the complete callback trace before unwinding. Other panic configurations deliberately trigger after the search has finished and remain non-panicking, matching Rust.
- The original Rust key overrides `PartialOrd::lt`, `PartialEq::ne` and `PartialOrd::le` independently. `Ord::cmp` panics if called, and `eq`/`partial_cmp` are asserted unused. The exact reviewer regression produces `lt, lt, lt, ne, le, le`, and a `ne` panic occurs after three `lt` calls.
- Arbitrary negative/positive Go comparison magnitudes, composite-key adapters, borrowed result identity, empty-input callback elision and zero internal allocation pass focused tests.
- Live original-source compilation/test execution and complete oracle-stream comparison pass, including under actual Go 1.23.12.
- Deterministic regeneration with `-check` passes under both actual Go 1.23.12 and Go 1.27.3. Across Go versions, valid committed gzip encodings are reused after bounded exact-payload/CRC validation; the current fixture and manifest bytes remain unchanged. Focused generator tests reject and repair invalid headers, CRCs, truncated streams, mismatched/oversized payloads and missing files.
- `go test -race -count=1` passes for the package.
- After the independent-operator fix, ten seconds of two-worker fuzzing completed **323,984 executions** without failure, comparing ascending/descending searches with a linear reference.
- Package and generator `go vet` checks pass, including generator vet under Go 1.23.12.

Representative commands from the repository root:

```sh
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test -count=1 ./port/rust/compiler/rustc_data_structures/src/binary_search_util
GOCACHE=/tmp/ferrugo-go-cache FERRUGO_RUSTC=/workspace/rust-port-go/.toolchain/rust/bin/rustc .toolchain/go/bin/go test -count=1 -run '^TestLiveOriginalRust$' ./port/rust/compiler/rustc_data_structures/src/binary_search_util
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-rust-binary-search -check
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test -race -count=1 ./port/rust/compiler/rustc_data_structures/src/binary_search_util
GOCACHE=/tmp/ferrugo-go-cache GOMAXPROCS=2 .toolchain/go/bin/go test -run='^$' -fuzz='^FuzzBinarySearchSlice$' -fuzztime=10s -parallel=2 ./port/rust/compiler/rustc_data_structures/src/binary_search_util
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go vet ./port/rust/compiler/rustc_data_structures/src/binary_search_util ./tools/port-rust-binary-search
```

These results validate the two complete utility files and their stated Go adapters. They do not claim completion of the containing rustc crate, its standard-library dependency files or the compiler as a whole. Production execution is Go-only.
