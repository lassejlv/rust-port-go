# Progress

## 2026-11-04 — checkpoint 1: project foundation

- Named the project Ferrugo.
- Initialized the empty repository on `main`; all implementation work stays there.
- Defined shared syntax/compiler/package-manager interfaces for parallel implementation.
- Set the executable-bootstrap goal and recorded the remaining compiler and Cargo scope.
- Installed a verified Go toolchain locally for development; toolchain binaries are not tracked.

The full rustc/Cargo port remains incomplete. This log records tested milestones rather than an estimated percentage.

## 2026-11-04 — checkpoint 2: executable compiler and Cargo bootstrap

- Implemented Go lexer/parser, source spans and syntax validation.
- Added type checking, scopes/shadowing, mutability, calls/recursion, return validation, contextual integer types and literal bounds.
- Added C11 lowering with ordered evaluation, short-circuiting, length-carrying strings and checked signed arithmetic.
- Added `ferrugo` CLI with check, native output and C emission.
- Added Go Cargo-style new/init/check/build/run/clean commands and a strict manifest parser.
- Added regression tests and a working Fibonacci Cargo example; all current package tests pass and `make demo` runs successfully on Linux/GCC.
- Added Linux/macOS CI configuration. Hosted CI results are not yet verified.

Remaining scope is listed in ROADMAP.md and COMPATIBILITY.md. This is an executable bootstrap, not a complete rustc/Cargo port.

## 2026-11-04 — checkpoint 3: expression semantics and differential validation

- Added value-producing if/else and block expressions, unit expressions, branch inference and control-transfer analysis.
- Tightened expression-statement grammar against rustc and rejected unsupported value-less divergent arithmetic rather than accepting invalid Rust.
- Enforced the accepted &str return lifetime-elision boundary.
- Added 30 committed fixtures: 18 executable cases, eight invalid-Rust rejections and four valid-but-unsupported boundaries. All match their expected behavior against rustc 1.99.1 on Linux.
- Added 12 more native expression/control-transfer regressions, manually cross-checked with rustc, and expanded fuzzing without observed parser/emitter panics.
- Passed the full local race-enabled Go suite, vet and the package demo.
- Fixed a macOS test path-alias assumption found by hosted CI; added rustc differential checks to Linux/macOS CI.
- Pinned the inspected Rust/Cargo source revisions and recorded a component inventory. No upstream component is fully ported.

The initial executable-bootstrap goal is achieved. The overall compiler/Cargo port is still incomplete; ownership, traits/generics, user-defined types, modules, standard libraries, crate linking and dependency resolution remain substantial next work.

## 2026-11-04 — checkpoint 4: review corrections

- Independent review found no backend mismatch across 102 additional evaluation-order programs compared with rustc.
- Corrected CRLF normalization and escaped line continuations in Rust strings while retaining original source offsets.
- Rejected leading underscores in Unicode escapes, bare carriage returns in strings and invalid UTF-8 anywhere in source, including comments.
- Tightened the manifest parser to reject invalid UTF-8, raw controls, lone carriage returns and non-TOML whitespace before comment removal or escape decoding.
- Added source/string regressions and 156 manifest syntax cases checked against Python's TOML/UTF-8 parsers.
- Passed the full local race-enabled suite with rustc differential comparisons and vet after these corrections.
- Verified hosted Linux/macOS CI, including rustc comparisons, for checkpoint 3: https://github.com/lassejlv/rust-port-go/actions/runs/41828174561.

The full port remains incomplete. Current compatibility and the remaining scope are documented separately; successful bootstrap checks do not establish Rust ecosystem compatibility.

## 2026-11-04 — checkpoint 5: multi-file modules

- Added inline/external modules, public/private visibility, qualified calls and simple function/module import aliases.
- Added canonical name resolution with `crate`, `self`, repeated `super`, import-chain and privacy checks while preserving lexical variable shadowing.
- Added a crate file loader for both `name.rs` and `name/mod.rs`, including nested inline/external bases, ambiguity/missing-file errors and physical file-cycle checks.
- Unified CLI/Cargo checks, native builds and C emission around file-based crate analysis. Output protection includes all participating module sources and hard-link/symlink aliases.
- Preserved original source filenames and positions in diagnostics.
- Supported root `main` aliases through an independent compiler-generated wrapper after review against rustc.
- Added eight multi-file success cases and nine rejection cases; all 47 committed conformance cases pass against rustc 1.99.1. Resolver cases and loader/CLI/Cargo regression tests pass.
- Passed the full local race suite and vet; updated the Cargo example to import Fibonacci from a second source file.

The multi-file goal is implemented locally; hosted CI will validate the pushed checkpoint. Named structures and field access are the next language milestone. The complete compiler and Cargo port remains the overall goal.

## 2026-11-04 — checkpoint 6: consistent C output paths

- Fixed a reviewed path mismatch in C-file emission: source protection checked a normalized output path while publication used the original spelling. A symlink followed by `..` could otherwise target a loaded module source.
- C emission now uses one normalized absolute output path for protection, temporary-file creation and publication.
- Added a regression that reproduces the symlink/parent traversal case and verifies source bytes are preserved; focused C-emission tests pass.

## 2026-11-04 — checkpoint 7: typed Copy records

- Added named record declarations deriving Copy and Clone, primitive/nested field types, constructors with shorthand fields, projections and mutable local field assignments.
- Resolved record types and import aliases across modules into canonical type identities while preserving source type spelling in parsed signatures.
- Added typed function parameters/results and branch values containing records, field/type privacy checks, and cycle detection for impossible recursive by-value layouts.
- Added topologically ordered C record definitions, collision-free record identifiers, by-value copies and constructor evaluation in written field order.
- Preserved mutable assignment places instead of mutating temporary copies; RHS evaluation precedes field writes.
- Added 21 fixtures; all 68 committed conformance cases pass against rustc 1.99.1, including construction/copies/mutation/order/privacy failures and documented unsupported move-only records.
- Independent review found and corrected primitive-name record import shadowing; function/module aliases named like primitive types remain allowed.
- Updated the multi-file Cargo demo to return Fibonacci results in records. Local full race tests, vet and the demo pass.

