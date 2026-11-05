# Pinned Symbol and edit-distance port

This scope implements `compiler/rustc_span/src/symbol.rs` and the complete
`edit_distance.rs` at Rust `2d20b42b883dcfc014f3c2c56081eeb6091c2179`, including
their original child tests. Symbol remains scoped partial because its
owned String result contract is open; [the API review](../../docs/SPAN-API-GAPS.md)
withholds whole symbol.rs credit. `symbol_predefined.go` is deterministic generated
data for the complete macro invocation. Production code is Go 1.23 and its
standard library; Rust is used only by development reference tools.

The default compiler-build profile is `CFG_RELEASE=1.101.0`: 2,421 predefined
strings and 2,359 named constants. All constants have immutable getters.
Keywords use `Kw<Name>()`; non-keywords use `Sym_<exact_Rust_identifier>()` to
preserve case-distinct source names. `SymC`, `SymF`, `SymD`, `SymMacroRules`,
`SymDummy`, and `SymInclude` cover the source aliases and shared runtime bridges.

`InternSymbol(session, text)` and `Symbol.AsStr(session)` adapt scoped TLS to
an explicit actual `SessionGlobals`. Symbols remain four-byte checked indices;
their equality, ordinary order and ordinary hash use indices. Stable comparison
uses UTF-8 content. Stable hashing uses the actual concrete stable hasher and
the source `[u8]` specialization: `usize` length followed by one bulk write.
It does not use std string Hash's sentinel convention. The ordinary Hash API
uses the complete required Hasher contract and preserves typed overrides and
the inherited native-endian `write_u32` default.

Symbol and ByteSymbol share the same locked store and insertion indices. The
actual shared `Lock` adapter preserves the selected synchronization mode and
borrows its inner cell. Go maps and immutable strings supply the private hash
table and arena backing. `ByteSymbol.AsByteStr` returns an immutable Go string
that may contain any bytes; `AsBytes` copies. Callers must not mix session IDs
or outlive the originating interner's source lifetime. Vacant entries copy nonempty input into distinct arena storage; occupied entries
retain the original arena allocation, and prefill retains static borrows.
`StdlibStableCratesBorrowed` returns the read-only static array borrow; the
older slice convenience returns an independent copy. Explicit session Close
invalidates the real lock and releases its owned data. This bridge makes no
Rust ABI/niche-layout claim for Go's generic `Option` representation.

Ident's source equality and Hash use its name and hygiene context, ignoring
positions and parent. Use `Equal` and `Hash`, or the immutable Go-only
`CanonicalKey` for host maps; builtin Go equality also sees the full Span.
Normalization, edition lookup and dollar-crate conversion use the actual
hygiene runtime. `NewIdentAtLocationCell[WithContext]` is the borrowed compiler caller-location
primary; the same immutable cell reaches diagnostic and panic callbacks.
`NewIdentAt[WithContext]` remains the scalar caller-location convenience. Helpers without `track_caller` resolve exact native callsites only through a
caller-supplied `PanicOperations.LocationCells` profile (or the scalar
`Locations` convenience). Without a profile, the
diagnostic keeps its logical `Host=true` projection and the panic location
remains nil. Host convenience captures are explicitly marked `Host=true`.

`SymbolStableComparatorOwned[WithContext]` consumes an arbitrary context with
observable Drop before returning the noncapturing comparison function. The
returned function takes the active session when invoked. The trivial-drop
convenience is separate. Translated Rust panic sites use typed `Context.PanicAt`/`PanicFormattedAt`;
raw Go callback-local defer/recover timing remains host behavior. The owning
outer destructor enforces abort if a panic escapes while it is unwinding.

The source18 debug primitives are explicit new neutral helpers in
`port/support/rustfmt/unicode18.go`. Existing Unicode17 primitives and fixtures
remain unchanged. Symbol's wrappers and contextual lower/uppercase code use
the exact pinned core Unicode18 tables. The latter preserves multi-character
conversions, NUL, and the original final-sigma scan of Case_Ignorable/Cased.

The source-test interner witness only exposes the real private prefill methods
to mirrored child Go tests. It adds no upstream public API or source credit.
The data generator supports the pinned macro invocation; the complete original
proc macro remains an independently compiled development dependency. This
scope does not claim a Go Rust parser/proc-macro crate implementation.

