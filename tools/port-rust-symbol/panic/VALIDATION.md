# Symbol ordinary panic and owned-comparator contracts

The observer links the complete unchanged pinned `rustc_span` crate and all
actual macro/session/hygiene dependencies through the shared reference build.
It never replaces compiler methods or compiles rewritten original bodies.
`manifest.json` records the exact compiler, source graph, locked registry
checksums, source bytes, client argument spelling and Go/source bindings.

Selected SDK: official nightly 2026-11-04, compiler
`708c94ef093ecc42fb8851e17d8383d999727abc`, Unicode 18.0.0,
`CFG_RELEASE=1.101.0`, x86_64 Linux. Four complete crate builds independently
cross debug assertions and overflow checks. Each provides the actual paired
metadata/rlib link arguments; an rlib alone is insufficient.

| Source operation | Actual Rust payload kind |
| --- | --- |
| Generated SymbolIndex u32 bound | static_str |
| Symbol/ByteSymbol Ord clamp | static_str |
| Absent scoped session | static_str |
| Edit-distance limit addition overflow | static_str |
| Interner vector indexing | owned_string |
| Duplicate prefill diagnostic | owned_string |
| RawLifetime expect | owned_string |
| Generic ToString Display error fallback | owned_string |
| Emitted compiler bug | opaque ExplicitBug |

Ordinary payload bytes, hook flags, source filenames/line/column, panic state,
writer events and destruction order come directly from native execution.
The actual constructor traces also compare the diagnostic location reference
and panic-hook location reference: they are the same cell for both ExplicitBug
and string fallback. `NewIdentAtLocationCell` and selected internal cells keep
that exact immutable reference through the shared bug implementation. The
scalar At API remains a convenience; scalar callback swap/restore preserves
any carried primary borrow.

Locations remain verbatim; no build path is inferred, normalized or inserted
by the primary Go sites. The older constructor diagnostic corpus retains its
separate named logical projection. Missing metadata yields nil locations;
independent Go witnesses also pass a selected `./` spelling through unchanged.

The ToString fallback is a separate actual std client whose Display returns
fmt::Error. Symbol's fixed infallible string writer cannot safely force that
branch. The audit corrects its old adapter text to the actual
`a Display implementation returned an error unexpectedly: Error`, without
claiming an artificial Symbol input can trigger it.

There are 660 native observations and 177 grouped rows: 83,669 raw bytes and
3,132 compressed bytes. Raw SHA-256:
`4f8340f215379435f7a51f06dd08e6656a5f897c90fcd9e7b0b63403c1e6e40f`.
Compressed SHA-256:
`b3ac84aeccb924c9f44c0cdd3e94130a19e12ae00f5009354b6a4f6fe08fc733`.

| Build | Ordinary observations | Ordinary aborts | Extra no-sync abort |
| --- | ---: | ---: | ---: |
| d0o0 | 164 | 44 | 1 |
| d0o1 | 164 | 46 | 1 |
| d1o0 | 164 | 48 | 1 |
| d1o1 | 164 | 50 | 1 |

The 192 isolated aborts compare the abort class and every component/hook event.
Native runtime backtrace/abort-message text is outside this component scope.
Each ordinary failure is exercised in normal catch, caught inherited unwind,
escaping inherited unwind and processing-panic contexts. Subsequent successful
intern/lookup proves bounds unwinding releases the lock. The additional no-sync
witness attempts a recursive lookup from the bounds hook: the lock is still
held and the second panic aborts before cleanup or a second hook.

The owned ContextOrd witness consumes a non-Copy context constructed in an old
scope. Its Drop receives the current factory scope, updates the actual value
and a field, interns through the active factory session, and then destroys the
field. The field observes the updated value before the comparator returns.
The noncapturing returned function is invoked under two later independent
sessions. Body/field failures include local double-panic aborts and preserve
catch behavior under an inherited outer unwind. No global Go panic-state
interception or hidden owning callback environment is introduced.

Whole SessionGlobals constructor cleanup, including consumed SourceMapInputs
loader destruction after duplicate prefill fails, is separately demonstrated
by `tools/port-rust-span-lib/native_session_inputs.rs`. These Symbol hooks run
before that constructor's unwind cleanup; the owner proof is not duplicated
as new source/file/test credit.

Reproduction with the prepared exact source/registry/native graph:

```sh
python3 tools/port-rust-symbol/panic/generate.py --prepared --offline --check
FERRUGO_SYMBOL_PANIC_LIVE=1 go test ./port/rust/compiler/rustc_span/src -run '^TestSymbolPanicLiveRegeneration$' -count=1
go test ./port/rust/compiler/rustc_span/src -run '^TestSymbolOrdinaryPanic' -count=1
go test -race ./port/rust/compiler/rustc_span/src -run '^TestSymbolOrdinaryPanic' -count=1
```

Tools honor the shared exact-nightly `FERRUGO_RUSTC` override and use offline
Git source access. Native children have ten-second deadlines, 1 MiB stdout and
stderr acceptance bounds, disabled core dumps and a 16 MiB retained raw limit.
Go replay checks compressed/raw hashes and count/size bounds; abort children
have ten-second deadlines. The opt-in native check has a twenty-minute limit, including mandatory complete verification of all four profiles.
Existing gzip bytes are retained whenever raw observations match.

Final Go 1.23.12 gates pass: the complete Symbol/Ident-location race run
(26.552 seconds), span/dependency/child vet, both original child race packages,
and the opt-in fresh native regeneration (5.06 seconds). Sixteen explicit
unlocated/caller-spelling witnesses, eight borrowed-cell/swap-restore cases
and the parent's 32 constructor-profile cases pass. The 386 binary compiles;
that check does not claim target runtime validation.


The checked-in fixture is historical producer evidence, not a cross-host
filename contract. The actual scoped-tls source path is compiled into its panic
location; moving CARGO_HOME changes that producer metadata. `--output-dir DIR`
produces an independent manifest.json, symbol_panic.jsonl.gz and raw
symbol_panic.jsonl without changing historical fixture bytes. Set
FERRUGO_SYMBOL_PANIC_PROFILE_DIR to that directory for all Go panic replays and
for the opt-in native `--check`. Producer filenames are never normalized.

Every whole-crate Symbol tool now uses the existing complete verified reference
loader for each requested profile. `--prepared` cannot bypass verification, and
cold four-profile requests do not depend on pre-existing build-profile JSON.
Selector, poisoned-linkage selection, cold-profile dispatch, gzip CRC/bounds,
relocated normal/abort replay, missing selections and corruption checks are
retained in test_reference.py, panic/test_portability.py and
symbol_panic_evidence_test.go. See ../PORT.md for exact CI commands/artifacts.

Default panic generation now verifies bounded historical raw equality before
any write, including when `--check` is absent. A different producer filename,
missing historical evidence, invalid gzip CRC or oversized historical input
requires explicit `--output-dir`; it cannot replace the historical fixture.
Equal raw observations may refresh actual source-binding metadata while
preserving the exact existing gzip representation. Isolated regressions cover
both default rejection and successful independent new-producer output.