General ownership, borrowing, generic traits, move-only records, enums and standard libraries are still required for the overall port. The next Cargo milestone is local library crates and path dependency graphs.

## 2026-11-04 — checkpoint 8: local library crate graphs

- Added separate crate roots and extern preludes, transitive/renamed local library dependencies, shared diamond type identity, and library checking without a main entrypoint.
- Extended the bounded Cargo manifest parser with inline path/package dependency tables, single binary/library root discovery, graph cycles/name checks and `new/init --lib`.
- Added `CheckCrate`/`BuildCrate` graph APIs and standalone library checks; the compiler CLI accepts `--check --crate-type=lib`.
- Kept dependency crates isolated: `crate`/`self`/`super` resolve inside their own crate, private items stay private, and only the selected binary needs main.
- Added explicit package identity for distinct packages sharing a source file, preserved logical source/manifest symlink directories, and guarded all dependency source files against output replacement.
- Added 16 committed package fixtures checked against original Cargo 1.99.1 offline, plus native compiler graph tests compared with real rustc rlibs. Existing 68 language/module/record fixtures continue to pass.
- Added a runnable example with two local library crates and a renamed dependency; it prints `fib(10) = 55 through two local library crates`.
- Local race-enabled tests, Go vet, both demos and Rust/Cargo differential runs pass. Hosted CI now includes compiler graph and original-Cargo package comparisons.
- Explicitly rejected replacing the standard prelude through an external `std` dependency. Full edition-specific imports/prelude behavior remains unimplemented.

This milestone links supported local library code into one bootstrap C executable. It does not emit Rust rlibs, implement Rust ABI compatibility, or complete registry/feature/version/workspace resolution. The overall port remains in progress.

## 2026-11-04 — checkpoint 9: integer widths and bit operations

- Added signed i8/i16/i32/i64/isize and unsigned u8/u16/u32/u64/usize, preserving nominal distinctions even when host widths agree.
- Added exact literal bounds, signed minima, unsigned formatting, integer complement/bitwise operations, boolean eager bit operations, shifts and compound assignments on locals/record fields.
- Shift RHS types are independent of LHS types; counts outside the LHS width (including negatives) panic with status 101. Left shifts truncate bits and signed right shifts sign-extend.
- Added C runtime helpers that check overflow before arithmetic and avoid undefined signed shifts/conversions through unsigned bit containers and memcpy.
- Added 25 numeric fixtures; all 93 language cases pass against rustc. Existing 16 package fixtures continue to pass against original Cargo offline.
- Direct C runtime tests exhaust valid i8/u8 arithmetic pairs, cover shift boundaries and verify 105 panic modes; normal and UBSan runs pass.
- Independent review matched 2,703 numeric outputs and 60 runtime trap cases with rustc; optimized emitted C also passed UBSan.
- Host pointer width is checked against the C compiler target. i128/u128, floats, chars, casts and cross-target selection remain further work.
- Pinned the comparison step's Rustup toolchain environment so copied Cargo fixtures use the selected version outside the repository directory.

## 2026-11-04 — checkpoint 10: integer casts

- Added `as` parsing, target name resolution and type checking across all supported integer pairs and from bool to integer.
- Generated casts evaluate once, truncate unsigned bits and transfer signed bit patterns with memcpy, preserving Rust narrowing and sign extension without undefined C conversions.
- Matched Rust's source inference boundaries: direct literals/unary/block tails may receive a cast expectation, while binary/conditional expressions establish their own source types.
- Conservatively require annotations/suffixes for casts from initially defaulted integer locals rather than silently treating Rust target-directed locals as i32.
- Added 19 cast fixtures and inference regressions; all 112 language cases pass against rustc. Existing 16 Cargo package cases continue to pass offline.
- Independent review matched 1,420 fixed-source cast outputs and 31 inference/grammar reproductions; optimized emitted C passed UBSan.
- Full local tests and vet pass. General coercion/float/char/reference casts and full global inference remain further work; fixed-size arrays are the next language milestone.

## 2026-11-04 — checkpoint 11: owned Copy arrays

- Added fixed-size array types/literals/repeats, nested arrays and records, typed function values, usize indexing, indexed mutation and array length access.
- Added bounds panics and preserved repeat evaluation once even at zero length, RHS-before-place assignment order, and place-versus-temporary index behavior.
- Added mixed array/record layout analysis, element privacy and canonical type identity across crates; bare value-less divergent indexes reject while typed unreachable usize tails retain valid control flow.
- Added 22 array fixtures and replaced the unsupported-array boundary with slices. All 134 language conformance cases pass against rustc; the 16 package fixtures remain green.
- Review caught and corrected inflated zero-sized storage and unnecessary whole-array/record copies during len/field reads. Logical zero-sized aggregates now use one dummy byte; zero-sized fields carry no stored data. Constructors and indexed places retain side effects and bounds checks.
- Added native regressions for large zero-sized nests, repeated length/scalar-field reads, zero-sized field effects and zero-length arrays of large elements.
- Independent review matched 23 array/side-effect/layout regressions against rustc and ran their emitted C at O2 with UBSan without reports.
- Updated the two-library Cargo demo to use an array of returned records and usize indexing. Local race tests, vet and both demos pass.

Lengths are literal-only and bounded to 65536; general const evaluation, slices/borrowing, reference/string/unit elements, iteration traits and Rust layout/ABI introspection remain future work.

## 2026-11-04 — checkpoint 12: bounded for loops

- Added bounded exclusive/inclusive integer ranges and owned Copy-array loops with name, mutable-name and wildcard bindings.
- Preserved left-to-right, once-only bound evaluation, array snapshot semantics, zero-sized item effects, nested break/continue and scoped bindings. Inclusive maximum endpoints skip increment after the last item, including on continue.
- Matched Rust's unit-valued for-statement checking even when a header expression diverges.
- Removed the conservative defaulted-local cast rejection after direct rustc comparisons confirmed that local paths are cast inference barriers; direct expression inference boundaries remain checked.
- Added 16 fixtures. All 150 language cases pass against original rustc, and the Cargo demo now indexes its array with a for loop.

The next goal is complete upstream file ports with a per-file inventory and reproducible upstream comparisons. The current bootstrap cannot compile production Rust; completing the compiler, standard libraries and Cargo remains required.

