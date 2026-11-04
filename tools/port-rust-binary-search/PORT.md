# rustc binary-search utility source port

This is the complete port of both pinned Rust source files at `2d20b42b883dcfc014f3c2c56081eeb6091c2179`:

| Rust source | Exact mirrored Go target | Git source blob | Raw source SHA-256 |
| --- | --- | --- | --- |
| `compiler/rustc_data_structures/src/binary_search_util/mod.rs` | `port/rust/compiler/rustc_data_structures/src/binary_search_util/mod.go` | `1c6e227cec40870c0288f60270f609aeb4ebdc8e` | `6ed8ff0a143e0e4badf23ccd92e483fb14597584c0665cee582882597f086fc9` |
| `compiler/rustc_data_structures/src/binary_search_util/tests.rs` | `port/rust/compiler/rustc_data_structures/src/binary_search_util/tests_test.go` | `d74febb5c0fc4a13e2530de71982520e3a944fcf` | `6439fcec412b6c59802000c946aa3cd6c493df11dcfb465cf187a5e97f6fd10b` |

All upstream executable items are covered. The primary production `binary_search_slice` API maps to `BinarySearchStorageOwnedWithContext` and its normal-context convenience. It consumes a complete owned extractor, borrows the original query and temporary key cells, and accepts the validated `IndexRaw` storage including full-width zero-sized slices. `BinarySearchSliceOwnedWithContext` projects ordinary Go slices while preserving nonempty result identity. The earlier `BinarySearchSliceWithOperators` preserves independent operators for trivially destructible keys/extractors; `BinarySearchSliceBy` and `BinarySearchSlice` provide comparator-derived and ordered-key conveniences. The test-only `Element`, `test_map`, `get_key` and `binary_search_slice_test` map to `element`, `testMap`, `getKey` and `TestBinarySearchSliceUpstream`, preserving all five original lookup assertions and stable duplicate ordering. Rust's conditional test module is represented by Go's test files.

The package is `binarysearchutil`. Production uses the Go typed-storage port and neutral panic context; it contains no process execution, Rust fallback, source-file loading or network behavior. Licenses are preserved in `port/rust/LICENSE-MIT`, `port/rust/LICENSE-APACHE` and `port/rust/COPYRIGHT`. The owned interface and all native cleanup/borrow evidence are documented in [owned/README.md](owned/README.md).

## APIs and adapters

```go
type KeyOperators[K any] struct {
    Less        func(K, K) bool
    NotEqual    func(K, K) bool
    LessOrEqual func(K, K) bool
}

func BinarySearchSliceWithOperators[E, K any](
    data []E, keyFn func(*E) K, key K, operators KeyOperators[K],
) []E

func BinarySearchSliceBy[E, K any](
    data []E, keyFn func(*E) K, key K, compare func(K, K) int,
) []E

func BinarySearchSlice[E any, K cmp.Ordered](
    data []E, keyFn func(*E) K, key K,
) []E
```

The full adapter invokes `PartialOrd::lt`, `PartialEq::ne` and `PartialOrd::le` independently through `Less`, `NotEqual` and `LessOrEqual`. It does not replace `!=` with an equality/comparison call, infer one operator from another, or call `Ord::cmp`. Rust implementations may consistently override those methods while logging, counting calls or panicking independently; those behaviors are preserved. For `[1, 2, 2, 3]` and key `2`, both original Rust and Go call `lt, lt, lt, ne, le, le`. An independently panicking `ne` unwinds immediately after the three `lt` calls.

`BinarySearchSliceBy` deliberately derives the operators from one total-order comparator: any negative value is less, zero is equal and any positive value is greater. It is a convenience adapter for default/comparison-derived operator behavior, rather than a substitute for independent overrides. `BinarySearchSlice` uses Go `cmp.Compare` for ordered keys. Its floating-point behavior follows Go's total NaN ordering; Rust primitive floats do not implement `Ord`, so that convenience-adapter extension is not claimed as a Rust primitive-float API.

Input must be sorted consistently with the extractor and comparator. The function does not sort or validate the whole slice. It performs the same lower partition, explicit equality check and upper partition as the original source. No match returns nil, representing Rust's empty slice. A match borrows the original contiguous input range. Go does not enforce Rust's shared-reference immutability statically; callers must preserve the ordering contract while searching.

The internal `partitionPoint` adapter follows the fixed-iteration probing of Rust's `slice::partition_point`/`binary_search_by`, including the final repeated probe. Substituting Go `sort.Search` would preserve normal results but change observable key/comparator calls and panic timing. Original-source traces verify both. The adapter does not claim completion of Rust's separate `library/core/src/slice/mod.rs` source file.

The search uses O(log n) extractor/comparator calls and O(1) auxiliary storage. The legacy trivial-drop conveniences retain zero internal allocation. Empty input calls neither extraction nor comparison; the owned entrypoints still destroy their consumed extractor. Owned temporaries drop after each operator, including during unwinding, and the extractor drops after the search. Primary callbacks use the explicit panic context for source failure timing; plain Go callbacks retain Go-local defer/recover behavior. Callback costs are additional to the search's own complexity.

## Original-source oracle and reproduction

The development tool reads the exact original files from the pinned Git object cache and materializes temporary reference copies. It never edits upstream tracked files. It compiles/runs the complete original Rust test, then compiles the oracle against the original module without implementation shims. Rust 1.99.1 (`58cf8f9c3`, 2026-10-13) is the recorded standard-library/probing baseline.

Ordinary Go tests need neither Rust nor an upstream checkout:

```sh
go test ./port/rust/compiler/rustc_data_structures/src/binary_search_util
```

Setting `FERRUGO_RUSTC` opts tests into live original-source compilation and exact fixture-stream comparison. `FERRUGO_RUST_SOURCE` selects a checkout, defaulting to `.toolchain/upstream-rust` relative to the repository root. Requested empty, missing or wrong-version compilers fail explicitly.

```sh
RUSTUP_TOOLCHAIN=1.99.1 FERRUGO_RUSTC=rustc go test ./port/rust/compiler/rustc_data_structures/src/binary_search_util
RUSTUP_TOOLCHAIN=1.99.1 FERRUGO_RUSTC=rustc go run ./tools/port-rust-binary-search -check
```

The generator supports `-root`, `-rustc`, `-rust-source` and `-check`. Bare compiler names resolve on PATH; relative paths resolve against the repository root. Omitting `-check` regenerates the deterministic compressed fixture and `oracle_manifest.json`. The manifest records both Git blob IDs, raw source hashes, oracle-source hash and raw/compressed fixture hashes. An existing gzip stream is preserved only after bounded decompression validates its checksum and exact equality with the newly executed Rust oracle. This permits different Go releases to verify the same committed DEFLATE encoding; check mode still compares the exact compressed-byte hash recorded in the manifest. Generation rebuilds corrupt, missing or different-content streams.

The oracle Rust key independently overrides `lt`, `ne` and `le`, makes `Ord::cmp` panic if called, and asserts that `eq` and `partial_cmp` remain unused. It records operator identity and operator-specific panic positions in addition to key/comparison traces. The oracle includes malformed/unsorted inputs and deliberately inconsistent/stateful comparator cases to verify the pinned implementation's bounded probing and panic behavior. Such cases remain outside the sorted, total-order API contract; their returned ranges are not promised as a meaningful multimap result.