## Reproduction

New tools require official nightly `2026-11-04`, Rust 1.101.0-nightly commit
`708c94ef093ecc42fb8851e17d8383d999727abc`, and Unicode18. They honor explicit
`FERRUGO_RUSTC`, `FERRUGO_CARGO`, `FERRUGO_RUSTDOC`, `CARGO_HOME`, and
`FERRUGO_RUST_SOURCE` selections. The default is the matching local dated
toolchain, with no compiler1.99 fallback. Original sources are read from
verified pinned Git objects with lazy fetching disabled.

```sh
python3 tools/port-rust-span-reference/prepare.py
python3 tools/port-rust-symbol/generate_predefined.py --check
python3 tools/port-rust-symbol/verify_predefined.py --offline --prepared --check
python3 tools/port-rust-symbol/test_original_macros.py --offline --prepared --check
python3 tools/port-rust-symbol/generate_edit_distance.py --offline --check
python3 tools/port-rust-symbol/generate_unicode18.py --offline --check
python3 tools/port-rust-symbol/generate.py --offline --prepared --check
python3 tools/port-rust-symbol/generate_borrow_contract.py --prepared --check
python3 tools/port-rust-symbol/panic/generate.py --prepared --offline --check
python3 tools/port-rust-symbol/audit.py --check
go test ./port/rust/compiler/rustc_span/src/symbol ./port/rust/compiler/rustc_span/src/edit_distance
go test ./port/rust/compiler/rustc_span/src -run '^TestSymbol|^TestIdent.*Location'
go test -race ./port/rust/compiler/rustc_span/src -run '^TestSymbol|^TestIdent.*Location'
```

The full original reference graph contains all ten original compiler crates,
their exact locked registry dependencies, all 43 original span tests, and the
actual macro/serialization/hash implementations. No original body is modified.
Use its complete paired metadata/rlib link arguments; an rlib alone is
insufficient with this Cargo version.

The reference tools cap requests and fixture sizes, run child processes with
timeouts, and disable core dumps for deliberate abort probes. Compressed
fixtures retain existing valid compressed bytes whenever raw bytes match,
keeping verification independent of compressor-version differences.

## Coverage

- Every generated name/index/prefill byte and all ASCII bases, six tracked-env
  profiles, and all four original proc-macro tests.
- Original Symbol tests: three declarations and ten assertions in source order.
  Original edit-distance tests: five declarations and 26 assertion sites,
  including the source's exclusive-`char::MAX` loop.
- The retained whole-crate corpus covers keyword closure evaluation, arbitrary
  byte sharing/mutation, all 65,536 two-byte strings, extras/duplicate diagnostics,
  name matching and tie selection, Unicode casing, fallible formatter traces,
  real memory codecs, raw-ID bounds, integers through i128/u128, non-root
  hygiene normalization, caller locations, and inherited/default trait methods.
- Four independent arithmetic profiles cover both unchanged edit algorithms.
- All 1,112,064 valid scalars match native char/str Debug writer traces, four
  quote-flag results, lower/uppercase, and two contextual sigma strings each.
- Seven allocation/borrow observations match in all four native profiles,
  including fresh versus occupied allocations, shared string/byte storage and
  static prefill/standard-library-crate borrows. No raw address or allocator ABI
  is compared between languages.
- ContextOrd probes cover consumed non-Copy state, two later sessions, normal
  and inherited Drop panics, processing-panic state, and isolated aborts.
- An additional 660 actual whole-crate observations in all four arithmetic
  profiles cover typed panic payloads, exact source locations, processing phase,
  generated SymbolIndex bounds, real interner lock release after a bounds
  failure, duplicate prefill, RawLifetime expect, ordinary clamp, absent scoped
  session, edit-distance limit overflow and constructor diagnostics. The 192
  isolated aborts retain every component/hook event. A generic std ToString
  client separately proves its error fallback; no fabricated Symbol input is
  claimed to force that unreachable infallible-writer branch. Four additional
  no-sync configurations prove an interner bounds hook still holds the lock:
  attempted recursive access aborts before any local cleanup or second hook.