## 2026-11-04 — checkpoint 13: complete source-file ports and provenance

- Inventoried all 67,326 top-level entries in the pinned Rust/Cargo Git trees: 40,930 Rust files, 26,383 support/build files and 13 explicitly unexpanded gitlinks. Every file has a destination and source object identity. Compiler test inputs remain Rust programs; executable harness implementations map to Go tests.
- Ported the complete low-level lexer implementation and its complete test file. All 17 upstream Rust tests and their Go counterparts pass; 231 retained original-generated cases cover all token and literal variants. Live comparisons also cover seeded UTF-8/helper stress and hydrated source files.
- Generated lexer Unicode tables from exact locked dependencies and compared every Unicode scalar. The source-ported cursor now drives the bootstrap frontend's source scanning through an explicit adapter.
- Ported Cargo's complete edit-distance file, including generic suggestions, scalar comparisons, transpositions, Unicode lowercase expansions and contextual final sigma. The Rust 1.99.1/Unicode 17 standard-library profile is explicit and separate from pinned core's Unicode 18.
- Ported Cargo's complete concurrent queue file, including bounded/unbounded producers, timed consumers, drains and the full Rust duration range. Original-source operation traces, controlled timeout interleavings and race tests pass.
- Ported Cargo's complete graph file with sorted traversal, path/cycle behavior, debug assertions and explicit Go trait/storage adapters. All three upstream tests and 1,416 graph cases match original debug and release builds within the documented logical graph contract.
- Ported pinned core's complete Unicode 18 generated data file, retaining all 33 compressed tables and algorithms. Every scalar matches the unchanged original for all 13 predicates and all four exact three-slot case conversions.
- Added source-specific provenance/validation reports and a hosted job that fetches exact references and compares Go ports with the unchanged originals. Full local race tests, vet, compiler/Cargo comparisons and inventory tests pass.
- Verified the minimum Go 1.23 toolchain and fixed executable lookup, machine-specific report metadata and cross-version gzip regeneration differences before pushing. Removed a scheduling-dependent Rust queue test probe; the direct upstream operation comparisons and controlled Go concurrency regression remain.

These six completed source files begin the direct port. The broader compiler, standard libraries, Cargo integration and production compatibility gates remain unfinished. Work continues with the compiler's binary-search and base-encoding utilities.

## 2026-11-04 — checkpoint 14: compiler utility files and complete generic adapters

- Ported the complete binary-search module and its test file, preserving borrowed result ranges and separate Rust less-than, not-equal and less-or-equal operators. The expanded oracle matches 41,835 operator cases and 34,536 comparator-adapter cases, including callback traces and panics.
- Ported complete base-N encoding and its test file: all 128 input bits, exact digit alphabet, fixed widths, custom trait implementations and independent method overrides. Original comparisons cover 74,781 numeric cases, 543 failure cases and 1,225 custom-trait cases.
- Ported complete union-find and its test file with source-order path compression, rank updates, owned snapshots, debug/release checks and typed-index adapters. All 3,788 original-source records match results, parent/rank state, callbacks and recovered partial mutations.
- Expanded Cargo's graph adapter from native-comparable keys to every comparator-ordered Go key type, including slices and records containing slices. Independent Eq/Ne callbacks and stable edge addresses now have original-source comparison coverage. Existing integer-corpus results are unchanged.
- Ported the complete typed-index file, including custom/default operations, checked/wrapping profiles and all standard/nightly range conversions. All 32,644 original-source records match; source feature availability has an explicit Go build tag. Union-find's built-in index adapters now use this shared component.
- Ported the complete number-formatting module and its test file. All four upstream tests and 47,827 cases match, including 42,181 floating-point values, nonfinite values, signed zero and rounding boundaries.
- Review found and closed generic extension-point gaps before these new files received completion credit. Retained dependency shims are scoped and documented; they do not count as ports of other source files.
- Original-source, minimum-Go, race, vet, reproducibility and cross-version fixture checks pass. Hosted CI now rebuilds the new utility references; the preceding source-port and Linux jobs passed, while the macOS runner remained queued.

Fifteen source files now have recorded port/validation evidence. This does not complete their containing compiler crate or production Rust compatibility. The complete format-string parser and its literal-escaping dependency are next.

## 2026-11-04 — checkpoint 15: full format parser and literal escaping

- Ported the complete 1,042-line format/assembly/diagnostic string parser and its complete 607-line test file. All 28 translated tests preserve original inputs, expectations and byte spans; an independent source/Go literal audit is retained.
- Matched 4,684 original-source requests and 57,824 complete state observations, including errors/suggestions, derived Debug, lookaheads, unfused iteration, public state mutations and checked/wrapping panic recovery. The unchanged parser and lexer tests all pass in the reference workspace.
- Ported the entire locked `rustc-literal-escaper 0.0.8` dependency, all nine tests and 27 benchmark workloads. Its separate manifest accounts for all 14 archive files and preserves licenses. Retained cases and exhaustive scalar checks validate values, errors, warning severity, ranges and public enum constructors.
- Replaced manual normal-string decoding with the complete escaping component, preserving CRLF normalization, original byte/scalar error ranges, NUL and explicit frontend boundaries.
- Replaced duplicate print-format scans with the complete format parser and a shared checked plan. The supported macro profile remains bare sequential `{}` and escaped braces; the emitter evaluates arguments once in source order and consumes that plan.
- Ported Cargo's complete package/registry/profile/feature/path-base name validation and sanitization file. All eight functions, exact errors and Unicode 17 behavior match 32,779 original requests and every scalar. This is a Cargo component port; bootstrap manifest feature boundaries remain explicit.
- Actual Go 1.23 source/reference, race, vet, fuzz and reproducibility checks pass. The 150 language and 16 Cargo comparisons continue to pass. The preceding checkpoint's Linux, macOS and upstream-reference jobs are green at `1e6d540`.

Eighteen top-level upstream source files have evidence, with the three dependency source/test/benchmark files tracked separately. Production Rust compilation remains unfinished. Work continues on complete typed storage and intervals for hygiene/parser capture, followed by the span/session runtime, while Cargo source ports continue in parallel.

