# `rustc_index` typed storage port

This block translates the complete pinned files
`compiler/rustc_index/src/slice.rs`, `vec.rs`, and `vec/tests.rs` from
`rust-lang/rust` revision `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.
The production package uses only the Go standard library and requires Go 1.23.
The source implementations are in
`port/rust/compiler/rustc_index/src/{slice,vec,vec_iter,vec_runtime,vec_format}.go`;
`vec_nightly.go` carries the original feature-gated implementations under the
`ferrugo_rustc_nightly` build tag. These helper files remain evidence for the
two original implementation files, rather than extra upstream file credits.

The normal and tagged implementations were compared against the original
whole Rust modules across all eight feature/assertion profiles. See the [oracle instructions](oracle/README.md) and the
[reference build](reference-build/README.md) for reproducible evidence.

## Source-to-Go API

| Source surface | Go surface |
| --- | --- |
| `IndexSlice::{empty,from_raw,from_raw_mut}`, reference `Default` | `EmptyIndexSlice`, `IndexSliceFromRaw`, `IndexSliceFromRawMut`; `FromStorage` variants carry logical raw storage |
| slice `len`, `is_empty`, `next_index`, `last_index` | `Len`, `IsEmpty`, `NextIndex`, `LastIndex` |
| slice `iter`, `iter_mut`, `iter_enumerated`, `iter_enumerated_mut`, `indices` | corresponding `Iter*`/`Indices` methods and reference/mutable/index iterators |
| `swap`, `copy_within`, `get`, `get_mut`, `pick2_mut`, `pick3_mut`, `binary_search` | corresponding methods; primary `BinarySearchRef` retains the query cell |
| generic `IntoSliceIdx` and sealed `SliceIndex` dispatch | existing `IntoSliceIdx` witnesses, `TypedSliceIndexResult`, `SliceRef`, `SliceIndexer`, and `IndexSliceGetWith`/`GetMutWith`/`IndexWith`/`IndexMutWith`/`CopyWithinWith` |
| slice `Index`/`IndexMut` | `At`/`AtMut` for a typed index, `Index`/`IndexMut` for source standard index variants, generic helpers for custom conversions |
| `invert_bijective_mapping` | `InvertBijectiveMapping`, including both debug-only assertions and diagnostic Debug dispatch |
| slice `ToOwned::{to_owned,clone_into}` | `ToOwned`, `CloneInto`; clone-into reuses allocation and reserves its tail before cloning |
| `IndexVec::{new,from_raw,with_capacity,from_elem,from_elem_n,from_fn_n}` and `Default` | `NewIndexVec`, `IndexVecFromRaw`/`FromStorage`, `IndexVecWithCapacity`, `IndexVecFromElem`, `IndexVecFromElemN`, `IndexVecFromFnN`/`FromFnNOwned` |
| `as_slice`, `as_mut_slice`, `Deref`, `DerefMut`, `Borrow`, `BorrowMut` | corresponding methods returning actual borrowed storage views |
| `push`, `pop`, `append`, `resize`, `resize_to_elem`, `ensure_contains_elem`, `shrink_to_fit`, `truncate` | corresponding methods; owned factory variants preserve closure destruction |
| owned `into_iter`, `into_iter_enumerated`, `IntoIterator` | `IntoIter`, `IntoIterEnumerated`; `OwnedIterator.IntoIterator` preserves owned allocation reuse during recollection |
| borrowed/mutable `IntoIterator` for vector and slice | `Iter`/`IterMut`, and slice `IntoIter`/`IntoIterMut` |
| `drain`, `drain_enumerated` | `Drain`, `DrainEnumerated`, with owned `IndexRangeBounds` callbacks |
| `Extend`, `FromIterator`, `From<[T; N]>` | `Extend`, `IndexVecFromIterator`, `IndexVecFromArray`; array conversion allocates fresh exact-length storage and moves elements |
| `IndexVec<I,Option<T>>` methods | `OptionIndexVec`, `Insert`, `GetOrInsertWith`/`Owned`, `Remove`, `Contains`, `DebugMapViewCompact` |
| derived `Clone`, `PartialEq`, `Eq`, `Hash` | `Clone`, `CloneFrom`, `Equal`, `NotEqual`, `Hash`; `HashIndexRawRef` supplies the standard slice hash length/element sequence with independently overridable `hash_slice` |
| slice/vector/map/compact-map `fmt::Debug` | primary `FormatDebug`; `ValueOps.FormatDebug`, `WithKeyFormatter`, complete `IndexDebugFormatter` options, fallible string/character writers, pretty padding, and panic context |
| vector `unsafe impl Send` when `T: Send` | Go has no corresponding static auto-trait; lawful transfer/sharing remains the caller contract, with no extra runtime method |
| nightly `Extend::{extend_one,extend_reserve}` | tagged `ExtendOne`, `ExtendReserve` |
| nightly `Encodable`/`Decodable` | tagged `Encode`, `DecodeIndexVec`, with actual element and specialized slice/vector codec witnesses |

`Idx[I]` and every standard `IntoSliceIdx` conversion are the previously
validated full `idx.rs` port. Storage uses no separate reduced index shim.
Indices can carry slices or other metadata: there is no Go `comparable` bound.
The same lawful Rust `Copy`, `Eq`, `Hash`, and `Debug` constraints remain caller
contracts when represented as Go callback witnesses.

## Ownership and runtime contracts

`IndexVec` owns `IndexRaw`; `IndexSlice`, element pointers, and borrowed
iterators alias actual cells. Callers must respect Rust's borrowing rules:
do not grow, move, or destroy storage while a conflicting borrowed view remains
in use. Go cannot enforce these lifetimes statically. `Close` is the explicit
destructor boundary; `defer container.Close()` also preserves an active panic.
Owned iterators and drains require `Close` when they are not fully consumed.
Moving a raw allocation, array, or iterator invalidates the corresponding
source owner. Go garbage collection is not a substitute for source Drop.

`ValueOps` primary borrowed callbacks (`CloneRef`, `CloneFromRef`, `EqualRef`,
`NotEqualRef`, `CompareRef`, `FormatDebug`, and `DropWithContext`) receive actual
source/destination cells. Value-taking callbacks and string Debug callbacks
are conveniences for implementations without those borrowed observations.
Slice equality invokes element `ne`, as the source standard slice does;
`Option<T>::ne` uses the default negation of Option equality, which invokes
inner element `eq`. Custom `CloneFromRef` and element `ne` remain independent
overrides. `Copy` and side-effect-free `TrivialClone` are separate properties.
The standard hash bridge likewise has a borrowed `HashIndexRawRef` surface.

Plain factory functions represent trivial-drop Rust closures. `IndexFactory`,
`IndexFnMut`, and `IndexOnceFactory` represent owned closure environments and
include independent call/destruction operations. Their destruction happens
even when no value is requested and at the original branch-specific point.
`IndexValueIterator` includes exact/default hints, trusted-length dispatch,
for-each/fold overrides, owned residual cleanup, and an explicit owned-vector
specialization. These are trait witnesses, not a Go standard-library iterator
implementation claim.

The exposed iterator shortcuts follow the source's distinct implementations:
raw reference `Nth`/`NthBack`/`Count`/`Last` can skip without typed constructors;
mapped iterators execute each mapper for skipped and consumed elements.
`Nth`/`NthBack` borrow the iterator, whereas `Count`/`Last` consume it. Do not
reuse a consumed iterator, including after catching a consuming panic.
Enumerate's counter remains independent of its underlying position after a
skipped mapper panics. Owned Vec skipping advances the whole skipped range
before slice drop glue; Drain defaults discard each value separately.

A first panic runs required cleanup and then propagates. A panic escaping
required cleanup while another panic is active terminates the process. Go exit
134 represents this source abort class; it does not claim Rust SIGABRT or any
signal-number ABI. The policy is local to these adapters, not process-global
Go panic interception. Abort comparisons run only in isolated subprocesses.

`IndexFormatContext` distinguishes ordinary unwinding from processing an
assertion panic message. Primary translated Rust panic sites use
`context.Panic`: panic-message processing aborts immediately, before formatter
local destructors, while an ordinary panic permits local cleanup. Plain Go
`panic`/`defer` callbacks retain Go's local cleanup timing; they are convenience
callbacks rather than a transparent Rust unwinder. Standard `FormatDebugInt`
and `FormatDebugUint` carry source integer width, signed hexadecimal behavior,
padding, and writer dispatch. Arbitrary Debug implementations use the primary
formatter callback. Legacy string callbacks and the generic Go string fallback
do not promise Rust primitive escaping or formatting flag support.

## Storage domain and representations

`IndexRaw` retains a `uint` logical length for Rust zero-sized types, including
`usize::MAX`, with Rust's maximum ZST vector capacity and without a Go slice
allocation. Borrowed views share the same representative cell. A
`RustZeroSized` metadata representation must describe an inhabited source ZST
with no distinct per-element payload. `IndexRawZeroSized` and `FromStorage`
are the full-domain raw boundaries. `AsGoSlice` is an explicitly limited
projection and rejects a logical length that cannot fit Go's signed slice
length. No main operation truncates the logical uint domain to an `int`.

`ValueOps.RustSize` supplies the source layout size for allocation growth buckets
when Go metadata changes the physical size. `OptionLayout` specifies the whole
source Option layout, including niches. The ordinary bool option convenience
uses its verified one-byte source layout. These layout descriptors model
observable source capacity behavior; they do not claim Go structs have Rust
layout, alignment, niches, pointer provenance, or ABI. Nonzero allocations
remain subject to the host allocator and address-space limits.

Constructors without a config enable overflow checks and debug assertions.
`IndexStorageConfig` selects them independently; source checked arithmetic,
always-checked range operations, and release wrapping remain distinct.

## Original tests and dependencies

`vec/tests_test.go` carries all five original tests. The selected macro-generated
index is represented explicitly in Go; it does not claim a Go port of
`newtype_index!`. The reference build uses the entire actual pinned proc-macro
crate and preserves macro expansion output. It runs the original niche/layout
assertions and bidirectional stepping tests without changing their assertions.
Go tests distinguish those source ABI facts from Go representation adaptation.

The development reference also compiles the actual original serialization and
hash crates, with exact locked registry archives and source provenance. Normal
and nightly reference tests pass, including an actual codec round trip. These
dependencies and validation scaffolds earn no extra Go source-file completion
credit. Production storage depends on the Go standard library and the existing
ported `Idx` component; it does not depend on Rust, Cargo, serde, or the
development reference workspace.
