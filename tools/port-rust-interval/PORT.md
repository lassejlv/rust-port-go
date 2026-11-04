# Complete interval-set and sparse-matrix source port

Pinned upstream: `rust-lang/rust` commit `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.

| Whole source file | Go destination | Git blob | Raw SHA-256 |
| --- | --- | --- | --- |
| `compiler/rustc_index/src/interval.rs` (464 lines) | `port/rust/compiler/rustc_index/src/interval.go` | `475c81e66ab09229137350775db615e39df24eee` | `1f1e094a6d260ebd3ffd00309a324d4c2ab93620463c6fb6dec5ba22715da3bc` |
| `compiler/rustc_index/src/interval/tests.rs` (199 lines) | `port/rust/compiler/rustc_index/src/interval/tests_test.go` | `cf3222e6c6572303939f620858a4d98a90ba2bbe` | `a2e057843e58fbee1b3057fe09bd25f70c64953701d7d60883d3aa0a4f87a2e2` |

Both sources are MIT OR Apache-2.0; the notices and license texts are preserved under `port/rust/`. These two mappings do not establish completion of `rustc_index`, the compiler parser or Rust compilation.

`IntervalSet[I any]` ports every source method and private algorithm: construction, clear, element and interval iteration, insert and append of individual points/ranges, membership and intersection, superset/disjoint queries, empty queries, first-unset/last-set queries, insert-all, union and invariant checking. Its derived `Clone` and `Debug` are represented by `Clone`, default clone-then-replace `CloneFrom`, and primary `FormatDebug` with `Debug`/`fmt.Stringer` string conveniences. The derived debug representation streams the private SmallVec pair list, domain and PhantomData type name with complete caller options and independent string/character writes. `IntervalStep.TypeName` is the canonical source type-name witness; the port never substitutes Go reflection spelling. The built-in factories supply `u32` and `usize`.

`SparseIntervalMatrix[R,C any]` ports its constructor, row indices, row lookup, enumerated iteration, row clearing, internal growth, union of a supplied row or two stored rows, row disjointness, whole-row insertion, point/range insertion and append, membership and derived cloning. Rows use the actual source-ported `IndexVec[R,IntervalSet[C]]` from `vec.go`, including its growth and disjoint mutable-reference operations. The element `ValueOps` performs deep interval-set cloning; these values are neither Copy nor TrivialClone. There is no replacement matrix storage implementation in this port.

The private `SmallVec<[(u32,u32);2]>` becomes a Go slice of the same inclusive u32 pairs. This replaces its inline-allocation optimization while retaining values, ordering, deep cloning and source failure behavior. SmallVec's own source is not being claimed as ported. Native machine-width `uint` represents Rust `usize`; narrowing casts to u32 and the source's checked arithmetic remain explicit.

## Public generic contracts

`Idx[I]` is reused from the existing complete index-trait port. Key types remain `any`: slice-backed reference metadata is supported, and no Go `comparable` constraint is introduced.

`IntervalRangeBounds[I]` adapts arbitrary implementations of Rust's `RangeBounds` plus `Clone`. `CloneFn`, `StartBoundFn` and `EndBoundFn` are independent normal-context callbacks. Primary `CloneWithContextFn`, `StartBoundWithContextFn`, `EndBoundWithContextFn` and `CloseWithContext` receive the explicit `IntervalPanicContext` (the neutral `rustfmt.Context`); `Close` is the normal destructor convenience. Plain primitive range factories have trivial destruction. A shallow default clone is a trait witness, not automatic cloning of owned resources; custom owned types supply their complete clone and destructor behavior. The source order is preserved: clone the supplied range, read its cloned start bound, convert its start index, destroy that cloned range, then consume the original to read and convert its end bound, and destroy the original before the remaining interval algorithm. Clone, start/index and clone-destructor failures clean the still-owned original exactly once. All five set range methods provide context-first `WithContext` variants; matrix range insertion and append own the argument before row growth and transfer it only after the row succeeds. A clone may return a distinct implementation. Included, excluded and unbounded endpoints remain separate variants until the source algorithm converts them. Factories cover half-open, inclusive, open and full ranges, arbitrary bound pairs and existing standard/core inclusive values. Exhausted standard inclusive ranges return an excluded start as their end bound, matching Rust's range-bound API. Factories accept key-clone callbacks where standard ranges clone their endpoints.

`IntervalStep[I]` supplies Clone, equality, partial comparison, independent relational-operator overrides and the Step operations. It represents all five required Step callbacks (`steps_between`, both checked steps and both overflowing steps), the ordinary forward/backward defaults and independent overrides of those and their unchecked counterparts. Unordered partial comparisons are retained. A nil clone callback denotes a Copy-like value copy; a missing callback for an operation that actually needs it fails instead of silently substituting integer behavior. The native u32/usize factories supply their corresponding arithmetic, including checked and unchecked profiles.

These descriptors bind the traits consumed by the original file; they do not constitute a source port of the standard library's Step, RangeBounds or Iterator implementation files. Use consistent lawful type-trait providers when interacting sets represent one Rust key type. As with the existing `Idx` adapter, Go does not statically enforce Rust's trait laws or borrow lifetimes.

`IterIntervals` constructs typed endpoints lazily and advances its backing slice cursor before conversions. `Iter` flattens inclusive ranges using the supplied stepping behavior. Their `Next`, `SizeHint` and Go sequence adapters preserve lazy consumption; the differential driver also catches iterator panics and resumes the same iterator. Matrix iterators delegate to the actual typed storage implementation. Shared views must follow Rust's borrowing rules, and independent ownership copies must use `Clone` rather than copying a Go structure containing slice metadata.

## Arithmetic, assertions and source quirks

`IntervalConfig{OverflowChecks,DebugAssertions}` models independent Rust build options. Ordinary constructors enable both. Always-on source assertions remain always on. Debug invariant checks run only in their selected profile, after the same mutations as the original source. The matrix passes matching options to its underlying typed vector.

The port retains several observable details:

- Partition-point predicates evaluate u32 additions inside each actual probe, including Rust's repeated final probe. Moving an addition outside the predicate would change empty-list failures.
- Insertion can mutate the list before a debug assertion panics. Insert-all clears the previous list before checking whether the domain end fits u32. Matrix growth can complete before a later range, domain or disjoint-reference failure.
- Append preconditions, cast truncation and overflow failures follow the original order. Matrix disjointness performs both row lookups even when the first row is missing. Missing-row membership skips the column callback.
- Union first checks domains. When the receiver has fewer intervals, it clones the receiver, copies the other interval list and recursively inserts the backup. Its returned bool can therefore be false even though the receiver changed, such as an empty receiver unioned with a nonempty set. The port preserves that result.
- Row equality is independent of index conversion. Distinct keys mapping to one storage index can pass the equality check and subsequently fail the disjoint mutable-reference operation.

Panic success/failure, post-failure state and callback order are compared; diagnostic spelling differs between Rust and Go. The primary formatter preserves full flags, mutable option state, writer dispatch, errors, pretty indentation and reuse after failures. It does not add Debug to SparseIntervalMatrix, whose source derives only Clone.

Owned range operations distinguish ambient panic state from a locally escaping panic. An ambient unwind alone remains catchable inside a destructor; a panic escaping required local cleanup during another local panic aborts. The component-local policy uses Go exit134 as the source abort class, without claiming signal ABI or changing Go's global panic behavior. Translated Rust panic sites use `context.Panic`, which aborts immediately while processing a panic message. Plain Go panic/defer callbacks remain conveniences with Go local cleanup timing. Processing-panic tests and double-panic cleanup comparisons run only in isolated child processes.

Internal arithmetic and storage failures use component boundary guards rather than `context.Panic` at every failure site. In the pinned call graph these guards run before any observable owned-range cleanup; the intervening storage factory has trivial captures and the stored interval rows have no user destructor. Native processing-panic probes and independent source review cover this boundary. Adding an observable inner destructor, lock release or other cleanup requires revisiting it before extending the same guarantee.

## Original-source oracle and profiles

The oracle copies `interval.rs`, its complete test file, and the actual original `idx.rs`, `slice.rs` and `vec.rs` directly from verified pinned Git objects. All bytes remain unchanged. A small module/re-export wrapper compiles the three original storage files as a real dependency library. The interval client imports that library and actual SmallVec 1.15.1, whose archive checksum comes from the pinned `Cargo.lock`:

`67b1b7a3b5fe4f1376887184045fcf45c69e92af734b7aaddc05fb777b6fbd03`.

There is no fake vector, SmallVec shim or patched algorithm. The dependency uses its default feature set. SmallVec is downloaded only when `-fetch-dependency` is explicitly requested and the archive is absent; original Git objects are never fetched implicitly.

Both the original storage dependency and interval executable are rebuilt for each of four overflow-check/debug-assertion combinations. The oracle uses Rust 1.99.1 (`58cf8f9c3`, 2026-10-13) on x86_64 Linux. `RUSTC_BOOTSTRAP=1` enables its real `step_trait` API only in validation compiler processes. This compiler revision differs from the source revision; the recorded validation profile is explicit and does not claim to be an entire compiler built from the source pin. These files compile unchanged on that profile, so no rustc-dev installation or substitute private types are required.

The original interval file contains no cfg-dependent codec implementation or other conditional algorithm. Its parent upstream `lib.rs` exposes the module only with `feature="nightly"`; that crate-root file is not yet ported. The Go interval implementation is available in both current Go build profiles. The oracle's storage dependency uses the original non-nightly storage configuration: it does not exercise storage codecs, which are the separate whole-`vec.rs` port's obligation. No source-port completion credit for those files is earned here.

The tool executes all seven original tests separately. The unchanged original fixture records 40,380 original-source comparisons: results or panics, complete generic callback traces and final set/matrix state. Cases include every method and bound kind, all four arithmetic/assertion profiles, high machine-width inputs, u32 boundaries, custom range clones, non-comparable keys, lazy iterator resumption and size hints, row growth and aliasing, and 384 deterministic stateful random programs. The manifest checksums the source bytes, dependencies, wrapper, requests, responses and compressed/raw fixture. Ordinary Go tests are offline and require none of Rust, Git or network access.

```sh
go test ./port/rust/compiler/rustc_index/src/interval
go test -tags ferrugo_rustc_nightly ./port/rust/compiler/rustc_index/src/interval
go run ./tools/port-rust-interval -fetch-dependency -check
FERRUGO_RUSTC=/path/to/rustc go test ./port/rust/compiler/rustc_index/src/interval
```

Omitting `-check` regenerates the fixture and manifest after comparing every answer with Go. `-root`, `-rustc`, `-rust-source` and `-smallvec-archive` select local paths. Compiler names resolve on PATH; relative compiler paths resolve against the repository root. `FERRUGO_RUSTC` and `FERRUGO_RUST_SOURCE` provide optional defaults. Explicitly opting into an empty, unavailable or different-version compiler fails. Live tests require the checksummed dependency archive to be prepared explicitly beforehand.

Production imports only Go standard-library packages and the shared Go typed-storage implementation. It never reads fixtures, invokes Rust or accesses the network.


## Additional ownership and formatter evidence

[`compat/README.md`](compat/README.md) documents the independent additional
22,000 original-source records and 106 isolated abort comparisons across four
separately compiled overflow/debug profiles. These use the whole original
464-line interval source, its 199-line tests, original typed storage, and the
actual pinned SmallVec archive without body patches or container shims. All
seven original tests also run in every added profile. The original 40,380
records and compressed bytes remain intact. Added production helper files
`interval_format.go` and `interval_owned.go` are evidence for the same single
implementation file, not additional source completion credits.