## 2026-11-04 — checkpoint 16: complete edition metadata

- Ported the complete edition module, including all five editions, predicates, names, parsing, ordering, formatting and derived hashing/serialization contracts.
- Compared 3,220 original-runtime records. These cover every edition pair, formatting options and writer failures, 645 stateful hash comparisons, every decoder byte tag and 2,070 parsing cases.
- Built the unchanged edition file with the original derive macros, real memory codecs and real standard/stable hashers. Bounded unused reference interfaces fail if reached; their audited call count is zero. These reference interfaces receive no source-port credit.
- Verified all 31 locked dependency archives and 920 expanded registry files actually used by the reference. Five isolated checks confirm that changed, missing, extra and symlinked sources fail verification.
- Added the edition reference to hosted validation and expanded pinned source preparation for the compiler runtime. Minimum-Go comparison, race tests, vet and cross-version fixture checks pass. The preceding checkpoint passed Linux, macOS and upstream-reference jobs at `b053c86`.

Nineteen top-level source files now have recorded evidence. The typed storage/interval block and complete semver dependency/Cargo extensions remain under implementation and review; their uncommitted presence grants no completion credit. Work on span encoding and shared hash types has begun. The compiler still cannot compile production Rust.

## 2026-11-04 — checkpoint 17: fixed-width hash types and completion corrections

- Ported the complete compiler hash-value module: 64/128-bit values, wrapping arithmetic, conversions, cloning, ordering, typed hashing and delegated Debug/LowerHex behavior. Independent review and the actual minimum Go race/vet checks pass.
- Matched 8,635 original-source requests and 85,483 records. Formatting checks include every writer-failure position, independent character writes, empty writes and formatter state after errors. The unsigned primitive formatter is scoped supporting code, not a standard-library file completion claim.
- Reopened six earlier source records after review found incomplete formatter or generic runtime adapters. Their algorithm evidence remains; the [contract review](SOURCE-CONTRACT-AUDIT.md) identifies the missing work. The literal-escaping dependency implementation is similarly marked partial until its formatter and owned-callback contracts close.
- Updated hosted reference checks for the hash module. The preceding edition checkpoint passed all hosted jobs at `048b316`.

The current ledger has fourteen validated top-level files. Completion counts now reflect the reopened contracts instead of retaining premature full-file credit. Typed storage/intervals, semver/Cargo versions and the span runtime continue in parallel; production Rust remains the overall unfinished goal.

## 2026-11-04 — checkpoint 18: typed storage and complete union-find interfaces

- Ported complete typed slices and vectors plus all five original vector tests. The primary APIs retain actual borrowed cells, owned callback/range cleanup, generic trait overrides, iterator specializations, zero-sized vectors through `usize::MAX`, full formatter state and nightly codecs.
- Independent original-source comparisons pass 150,068 records and 189,668 operation steps across eight separate feature/arithmetic/assertion profiles, including 40,232 caught panics and 104 isolated abort cases. The original index macro and serialization implementations are used unchanged in reference builds.
- Closed union-find's full Debug and public return-type gaps. The primary snapshot now returns the validated typed vector; its legacy slice-shaped result remains a convenience. All 25,040 full formatter records and the original 3,788 algorithm records pass.
- Follow-up API review reopened the binary-search implementation for owned key/extractor cleanup and borrowed operator cells. Existing search comparisons remain retained, and the source test file remains separately validated.
- Minimum-Go, normal/nightly, race, vet, original-source and artifact-integrity checks pass for the integrated block. Hosted checks now include the storage reference and union-find formatter. The preceding hash checkpoint passed all hosted jobs at `b472da0`.

Seventeen top-level files currently have completion evidence. Interval ownership/formatting, earlier reopened contracts, semver's owned iterator collection, stable hashing and span/session runtime remain active work. None of these milestones establishes production Rust compatibility.

## 2026-11-04 — checkpoint 19: intervals, spans and source-contract restorations

- Ported complete interval sets/sparse matrices and all seven original tests. The unchanged original matches 62,380 comparison records and 106 isolated aborts across four independent assertion/overflow profiles, including owned range cleanup and full formatter state.
- Ported complete fingerprints and their original permutation test. The reference uses actual serialization/hash dependencies and compares 9,144 requests, 55,292 records, typed hash callbacks, layout/codec operations and ownership failure ordering.
- Ported complete span encoding with all four representations, interner identity, tracking, owned context mapping and encoded-versus-location trait distinctions. Four native profiles match 15,264 requests and 142,168 steps. A separate 1,344-case review corpus closes internal contextual failure timing, including actual interner-lock observations before abort.
- Restored full-file lexer, binary-search, format-parser and restricted-name records after implementing their remaining contracts. Evidence covers lazy owned lexer iteration, independent flags/ordering, real borrowed cells, full-width zero-sized search and mutable fallible formatter behavior. The separate literal-escaper dependency also regains its completed implementation record.
- Integrated the shared streaming formatter/runtime adapter, with native builder and exhaustive Unicode Debug evidence. It receives no standard-library source credit. Surrounding span/session/definition/hygiene and stable-hash bridge files remain explicitly partial.
- Added native regeneration, artifact-integrity and feature-profile gates to hosted CI. The preceding typed-storage checkpoint passed all hosted jobs at `801a17e`.
- The integrated minimum-Go snapshot passes the full race suite, vet and nightly storage/interval tests; all 150 language and 16 Cargo fixture comparisons still match original Rust/Cargo. Owned-search and shared-builder regeneration and 34 source/tool integrity tests pass.

Twenty-six top-level files have recorded completion evidence. Base-N formatting/owned conversion and Cargo graph's generic persistent-container contracts remain reopened. Complete semver/Cargo version and stable-hash blocks are under integration; the wider span runtime is next. Production Rust compilation remains unfinished.

Hosted preparation exposed an executable-selection bug: resolving a Rustup
`cargo` symlink changed its invocation name to `rustup`. Four new Python
reference clients now preserve the absolute command path without resolving its
symlink. A regression checks PATH and explicit selections for rustc, Cargo and
Go through an actual symlink multiplexer. Only tool/evidence bindings change;
native fixture records and compressed bytes remain retained.
All affected native clients then passed fresh checks through actual symlink
dispatchers, including the complete lexer/literal/profile matrices, parser
Debug, owned search, span encoding and ambient lock/abort proof. The companion
span client also canonicalizes its reference directory before identity checks.

