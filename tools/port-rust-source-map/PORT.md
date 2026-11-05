# Scoped SourceMap implementation

The complete source target is `compiler/rustc_span/src/source_map.rs` at
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`: 1,327 lines, SHA-256
`9fa60e859a71b59b6787028a569a17d9fa0b5f047af9bb12de14e9c34af6eeb3`.
Its 19 test declarations are translated in `src/source_map/tests_test.go`.
The external child-test package imports the actual parent implementation. The
Go-only `source_map_source_test_witness.go` forwards the original private
byte-position operation and directly constructs private expected filename
fields; it copies no algorithm and grants no additional Rust public API credit.
Coherent integration checks pass. Whole-file credit remains withheld for
owned String/Path/Cow inputs and results, as recorded in
[the API review](../../docs/SPAN-API-GAPS.md).
Neither this document nor the supporting helper files grant premature
whole-file or standard-library source credit.

| Original surface | Go implementation |
| --- | --- |
| `original_sp` | `OriginalSpan` / `OriginalSpanWithContext` |
| `FileLoader`, `RealFileLoader` | `source_map.go`, `source_map_loader.go`, scoped host adapters |
| `SourceMapInputs`, constructors, files read guard | `source_map.go` |
| registration, imports, binary/file loading, offsets, lookup | `source_map_files.go` |
| locations, doctest offsets, line queries, span text rendering | `source_map_locations.go` |
| generic `span_to_source`, external source loading, accessibility | `source_map_source.go` |
| predicates and text extensions | `source_map_extensions.go` |
| start/end/next point, character width, statement helpers | `source_map_points.go` |
| complete mutable/fallible manual Debug | `source_map_format.go` |
| `FilePathMapping`, Into/Cow conversions, remapping, Clone/Debug | `source_map_mapping.go`, `source_map_path.go`, `source_map_path_traits.go` |
| enabled tracing sites/instrumentation and immutable selected-build metadata | `source_map_tracing.go`, `source_map_tracing_profile.go` and shared tracing adapter |
| fatal stderr statement, current capture and reentrant lock | `source_map_stderr.go` and shared `ruststdio` adapter |
| binary Arc allocation preflight | `source_map_binary_layout.go` |
| owned environments/results, cleanup and I/O error traits | `source_map_owned.go`, `source_map_io_error.go` |
| ordinary source raises and profile-selected metadata | `source_map_panic.go` |

Public methods have explicit `WithContext` forms. Constructors accept the real
SourceFile/filename/position/hash/session implementations from the lib owner.
No dummy session or source file is used. Files have independent Shared owners
in the monotonic list and stable-ID map, plus each returned handle. The files
read guard borrows actual cells; cloning a returned owner is explicit. A
SourceMap must not be copied after its mutex has been used.

`SpanToSourceOwned` and the owned predicate/conversion entry points consume a
`SpanOwned[E]` environment, borrow its actual state for calls and destroy its
complete captured state at the source point. The result is an explicit owning
handle. The callback and all captured destructible fields belong in the
witness; a hidden extra Go closure destructor is not inferred. Plain functions
are conveniences for trivial captured destruction. In particular, a prepared
return value is leaked if a normal environment destructor panics at the source
return-place boundary. Borrows cannot escape their Rust source lifetime.

`FileLoaderOps[E]` borrows the same actual E cell in all four methods and its
complete destructor; providers uphold the shared-receiver and Send/Sync laws.
`SourceIOErrorOps[T]` represents a complete io::Error, including the variant's
Debug/Display/source and destruction, rather than an arbitrary inner Error.
Native OS errors, simple static messages, ErrorKind errors and owned strings
have concrete scoped constructors. OS-message Debug quotes raw OS formatting
without applying str Debug escaping, as the actual core Repr implementation
does. Standard error/container helpers here receive no whole dependency-file
credit.

Runtime Context borrows the current caller's tracing Scope and optional panic
operations. Owning values scrub this runtime pointer rather than capture the
constructor's caller. Later Drop/format/callback operations receive their own
current Context. Ordinary source raises dispatch the supplied hook before
unwinding; formatted expect failures start processing before Error.Debug and
invoke the hook only after rendering. Recover forwarding and fatal marker
raises resume without invoking the hook again. A second panic escaping a
required local cleanup invokes the explicitly selected compiler cleanup-abort
operation. Processing-panic raises abort before local cleanup. Host exit134 is
the Go abort representation; it is not Rust signal-number ABI identity.
Translated callbacks use the supplied Context at actual panic sites. Plain Go
panic/defer callbacks keep Go local cleanup timing and are conveniences, not
transparent Rust panic operations.

Panic filename spelling is never inferred or silently normalized in production.
`PanicOperations.LocationCells` is the borrowed-cell primary; `Locations` is
the value-copy convenience. Both are caller-owned immutable selected-profile metadata,
keyed by semantic operation site. Missing sites are honestly unlocated. Native
hook proofs explicitly supply the actual compiled `original/compiler/...`
spelling. The standard cleanup-abort metadata is separately pinned to the
actual selected SDK. `NewSourceMapTracingProfile` owns stable callsite cells
for explicitly supplied verbatim build metadata. `WithSourceMapTracingProfile`
borrows that profile through the caller's runtime metadata, preserving tracing,
panic operations and other components' profiles. Its semantic IDs distinguish
instrument spans from return events sharing the same line. The explicitly named
logical-compiler-path convenience is kept separate from native verbatim proof.
Custom compiled spellings are never silently rewritten. The composed proof
also selects the lib owner's independent tracing profile.

RustPath retains native raw Unix bytes or canonical Windows WTF-8, including
unpaired UTF-16 surrogates. Owned clones copy nonempty backing; borrowed views
and Hash/Debug callbacks retain source-buffer identity. Cow's borrowed/owned
choice is explicit. No filepath.Clean/Join substitute is used. Unix error
formatting now delegates to the accepted scoped `rusterrno` helper's actual
host XSI strerror_r binding. Without cgo, `RealFileLoader.UnixErrorBinding`
supplies the complete explicit operation; no inferred errno text is used.
Formatting runs lazily with the later caller's runtime Context. Its independent
native proof covers 84,123 errno/CStr writer cases, and SourceMap retains all
4,355 Linux/GNU cases plus explicit no-cgo binding/context tests.
The Windows backend uses the actual Win32 UTF-16
path normalization/open/exists/current-directory/error-format protocols; its
current comparisons are both binaries under Wine on Linux, not a Windows
kernel validation. Native Windows CI remains required for that host claim.

The native SourceMap reference always uses the entire unchanged original crate
through `tools/port-rust-span-reference/prepare.py`, with official matching
nightly 2026-11-04 (`708c94ef093ecc42fb8851e17d8383d999727abc`, std Unicode18).
`reference.py` validates the selected executable, original Git blobs, all
locked registry archives and their extraction, the original tests and linked
artifacts on every invocation. Compiler overrides preserve Rustup's invoked
executable name and reject mismatched SDKs. Native subprocesses have finite
timeouts and abort cases require actual SIGABRT.

| Retained proof | Cases / steps |
| --- | --- |
| entire original SourceMap algorithms, four independent build profiles | 514,300 records |
| enabled subscriber/instrument/event timing and field writers | 3,072 scripts / 123,168 events |
| verbatim selected SourceMap/lib tracing metadata, distinct span/return cells | 3,840 scripts / 153,024 events |
| SourceMap Debug writers, failures, mutable flags and same-Formatter reuse | 8,640 scripts |
| public safe import-overflow hook formats the same locked SourceMap | 720 scripts |
| owned custom loader/Error expect path, normal/inherited/processing | 96 scripts / 88 actual SIGABRT |
| expect Error.Debug returns fmt::Error after partial output | 16 scripts / 8 actual SIGABRT |
| owned generic extraction, return-place/guard/Arc/runtime behavior | 256 scripts / 96 actual SIGABRT |
| owned while/previous/take predicates across all panic contexts | 768 scripts / 288 actual SIGABRT |
| custom consumed Into/Cow path conversion and environment destruction | 512 scripts / 256 actual SIGABRT |
| borrowed predicate/environment temporary cells, Drop and early returns | 768 cases |
| internal source panic kind/payload/UTF-8 boundaries/explicit locations | 1,648 records / 594 caught panics |
| safe metadata-only full-u32 positions/import arithmetic and source panic sites | 784 records / 108 caught panics |
| imported metadata length differs from hash-matching external text | 1,600 records / 234 caught panics |
| public get_source_map None/Some, actual identity and Arc owners | 4 profile records |
| get_source_map without scoped session | 4 profile records |
| genuine StableSourceFileId assertion expansion (compositional) | 4 profile records |
| fatal stderr writes/errors, current capture/preheld lock and hooks | 160 scripts / 80 actual SIGABRT |
| actual SDK Arc constructor/public Layout boundary operations | 74 records |
| enabled subscriber during owned callbacks, hooks and cleanup/processing panic | 96 scripts / 88 actual SIGABRT |
| genuine Span wrappers, local/imported files and local/foreign expansions | 2,400 graphs / 350,400 steps |
| std Unix Path/OsStr, actual borrowed buffers and complete formatter calls | 42,390 records |
| Linux/GNU OS error code/kind/Debug/Display | 4,355 codes |
| actual Linux oversized sparse files, metadata rejection before reading | 2 records |
| Windows std Path/OsStr: actual Rust and Go PE binaries under Wine | 110,596 records |
| Windows std io::Error vs actual whole Go span backend under Wine | 42,224 records |
| Windows native filesystem host contracts vs actual whole Go backend under Wine | 151 records |

The locked Debug proof needs no private observer. Public imports register a
zero-length file at0, then a u32::MAX-length metadata-only file at1. Registering
a third file overflows end_position while the actual files write guard is held
in checked profiles. A real panic hook formats that same SourceMap through a
safe thread-local Arc, observing `<locked>`. Unchecked profiles also retain
construction outcome and post-operation file count.

The Span-wrapper proof calls the original `is_visible`, `in_external_macro`,
`find_ancestor_not_from_extern_macro` and `within_macro`. It uses actual foreign
expansion registration, real SourceFile constructors/imports and true metavar
insertion/read/freeze behavior. It compares parent-tracking callbacks, loader
read counts, accessibility and Arc counts, without production lib edits.

The translated upstream tests retain their original Windows `path_str`
conversion, including the selected X: root. All applicable declarations pass
in the actual Go PE package under Wine alongside the 151 filesystem records.
The Windows ErrorKind table has a deterministic generator that verifies the
official SDK source archive and independently cross-checks every used constant
against the root-locked windows-sys archive.

The fatal statement now uses the accepted shared `ruststdio` default or a
complete explicit binding. The shared lock covers all three argument writes,
and is released before I/O error rendering and the panic hook. A binding may
resolve the actual later caller's source-thread scope; no component-local
mutex or fabricated global TLS is used. Native cases install output capture
and prehold the reentrant lock after SourceMap construction. Closed stderr and
EBADF suppression, unknown error text and processing/
ambient unwind behavior retain actual source hooks and callback order. Partial
formatted-panic output is preserved if Error.Debug returns fmt::Error; a new
panic during formatting still aborts immediately.

The Arc preflight preserves source usize casting, array Layout validation and
header extension/alignment errors before allocating. Actual large invalid
constructor requests and exact public Layout operations cover the boundary;
valid huge allocations are never attempted. The public-layout header shim is
explicitly two AtomicUsize counters plus unit, matching the selected SDK; it
grants no Arc source-file credit. Logical 32-bit arithmetic is tested separately
from native 64-bit execution.

Remaining acceptance work is independent integration review and authoritative
native Windows host execution. String slicing delegates to the independently source-verified exact SDK1.101
`port/support/ruststr` helper; the original SourceMap composition data and caller
semantic sites remain unchanged.
This work does not claim a completed compiler frontend or source-file credit
for unported standard Vec/path/I/O, tracing or unicode-width internals.