- The new owned-context witness records construction versus current runtime,
  body and field Drop, a field seeing the body's updated value, intern access
  during Drop, return timing, and calls under two later independent sessions.
  Body/field failures cover normal, inherited-caught, inherited-escaping and
  processing contexts. Exact-location and absent-profile Go witnesses preserve
  caller spellings and never infer a native filename. Actual native constructor
  traces and Go cell witnesses also prove the diagnostic and panic hook share
  the same caller-location cell, including after scalar callback swap/restore.

`panic/manifest.json` preserves actual native hook filenames verbatim. The
older 60,984-record constructor diagnostic comparison intentionally retains its
named logical build-prefix projection; it is not an implicit native hook
location. Nil/uninitialized Symbol session handles represent the original
absent scoped session, while a consumed owner remains a separate Go misuse
guard. Invalid UTF-8, invalid scalar/enum and out-of-domain host integer inputs
also remain Go guards because those values have no safe Rust input counterpart.
The complete SessionGlobals constructor ownership proof, including consumed
SourceMapInputs loader cleanup after prefill failure, belongs to
`tools/port-rust-span-lib/native_session_inputs.rs`; this scope delegates to that
actual source implementation and does not duplicate its completion credit.

Only the 64-bit Linux reference profile is runtime-validated by this corpus.
The separate Go source algorithms retain `uint`/usize behavior and explicit
arithmetic profiles; architecture compilation is not a target-runtime claim.

Rust project licenses are retained by the parent port. Unicode18 data and its
Unicode3 notice are preserved with the separately ported core Unicode file.

## Verified producers and relocated replay

All whole-crate clients revalidate the complete original source, all pinned
registry archives and extracted files, compiler selection, all 43 upstream
tests and external linkage for each required build profile. `--prepared` is a
backward-compatible spelling only; it cannot skip that verification. A requested
macro-profile JSON selects assertion/overflow flags, never executable commands
or cached linkage.

The historical panic manifest/fixture describes its actual producer. In
particular, scoped-tls panic filenames contain the Cargo registry source path.
Those bytes are intentionally verbatim and are not expected to reproduce on a
host with a different Cargo cache root. Ordinary Go replay of that historical
profile remains supported. CI instead generates its own explicit producer:

```sh
producer="${RUNNER_TEMP:-/tmp}/ferrugo-symbol-panic-producer"
python3 tools/port-rust-symbol/panic/generate.py --offline --output-dir "$producer"
FERRUGO_SYMBOL_PANIC_PROFILE_DIR="$producer" go test -timeout 20m ./port/rust/compiler/rustc_span/src -run '^(TestSymbolOrdinaryPanic.*|TestSymbolPanicProducer.*|TestSymbolPanicAbortHelper)$' -count=1
FERRUGO_SYMBOL_PANIC_PROFILE_DIR="$producer" FERRUGO_SYMBOL_PANIC_LIVE=1 go test -timeout 20m ./port/rust/compiler/rustc_span/src -run '^TestSymbolPanicLiveRegeneration$' -count=1
python3 -m unittest discover -s tools/port-rust-symbol -p test_reference.py
python3 -m unittest discover -s tools/port-rust-symbol/panic -p test_portability.py
```

Retain the selected directory's manifest.json, symbol_panic.jsonl.gz and raw
symbol_panic.jsonl as build artifacts. Its source bindings and compressed/raw
hashes are checked before replay. Empty, missing, nonregular or symlinked
explicit selections fail; they never fall back to historical data. Abort
children receive the validated absolute directory and verify the same profile.
Copying a complete bundle to another directory preserves its producer metadata
and is tested independently of the local build's path. No production Symbol
code, original source body, or accepted reference preparer is changed by this
proof selection protocol.

Default panic generation now verifies bounded historical raw equality before
any write, including when `--check` is absent. A different producer filename,
missing historical evidence, invalid gzip CRC or oversized historical input
requires explicit `--output-dir`; it cannot replace the historical fixture.
Equal raw observations may refresh actual source-binding metadata while
preserving the exact existing gzip representation. Isolated regressions cover
both default rejection and successful independent new-producer output.