## 2026-11-04 — checkpoint 20: complete semver and stable-hash dependencies

- Ported the entire official semver 1.0.28 dependency archive: eight implementation files, six original test/support files and the benchmark file, with all support files and licenses inventoried separately. The Go implementation preserves parsing/matching, public comparator edge states, all trait/default overrides, owned iterator collection, Clone capacity/independence, full fallible formatting and optional Serde protocols.
- Compared 33,982 core requests, 3,136 Serde requests, 10,500 Debug records, 4,142 owned collector cases, 30 isolated aborts and 108 clone cases. All four feature combinations retain original behavior. All 34 original test functions, 71 assertion sites, four doctests and three benchmark workloads have explicit mappings; the optional Node comparison and compile-only 32-bit boundary remain documented.
- Ported both complete Cargo semver extension files, including all eleven original inline tests, exact/locked/precise requirements and RFC 3493's prerelease rules. The unchanged originals match 388,620 requests and 1,564,792 observations across checked/wrapping builds, plus full writer/state probes. This supplies components for version resolution; the bootstrap manifest acceptance profile remains bounded.
- Ported the entire rustc-stable-hash 0.1.2 dependency: four implementation and two original test files, default/nightly behavior, Sip1-3 buffering, typed primitive writes, generic hasher/result ownership, exact borrowed cells and full Debug contracts. Eight native build profiles match 3,742 streams, 10,594 formatter records, eight generic streams and 37 ownership cases. All original tests/doctests and private arithmetic comparisons are retained.
- Independent reviews closed concrete owned-collection, clone-capacity, visitor error-order and hasher runtime-context findings before acceptance. Supporting formatter/vector operation witnesses gain no standard-library file credit. Both dependencies have complete independent archive inventories and gain no top-level Rust/Cargo source count.
- Added native regeneration, original-test audits/execution and feature-tag race/vet gates to hosted validation. Checkpoint 19's Rustup dispatch follow-up is pushed at `c74a18e`; hosted results remain tracked separately from local evidence.
- Corrected cold-cache CI ordering so the exact SmallVec archive is prepared before interval's opt-in live test. The source test no longer reaches an intentionally offline checker before its dependency fetch. The new original stable-hash test runner also preserves Rustup executable names.
- The isolated minimum-Go integration snapshot passes the full race suite and vet, plus combined optional-feature race/vet. Fresh native semver/Cargo/stable-hash regeneration, both original-test audits, all four semver test/doctest profiles and all eight stable-hash test/doctest profiles pass. Source fixture bytes remain unchanged.

Twenty-eight top-level files have completion evidence. The next compiler block implements full symbols, definition IDs, hygiene, sessions, source files and source maps with their actual required dependencies. Base-N and Cargo graph's reopened contracts remain active. Production Rust compatibility is still unfinished.

## 2026-11-04 — checkpoint 21: source-file runtime prerequisites

- Added the exact unkeyed BLAKE3 operations needed by source-file hashing. The actual locked 1.8.2 crate matches 6,040 native requests through 2 MiB, including block/chunk/tree boundaries, update partitions, prefix/repeated finalization and resumed updates. Minimum-Go race/vet and independent source review pass; 386 cross-compiles without a runtime claim. The full archive is inventoried, with remaining crate/source APIs explicitly unported.
- Added the exact locked unicode-width 0.2.2 scalar operation used by rustc's character widths. Every one of 1,112,064 valid scalars matches the unchanged crate. String/CJK APIs and the remainder of that archive remain explicitly unported. Neither supporting adapter gains complete source-file credit.
- Completed the supporting DebugStruct non-exhaustive finish operation with 20,992 native writer/error/state cases. Existing lexer/literal outputs and gzip bytes remain unchanged; their helper bindings and original scalar-trace proof are refreshed.
- Added a reproducible reference for the entire unchanged rustc_span graph: ten compiler crates, 187 source/manifest files and 108 exact registry dependencies. Official nightly 2026-11-04 (Rust 1.101, Unicode 18) builds it without body replacements. All 43 original span unit tests and a real session/symbol/hygiene client pass in four independent assertion/overflow profiles. This reference build itself grants no Go port credit.
- Checkpoint 20 passed hosted Linux and upstream comparison jobs. Three concurrent macOS rustc invocations exceeded the fixture timeout; the differential CI step now limits package and fixture concurrency. Existing execution limits and comparison expectations remain intact.
- The isolated minimum-Go snapshot passes vet and affected-package race checks. Fresh BLAKE3, scalar-width and non-exhaustive-builder checks, all 48 executable-dispatch subcases, reference integrity tests and canonical complete-span preparation pass. The three timed-out fixture cases also pass locally with the revised concurrency flags; macOS verification remains a hosted check.

The ledger remains at 28 completed top-level files. Full session/source-file/symbol/hygiene/source-map and caching-view implementations are active uncommitted work, with native reference coverage being added before they receive completion records. The production compiler goal remains unfinished.

## 2026-11-04 — checkpoint 22: complete Hasher entrypoints

- Added complete required/default `Hasher` entrypoints to the existing 64/128-bit hash wrappers and fingerprints. Callers can supply raw-write-only hashers, independent typed overrides and arbitrary borrowed state. Exact Unhasher specialization remains distinct from wrapper and reference-wrapper implementations; inherited slice hashing retains the element loop and omits a length prefix.
- The unchanged source matches 2,808 native cases through both Go entrypoints, giving 5,616 comparisons. The corpus includes seeded and noncomparable state, callback failures, empty slices, all panic contexts and 380 native aborts replayed twice. Existing 85,483 hash-wrapper and 55,292 fingerprint records retain their original bytes.
- Added an independently usable development reference builder using actual pinned macro/hash/codec sources and exact registry dependencies. Its surrounding Symbol/Span providers are scoped interfaces and receive no source-file credit. Local input and registry checks run before and after Cargo. Executable dispatch regressions cover PATH, absolute and relative symlinks without changing Rustup command names.
- The isolated minimum-Go snapshot passes affected-package race tests and vet, fresh native regeneration, executable/source integrity tests and the complete file-map verifier. The existing large fixture bytes are unchanged.
- Checkpoint 21 passed all four hosted jobs: Linux, macOS, upstream source comparisons and the complete original-span reference. Full source-map/symbol/hygiene/definition-ID and owned Base-N contracts continue in the shared working tree.

