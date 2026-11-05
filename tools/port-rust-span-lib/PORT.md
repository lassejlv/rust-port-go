# rustc_span/lib.rs integration work

This is a **scoped partial** implementation. Coherent native/Go validation
passes, but owned String/Path input, public-field and result contracts remain
open. See [the concrete API gaps](../../docs/SPAN-API-GAPS.md). The lib.rs
implementation receives no full-file completion credit. The compiler bootstrap does not use this library as a production Rust
frontend yet.

The source is `compiler/rustc_span/src/lib.rs` at Rust commit
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`, Git blob
`44fbbbaccd66f3428ea135720511029be547f467`, SHA-256
`9682c3b7d227039f040afff82aae4d2fd8133692bc9ef84f0974323f8a0acca0`.
The exact source remains unchanged in the shared original reference graph.
`port/rust/compiler/rustc_span/src/lib.go` is the inventory's canonical target;
the `lib_*.go` files split the implementation into reviewable groups.
Full session FnOnce consuming/current-context primaries are in
lib_session_fn_once.go and lib_remaining_context.go; their retained semantic
boundary is documented in REMAINING_CONTEXT.md.

| Original block | Go implementation |
| --- | --- |
| Spanned, respan, dummy_spanned, ErrorGuaranteed | lib_spanned.go |
| SessionGlobals and scoped-session functions | lib.go, lib_session.go, lib_session_scope.go |
| MetavarSpansMap | lib_session.go, genuine data_structures UnordMap |
| RemapPathScopeComponents | lib_filename.go, lib_flags.go, lib_source_codec.go |
| RealFileName, InnerRealFileName, FileName, FileNameDisplay | lib_filename.go, lib_enum.go, lib_source_format.go |
| SpanData and basic Span getters | lib.go, lib_span_context.go, existing span_encoding.go |
| Geometry, ancestry, expansion/edition properties | lib_span_ops.go, lib_span_source_map.go |
| Combining/metavars, marks, macro backtrace | lib_span_combine.go |
| AttrId macro expansion | lib_attr_id.go, lib_attr_step.go |
| SpanEncoder, BlobDecoder, SpanDecoder and concrete opaque implementations | lib_codec.go, lib_codec_context.go, lib_codec_owned.go, lib_codec_ref.go, lib_codec_runtime.go |
| Span/SpanData Debug | existing span_debug.go, genuine shared session/source-map runtime |
| MultiByteChar, NormalizedPos, position macro/Pos | lib_pos.go |
| ExternalSource/Kind, SourceFileLines/Diffs, metadata/locations | lib_source_metadata.go, lib_enum.go |
| SourceFileHashAlgorithm/Hash | lib_source_hash.go |
| SourceFile construction, clone, lazy lines, external loading, coordinates, normalization, widths | lib_source_file.go |
| SourceFile and field codecs | lib_source_codec.go |
| StableSourceFileId | lib_source_metadata.go, lib_source_codec.go |
| Loc, LineInfo and error derived traits | lib_source_format.go, lib_error_format.go, lib_source_errors.go, lib_value_traits.go |
| SPAN_TRACK static fn dictionary | lib_span_track.go, existing span_encoding.go |
| Full required Hasher delegation | lib_hash_primary.go |
| Enabled lib tracing sites and selected-build callsite identity | lib_tracing.go, lib_tracing_profile.go |
| Actual derived comparison defaults | lib_order_defaults.go, lib_equality_defaults.go |
| Explicit concrete source-Self byte-slice encoding strategy | lib_byte_slice_strategy.go, lib_source_codec.go |

The original `FileLinesResult` alias is represented by the idiomatic Go
`(FileLines, *SpanLinesError)` return pair in the actual SourceMap API. A nil
error selects the owned FileLines value; a nonnil error selects the error and
the unused value has no live Shared owner. No Rust Display/Error trait is
invented for source diagnostic enums that do not implement it. Public enum
constructors and borrowed projections preserve a canonical active payload.
The two DistinctSources alternatives keep an actual separate boxed pointee.
Value constructors create fresh boxes, consumed-box constructors/take methods
move the same pointee, and Clone allocates a separate payload with field
clones. Box pointer ownership is singular; a successful take consumes the old
wrapper. Identity evidence compares relations only within each process. It
observes caller-owned wrapper destruction with the boxed value still live,
and makes no Box allocator/absolute-address ABI claim.

All four original `tests.rs` tests are mirrored in `tests_test.go`.
Generic derives borrow the actual node cell. In particular, aggregate derived
Clone inherits the default clone_from replacement behavior, and derived
PartialEq inherits `ne = !eq`; neither spuriously dispatches to a node's
independently overridden clone_from or ne. Copy is a caller/type-dictionary
obligation for Copy source T, not permission to copy arbitrary owned Go
descriptors.
`PosOperations` supplies all four required associated/borrowed methods for
arbitrary implementations, including non-Copy receivers. It passes current
context on every call and borrows the actual receiver cell. Method code and
implementation metadata are borrowed static dictionaries, not owning Fn
captures. The legacy `Pos` view remains available through an ephemeral Bind.

## Runtime and dependency boundaries

Explicit `SessionScope` handles represent one source thread's scoped TLS slot;
they are not goroutine-local storage. Source borrowed references must retain
their original lifetimes and exclusivity. The scoped generic functions consume
an actual `SpanOwned` environment, including unused-callback and unwind paths.
Temporary Go pointer/slice projections are borrowed views, not independent
owners. Rust str/char domains require valid UTF-8/scalars. Unix OS paths retain
raw bytes through the real RustPath adapter rather than filepath.Clean.

`rustfmt.Context` carries explicit inherited unwind/panic-processing state and
borrowed current runtime metadata. Primary WithContext operations receive the
current caller's value; reusable codecs/iterators retain no constructor-time
Runtime scope. Fresh translated raises use Context.PanicAt/PanicFormattedAt;
caught payloads forward through ResumeUnwind without another hook. Proven
second-panic cleanup boundaries use the current context's AbortCleanup.
Source locations come only from an explicit named caller profile, with missing
locations remaining nil. Processing a
second panic aborts before source-observable local cleanup. Plain Go callbacks
are conveniences: callback-local Go defer execution is not a transparent Rust
abort-timing claim. Their component boundary guards still precede owned
argument destruction. Global hook dictionaries represent immutable static fn
pointers/borrowed metadata, not unmodeled owned closure captures.

The following are genuine **scoped used-contract dependencies**, not completed
source files or crates:

* lib_shared.go provides actual value-cell Arc ownership, independent Clone,
  final Drop, count/identity and the used unique borrow. Weak/raw-pointer and
  allocator ABI APIs are not claimed.
* lib_sync.go provides the consumed Lock/RwLock/FreezeLock operations and owned
  guard leases, dynamic synchronization modes and actual lock/freeze behavior.
* `rustc_serialize/src` provides required Encoder/Decoder dictionaries, used
  independent provided-method overrides, real primitive/LEB128 algorithms,
  memory/file opaque codecs and deterministic map/set defaults. Other crate
  APIs remain partial. MemEncoder.finish returns raw bytes without a footer;
  FileEncoder.finish writes the source footer. MemDecoder requires that footer.
  Byte-slice defaults recognize only the exact concrete MemEncoder/FileEncoder
  types. Distinct wrappers use the generic element path even when Go promotes
  an opaque marker method. Span's projections of the actual original opaque
  Self select a static strategy explicitly; an erased SpanEncoderRef caller
  passes SpanEncoderRefByteSliceStrategy to the corresponding encoding primary.
  This strategy carries no constructor-time context and is not discovered
  through another inheritable method.
* Owned opaque write_with callbacks select FnOnce; with_position selects Fn,
  even if a type has independent FnMut/FnOnce overrides. Its cursor guard is
  destroyed before the owned Fn argument. Borrowed flush strategies keep the
  actual FnMut cell and receive the current operation context.
* The partial data_structures StableHash bridge uses the real locked stable
  Sip hasher. Genuine table/UnordMap and exact std stable-sort helpers back
  metavar maps and deterministic codec ordering. No Go-map or Go-sort
  replacement is used for those observable algorithms.
* Used BLAKE3 1.8.2 and UnicodeWidthChar 0.2.2 adapters retain their separate
  exact archive/source/scalar proofs. MD5/SHA1/SHA256 use Go's standard crypto
  algorithms, with the original byte/read/owned-input control flow retained.
* The macros.rs bug_impl bridge is explicitly partial and creates no macros.rs
  completion credit. Source caller witnesses remain distinct from Go host
  convenience locations.
* `port/support/ruststr` supplies exact SDK1.101 UTF-8 range indexing and all
  six slice-error branches, including source payload kinds and actual-site
  panic behavior. It retains its own official source archive, Unicode18 and
  original native context proofs, with no standard-file completion credit.

The new whole-span profile is the official dated nightly
`1.101.0-nightly (708c94ef0 2026-11-03)`, CFG_RELEASE 1.101.0, Unicode 18.
Scoped Unicode18 char/string Debug delegates are separate from the preserved
Unicode17 helpers used by previously validated 1.99 source profiles. The older
span_encoding oracle/corpus has not been rebound or rewritten.

## Current proof and remaining work

The retained clients call the complete unchanged original crate and genuine
dependencies; they contain no copied replacement implementation of lib.rs.
The shared preparer verifies the full original graph, 108 exact registry
dependencies, all 43 upstream crate tests and a real external session smoke
before each development generator uses the returned linkage.

Current retained comparisons cover SourceFile metadata/normalization/widths/
clones/serialization, full listed Debug and Display writer failures and mutable state,
scope flag methods/iteration/formatting, primitive memory encoding, file
buffering/delayed errors, deterministic map callbacks and owned comparator
cleanup, non-Copy generic aggregate traits, scoped FnOnce sessions, constructor
inputs/errors and owned opaque callback boundaries. The generic suite includes
partial decoder construction cleanup and provided encode_def_key overrides.
Separate diagnostic-box records retain cross-enum transfer, deep clone,
default clone_from, mutation, fallible formatting and real abort observations.
The custom Pos provider proof independently exercises all four methods, real
receiver identity/interior mutation, reverse non-Copy local destruction and
inherited/processing panic behavior against an actual source-trait impl.
The four-profile fresh-panic matrix observes exact payload/Any-kind/location
metadata for public mutable SourceFile states, concrete codecs, enum tags,
path encoding, ordinary position arithmetic and Step defaults. Its original
panic hook reenters the actual lines lock, proving the assertion-condition
temporary was released before the hook; Go also checks the actual mutex at
that point before calling the same blocking operation.
The inherited concrete comparison matrix checks every derived Ord type, the
source's Eq-only types, and within-process identities for equal non-Copy
min/max/clamp inputs. It adds no comparison trait to a source type that lacks
the derive. The failure companion proves the inherited Idx::plus location and
both SourceFile vector capacity failures using safe custom Decoder returns;
element callbacks have not run when capacity overflow is raised. The decoded
vectors use the genuine scoped vector collector rather than host make.
The enabled tracing matrix compares all five Lib callsites with verbatim
native metadata and immutable per-profile registration identity, and composes
with the separately owned SourceMap profile. The byte-strategy matrix compares
actual opaque Self, generic SpanEncoder projection, and a distinct wrapper;
the 31-byte wrapper emits 31 element callbacks and grows to capacity 40,
whereas the direct opaque encoder makes no element callbacks and has capacity
32. Neither physical allocation addresses nor allocator ABI are claimed.
Separate sibling proofs
cover Symbol, hygiene, IDs, SourceMap and caching views.

Root-owned original geometry, metavar/combine and valid hygiene-wrapper
matrices now exist alongside this work; SourceMap-dependent wrapper records
are retained by the SourceMap owner. Full tool/live-check refresh, the complete
source/API audit and independent review must still close before any full-file
completion claim. This report is not that completion attestation.

Original Rust source licensing is MIT OR Apache-2.0, retained by the repository
license copies and original-reference provenance. Separate dependency adapters
retain their own archive inventories/licenses; this report grants no additional
dependency-file completion credit.
