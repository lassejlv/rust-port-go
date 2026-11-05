This handoff is ready for independent source/API review; it is not a ledger
attestation or a production compiler claim. Source and dependency boundaries
are in PORT.md, with exact identities and per-client manifests in
VALIDATION.json.

Twenty-one owned clients link the complete unchanged original rustc_span crate
and its genuine dependencies. Their live `--check` commands use the official
matching 1.101 nightly. They retain 180,454 observations:

| Comparison | Original observations |
| --- | ---: |
| SourceFile metadata, normalization, widths, clone, codec | 8,232 |
| Debug writer calls, failures and mutable formatter state | 63,164 |
| Display writer calls, failures and mutable formatter state | 12,504 |
| Non-Copy Spanned, codec dispatch, scoped sessions | 188 |
| Inherited/processing session FnOnce | 72, including 42 actual aborts |
| Session constructor inputs/error/loader cleanup | 22, including 8 actual aborts |
| Flags methods/iteration/operations | 65,807 |
| Explicit radix formatter operations | 7,528 |
| Primitive memory codec | 3,256 |
| File codec in four profiles | 332 |
| Deterministic map callbacks/sort/owned comparator | 768 |
| Owned opaque callbacks | 62, including 4 actual aborts |
| Owned AsRef/Read hashing | 28 |
| Fresh panic payload kind/location, arithmetic and malformed metadata | 256 |
| Diagnostic Box ownership/identity | 40, including 5 actual aborts |
| Arbitrary non-Copy Pos implementation | 15, including 4 actual aborts |
| Actual derived/inherited comparisons and equal-owner selection | 14,104 |
| Capacity, inherited Idx and clamp failures | 84, including 20 actual aborts |
| Verbatim enabled tracing metadata and callsite identity | 192 |
| Concrete/alias/source-Self/wrapper byte-slice specialization | 280 |
| Full consuming session FnOnce/current public routes | 3,520, including 1,324 actual aborts |

The panic client uses all four independent debug/overflow profiles and retains
both explicitly normalized reference filenames and original native filenames.
Default production contexts invent no locations. A native hook safely reenters
the original source-file lines lock after the assertion's condition temporary
has ended. Go observes the real lock at the same point and then performs the
same blocking public operation. A custom safe Decoder's short raw-byte return
also exercises lazy-difference assertion formatting and retained failure state.

The Box client compares only within-process identity relations. It proves
cross-enum transfer preserves the pointee; Clone/clone_from create distinct
payloads and independent nonempty strings; mutation reaches the actual cell;
and owned wrappers still observe the live payload during ordinary/unwind
destruction. Processing-panic records are actual native aborts and retain
events only. No cross-language addresses, allocator ABI, post-abort state or
transparent raw-Go callback-defer timing are claimed.

Pos compares the actual source trait implemented by a non-Copy type with
interior state. All four methods dispatch independently, receive the actual
receiver cell, and preserve current ordinary/inherited/processing state.
Normal and failed conversions retain reverse local destruction. The Go
primary receives Context on every method, with no constructor-time Runtime.

The trait matrix checks all actual derived Ord types and the applicable
Eq-only types. It does not invent ordering for Eq-only source types. Equal
owned filename min/max/clamp choices are compared with within-process labels,
without comparing language allocation addresses. Safe custom Decoder values
exercise both SourceFile Vec length overflows before element callbacks at
positions 42/59; the inherited AttrId Idx::plus failure reports the idx.rs
site rather than the distinct macro Add site. Normal, inherited and processing
contexts retain exact native panic kinds/locations and real aborts.

The tracing matrix retains verbatim actual-build Name/File/Target/ModulePath
and line metadata for all five Lib sites, including the unavailable-source
branch. The primary immutable profile is selected before registration and
filtering, preserves callsite cell identity, and composes with the independent
SourceMap profile without replacing the caller's panic/tracing metadata.
The byte-slice records prove nominal wrapper types do not inherit an opaque
specialization: a 31-byte wrapper emits 31 EmitU8 callbacks and has capacity
40, while the actual opaque source Self emits none and has capacity 32.
Source Self adapters pass an explicit non-inheritable strategy dictionary.

Actual Go 1.23.12 affected checks pass, including the original four lib tests,
the owned native clients, race and vet. The refreshed broad TestLib race after
the comparison/tracing/byte closures took 20.722s; the earlier changed
Box/format/panic group took 11.081s and Box/Pos took 1.091s. Refreshed serializer
native race took 1.509s. Go 1.27 ordinary affected
span/serializer checks also passed in 3.863s/0.331s. Eleven
tool integrity tests pass, including all 22 owned/separately owned client CLIs'
rejected empty compiler
selection preserving all evidence, fresh reference verification rather than
trusting cached commands, and bounded gzip/CRC/symlink checks.

Root-owned geometry/hygiene-wrapper matrices and sibling SourceMap, Symbol,
ID, analyzer and tracing comparisons remain separately owned. The implementation
still requires coherent final integration and independent acceptance before
any full-file completion credit.

The independent MacroBacktraceIterator context companion is also separately
owned: 240 actual-source records include 72 native aborts, current-context
switching, retained iterator state after a tracking panic, and real expansion
Arc ownership. Its generator participates in the CLI integrity loop; its
observations are not included in the 180,454 owned total above.

The final consuming-session companion is in REMAINING_CONTEXT.md. It covers
all six source session FnOnce entry points through genuine consuming owners,
not just borrowed callback bodies. Transparent envelopes reuse the established
source guards; early/late Self consumption, unused callbacks, constructor
failure, complete field cleanup and prepared-return leakage retain their real
source behavior. The nine additional public context routes now accept the
current caller witness. Canonical tests_test.go holds all four original child
unit tests without a shim or changed assertion body.

The fresh environment-specific producer and fail-closed parent/child selector
passed all3,520 records/1,324 actual aborts under actual Go1.23 in12.839s.
Two isolated Go selector tests and four Python producer tests passed. Old
SpanEncoding15,264/142,168/64 and ambient-Debug1,344/140/48 native --checks,
including their actual Go replays, pass after the scoped-TLS raise correction;
all old compressed/raw fixture bytes are retained. Historical remaining-context
raw bytes are also unchanged. Verbatim producer locations require --output-dir
on other machines, never filename normalization or historical replacement.

Root owns the final isolated coherent-snapshot binding refresh and race gates.
Live future IndexMap/rustvec files are intentionally not pulled into that
snapshot by this owner report; no broad binding was bypassed or hand-patched.