The source ledger remains at 28 completed top-level files. This checkpoint closes additional trait entrypoints without claiming another complete source file. Production Rust compilation remains unfinished.

## 2026-11-04 — checkpoint 23: full Base-N contract restoration

- Completed the full Base-N file: arbitrary owned generic conversion, independently overridden ordinary/fixed methods, static associated length, exact checked/wrapping arithmetic, actual borrowed buffer views and full fallible Display formatting. The former copied-value/string APIs remain conveniences.
- The unchanged source matches 26,752 formatter records and 16,656 ownership records across four independent profiles, including 8,952 native abort observations. Complete caller-supplied destructors preserve actual-cell mutation, field/local cleanup order, moving/forgetting/storing values and later fresh runtime scopes. Both original Rust tests pass in every profile; earlier numeric/failure fixtures remain byte-identical.
- Added a neutral consuming-operation guard and explicit borrowed runtime metadata. No-context ownership conveniences release earlier scope metadata; contextual operations use the caller's current scope. The runtime support gains no standard-library file credit.
- Regenerated all twelve lexer/literal contract scopes and the literal dependency's full scalar proof against the isolated snapshot. Existing compressed fixture bytes remain unchanged; helper bindings reflect the runtime metadata addition.
- The isolated minimum-Go snapshot passes the full race suite and vet, including the Rust/Cargo fixture comparisons. Fresh Base-N formatter/ownership regeneration and original source tests pass; hosted validation now runs both new contract generators.

The ledger now records 29 completed top-level files. Cargo graph's persistent-container/generic contracts remain reopened. Full compiler sessions, symbols, source files, hygiene, definition IDs, source maps and their required sorting/container dependencies continue in parallel. Production Rust compilation remains unfinished.

## 2026-11-04 — checkpoint 24: verified collection and sorting support

- Added the actual hash-table behavior required by compiler unordered collections: probing/deleted buckets, growth and in-place rehash, source layouts, Entry variants, borrowed cells, clone rollback and owned cleanup. Two independent SDK profiles match 96,000 table operations, 1,098 owned-trait cases, 864 process cases per profile, 43,160 formatter cases and the scalar/layout/hash/default-dispatch proofs.
- Added Rust's normal stable-sort strategy with its source layout/Copy/Freeze metadata, small-sort selection, scratch sizes and panic hole restoration. The original SDK matches 98,720 requests and 7,978,963 comparison events, including 89,414 caught panics, final slot state, pointer equality and destructor order. Go storage supplies allocations; no Rust ABI or allocator contract is claimed.
- Added genuine partial UnordMap/Set foundations and their borrowed queries, traits and commutative stable hashing. The complete unchanged compiler graph matches 7,662 operation/format/hash records. Higher iterator algebra, conversion, sorting and codec methods remain under construction, so `unord.rs` receives no full-file credit.
- Closed the supporting iterator distinction between Map's `for_each` and Set's mapped `fold`, including constructor overrides and owned values. The previous semver collector evidence remains unchanged. The scoped stable-hash context bridge now supplies the real backing hasher without a dependency cycle; its source file remains partial.
- The isolated minimum-Go snapshot passes affected-package race checks and vet. Every retained backing/native sort/Unord proof regenerates successfully, with five source/tool integrity tests. Hosted validation now runs both SDK references alongside the complete original compiler graph.

The source ledger remains at 29 completed top-level files. These dependencies unblock larger complete compiler-file ports; neither supporting hash-table/sort code nor partial unordered collections gains whole upstream source-file credit.

## 2026-11-04 — checkpoint 25: complete strict crate-version hash

- Ported the full Svh module used to identify changed or mismatched crate versions: construction/projection, fixed32-digit hex text, Copy/Clone/equality, full Hash/default hash_slice, actual codec/stable-hash delegation and mutable fallible Debug/Display. There is no added ordering trait or hash-calculation algorithm.
- The complete unchanged compiler crate matches57,540 native records, including50,508 exact formatter failure cuts. Both character/string writers,23 format patterns, reused formatter state and the neutral Fingerprint Debug entrypoint match. Direct and concrete-state dictionary hashing retain default raw writes, typed overrides and exact/reference/wrapped Unhasher behavior in both assertion configurations.
- Independent source/API review found no gaps. The isolated actual Go1.23 snapshot passes affected-package race tests and vet; fresh original-source preparation/regeneration and source inventory verification pass. Existing Fingerprint/hash fixtures remain unchanged.
- Checkpoints23 and24 passed all hosted Linux, macOS, upstream and native-reference jobs. Shared span/source-map work remains under integration, including an explicitly scoped ordinary-panic/hook protocol and separate resume-unwind behavior; Cargo graph, hygiene/codecs and collection work continue in parallel.

Thirty top-level source files now have completion evidence. The caching view and other larger span files have retained local native comparisons but receive no premature completion record. Production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 26: reliable native ownership checks

- Checkpoint25 passed the Linux, macOS and complete original-span reference jobs. Its upstream comparison job hit the Base-N ownership client's three-second per-process limit during the last profile, after the first three profiles passed.
- Increased that isolated observation limit to30seconds and included the exact profile/request in timeout diagnostics. The bounded process still fails on timeout or any unexpected termination; the source corpus and accepted results are unchanged.
- Fresh regeneration from an isolated committed snapshot passes all16,656 ownership observations and8,952 actual native aborts across all four profiles. Original tests pass in each profile. All raw and compressed fixture hashes, request hashes and implementation bindings remain unchanged.

The ledger remains at30 completed top-level source files. Shared panic integration, full span/source-map files, unordered collections and Cargo graph continue in parallel; production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 27: explicit panic operations and cleanup integration

- Added a bounded ordinary-panic protocol with exact caller-supplied payload kinds/source locations, formatting before hook dispatch, separate resume-unwind, and real compiler cleanup-abort metadata. Payloads retain their original identity. There is no Go-global interceptor or standard-library source-file credit.
- Migrated the accepted ownership/collection/sorting guards to forward existing panics without invoking their hooks again. Only a proven second panic escaping required cleanup dispatches the selected runtime's cleanup diagnostic before abort.
- Fixed lexer owned callback entrypoints that dropped active runtime metadata. They now use the current caller scope, including an explicit scope with no observer, and restore the cursor's prior scope after the borrow.
- Compared45 original-runtime cases/24 native aborts,182 original-component scripts/107 native aborts,312 actual SDK collection/sort processes/258 native aborts, and12 unchanged stable-hash dependency cases/6 native aborts. These retain exact hook kinds/locations/flags, current observer scopes, real borrowed cells, local/field/argument destruction, final-return-place leaks and observable abort prefixes.
- Fresh regeneration of all twelve lexer/literal scopes, the literal dependency's full scalar trace, Base-N ownership and stable sorting retains all prior raw and compressed fixture bytes. New source/tool bindings are refreshed from an isolated minimum-Go snapshot.
- The isolated Go1.23 full race suite and vet pass, as do optional semver/stable-hash feature races and the added component/collection guard checks. Hosted validation now regenerates the new runtime, dependency and component proofs against their exact original toolchains.

The ledger remains at30 completed top-level source files. Full span/session/symbol/hygiene/source-map files, Cargo graph, unordered collections and their required dependencies continue in parallel. Production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 28: unstable and cached-key sorting

- Added the scoped original SDK's unstable sorting strategy, including small sorting networks, partition selection and heap fallback. The actual implementation matches103,895 requests and11,487,102 comparison events, with94,502 caught panics and exact final slot/destructor commitments.
- Added cached-key sorting with owned mutable key functions, actual tuple PartialOrd defaults/overrides, u32/usize index selection, stable permutation and deterministic key/function destruction. The original SDK matches5,768 ordinary cases,121,352 function calls,81,600 key drops and121,472 comparisons.
- Retained99 ambient-context cases/36 native aborts and198 explicit-hook processes/72 native aborts, covering resumed callbacks, hook ordering, remaining key cleanup and function/field destructor boundaries.
- Independent isolated Go1.23 sorting race/vet, fresh actual-SDK regeneration for all three new corpora and four tool-integrity checks pass. These adapters receive no whole std or Unord source-file credit.
- Added explicit cold-cache hydration for the component proof's separate pinned lexer input. A real local Git partial clone verifies offline rejection, explicit fetch, exact revision/path identity and subsequent offline reuse. Fresh182-case component regeneration retains every native fixture byte.
- Checkpoint26 passed all four hosted jobs. Checkpoint27 passed Linux, macOS and the complete original-span reference job; its upstream comparison job remains in progress.

The ledger remains at30 completed top-level source files. These sorting operations support the remaining complete compiler collection ports. Full span/source-map/hygiene, Cargo graph and current-context typed-vector work continue; production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 29: current runtime through typed vector ownership

- Added explicit current-context clone/clone_from, push, extend, collection and owned-iterator operations on the established IndexVec storage. Element/iterator operation dictionaries receive the caller's metadata; vectors and escaping iterators retain no construction-time runtime. Existing entrypoints remain empty-context conveniences.
- Source capacity failures now raise before the affected element/iterator cleanup. RawVec, trusted Extend and trusted collection failure sites retain their distinct observed source locations, even when the panic text is identical. Recover forwarding and actual cleanup-abort boundaries preserve the shared protocol.
- The complete unchanged UnordBag plus actual SDK Vec matches228 independent operation processes, including88 native aborts. Cases cover current observer B versus no observer, clone-prefix cleanup, catchable first destructor failures during an outer unwind, actual second-panic cleanup, resumed callbacks, iterator cleanup and zero-sized usize-maximum boundaries.
- The isolated actual Go1.23 race gates pass for storage, vectors and intervals (22.875s,77.990s and48.533s), retaining the earlier150,068-record corpus. Fresh native regeneration, vet and four integrity checks pass. The owner's default/nightly descendant checks also retain all legacy fixture bytes.

The ledger remains at30 completed top-level source files. This additive ownership bridge enables full unordered-bag and codec integration without a duplicate vector engine. Full compiler span/source-map/hygiene and Cargo graph work continue; production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 30: exact string slicing and Unicode 18 formatting

- Added scoped Rust string range indexing and character-boundary operations, including inclusive/exhausted ranges, borrowed successful results and all six slice-error priorities. Panic payloads retain exact Unicode character formatting, source excerpts and caller-supplied locations; current-context formatting starts before the hook.
- The actual official SDK matches197,024 requests,85,911 exact panic records and27 ambient cases, including8 real aborts. Source/channel/archive identities and licenses are verified;12 integrity checks include a separate checkout sharing the canonical toolchain. Go1.23 source-helper race/vet passes.
- Added an independent Unicode18 formatting proof that requires no unfinished Symbol implementation. Every valid scalar's char/str writer calls and public escape_debug result match1,112,064 native observations in272 SHA-256 blocks. The new profile coexists with the earlier Unicode17 comparison helpers and receives no whole standard-library source-file credit.
- Isolated minimum-Go formatter/string race checks pass (46.164s/4.841s), along with fresh native regeneration and SDK/source integrity checks. All12 lexer/literal scopes and the literal dependency's complete scalar trace regenerate with unchanged raw and compressed fixture bytes; their helper bindings are refreshed.

The source ledger remains at30 completed top-level files. The compiler's larger span/source-map/hygiene group now consumes these exact operations; full Cargo graph validation is also ready for independent acceptance. Production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 31: native Unix errors and retained panic metadata

- Added the scoped SDK Unix error formatter: actual XSI strerror_r, the source's zeroed128-byte buffer/negative-result rule and exact lossy CStr writer chunks. No Go errno table or inferred capitalization supplies Rust error text. Explicit bindings support builds without cgo; this gives no whole std/libc file credit.
- The standalone actual-SDK comparison passes84,123 errno/CStr requests, including all non-NUL one/two-byte strings, seeded longer invalid UTF-8 and fallible writer calls. Isolated Go1.23 race/vet and no-cgo execution pass;386 is compile-only. New independent Linux/macOS jobs compare each host's real Rust std and libc and retain host reports.
- Corrected Fingerprint's safe custom-decoder short-slice panic to retain the actual owned-string kind and optional selected-source location. A separate unchanged-source client retains24 normal/inherited/processing records and6 real aborts for both packed and unpacked decodes. The original55,292-row fixture and36 native abort cases regenerate unchanged, as do all182 integrated component records/107 native aborts. Refreshed their production bindings after the earlier cleanup-runtime integration.
- Hosted checkpoints27–29 had passed Linux, macOS and the original-span jobs, then failed the stale Fingerprint manifest hash. Checkpoint30's native string/Unicode comparisons passed, but a tool-selection test used a basename rustup cannot dispatch. The test now preserves rustc's required command name while checking selection through another PATH directory; all12 integrity checks pass.

The source ledger remains at30 completed top-level files. Full Cargo graph acceptance and coherent span/session/hygiene/source-map integration continue. Production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 32: borrowed locations and assignment ownership

- Added an explicit immutable location-cell profile. Repeated raises can now borrow the same actual cell through the hook; value-map conveniences still return independent copies and missing profiles stay unlocated. The actual SDK matches107 scripts,1,283 hooks and20,635 within-process identity comparisons. Numeric addresses are not compared between languages.
- Added the scoped live-destination drop-and-replace operation needed by compiler collections. It consumes the replacement, keeps the destination cell and installs the replacement's complete destruction operations after either successful old Drop or catchable old Drop failure. The original Rust assignment and default clone_from match72 cases, including30 real native aborts. Independent isolated Go1.23 race/vet passes.
- Refreshed all12 lexer/literal comparison bindings and the original panic/component/collection protocols against the new neutral helper. Existing native observations and compressed fixtures remain unchanged. These supporting operations receive no whole standard-library source-file credit.

The source ledger remains at30 completed top-level files. The complete Cargo graph algorithms and original tests have passed independent acceptance; its final shared stdio/capture/locking dependency is still being completed. Full span/session/hygiene/source-map and collection ports continue. Production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 33: complete Cargo graph source contracts

- Restored completion credit for the entire303-line Cargo src/util/graph.rs through the primary OwnedGraph interface. Genuine Rc copy-on-write containers preserve shared subtrees, actual borrowed nodes/edges, clone/compare/default overrides, lazy iterators/equality, mutations left by failures, B-tree paths/sort and exact owned cleanup. All prior historical comparison fixtures remain retained.
- Independently regenerated66,302 main and1,360 equality observations across four assertion/overflow profiles, including9,924 actual native aborts. All three unchanged original tests pass in each profile. Independent minimum-Go full Graph replay and targeted new races pass.
- Added a genuine shared stderr runtime with explicit source-thread identities, reentrant whole-statement leases, owned capture remove/restore/Drop, poison state and exact Vec byte capacity. It matches178 actual SDK1.99/1.101 cases/20 native aborts. Current Context flows through every operation; IO guards release before the component formats its panic. Whole std/Arc/TLS implementation credit is excluded.
- The original complete graph matches72 additional real stdio compositions: captured/global output, pre-held locks, repeat prints, exact raw writes and capture bytes/capacity/Arc counts. Eight EIO hooks independently prove another thread can acquire stderr before hook return. The actual stable SDK also matches4,451 errno/writer records; native libc supplies Unix messages.
- Added borrowed selected-build profile metadata independently of tracing payload/panic operations, so compiler callsite metadata can retain actual build spellings. All12 lexer/literal scopes, complete literal scalar trace, Base-N ownership and original runtime/component/collection protocols regenerate with unchanged native bytes. Fixed stdio compiler selection to preserve rustup's executable basename.
- Both checkpoints31 and32 passed all six hosted jobs. CI now regenerates full Graph/stdio proofs and permits30minutes for the complete Graph race corpus without reducing cases or assertions.

The source ledger is31 completed top-level files. Independent reviews of the coherent span/session/symbol/hygiene/source-map group have found additional trait/default, ownership and formatting boundaries; those remain open until fixed and validated. Production Rust compilation remains unfinished.

## 2026-11-05 — checkpoint 34: shared span collection, tracing and codec prerequisites

- Added scoped IndexMap operations over the genuine stored-hash vector/SwissTable backing: owned generic builders, Clone/CloneFrom, Entry/IndexedEntry, borrowed and consuming iterators, projections, range/drain and positioning. Independent native checks match18,000 stateful operations,17,904 indexed-entry cases,2,496 formatter-state cases and the range/drain corpus with164 guard cases/39 actual aborts. Additional retained corpora cover the other implemented interfaces; the remaining API stays outside this partial dependency port.
- Added scoped tracing dispatch, callsite filtering, field visitors, span ownership and explicit caller scopes. Independent regeneration matches384 ordinary states and20 failure scripts/8 native aborts. Review closed processing-panic cleanup, actual RefCell/zero-ID payloads, selected verbatim locations and writer-only registry poisoning, including the acquired guard's lifetime through the hook.
- Added actual primitive memory/file serializers, deterministic map defaults and owned callback handling. Four unchanged-source clients match4,418 observations. Unordered-bag storage and generic vector/container codecs add5,626 observations, including602 ambient-context and32 wrapper-specialization native aborts. Distinct encoder wrappers now retain generic per-byte calls; explicit dictionaries select the real concrete opaque specialization.
- The isolated actual Go1.23 snapshot passes full IndexMap/hash-table races (284.714s/72.162s), tracing race (0.371s), new bag/codec races (8.674s), serializer race (0.512s), vet and nine codec-integrity tests. Native clients revalidate their pinned references. All12 lexer/literal scopes regenerate with unchanged native fixture bytes and refreshed bindings for the added formatter helper.
- Hosted CI now runs the new native proofs. Race checking remains enabled; child-process checks skip the race runtime's one-second exit sleep, preserving all cases, assertions and timeout enforcement.

The source ledger remains31 completed top-level files. These scoped prerequisites prepare the complete span/session/symbol/hygiene/source-map block; they grant no whole IndexMap, tracing, serializer, unordered-collections or standard-library source-file credit. Production Rust compilation remains unfinished.
