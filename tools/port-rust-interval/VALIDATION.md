# Interval source-port validation

Whole-file mappings at Rust revision `2d20b42b883dcfc014f3c2c56081eeb6091c2179`:

- `compiler/rustc_index/src/interval.rs` → `port/rust/compiler/rustc_index/src/interval.go`; source Git blob `475c81e66ab09229137350775db615e39df24eee`.
- `compiler/rustc_index/src/interval/tests.rs` → `port/rust/compiler/rustc_index/src/interval/tests_test.go`; source Git blob `cf3222e6c6572303939f620858a4d98a90ba2bbe`.

Validation uses actual **Go 1.23.12** in both the ordinary and `ferrugo_rustc_nightly` build profiles. The independent oracle uses **Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), x86_64 Linux**, enabling the real Step trait with `RUSTC_BOOTSTRAP=1` during validation compilation. That compiler commit differs from the original source commit; this is the recorded standard-library/compiler profile, not a claim that the entire pinned compiler has been built.

The tool checks both target source files and the actual original `idx.rs`, `slice.rs` and `vec.rs` against fixed Git blob IDs before copying their bytes unchanged to a temporary build. Typed storage is compiled from those original files, without dependency shims. SmallVec 1.15.1 is compiled from the real crate archive, verified against its pinned Cargo.lock SHA-256 `67b1b7a3b5fe4f1376887184045fcf45c69e92af734b7aaddc05fb777b6fbd03`. It uses the dependency's default feature set. Both the original storage dependency and interval executable are rebuilt with each profile's matching flags.

Completed checks:

- All **seven complete upstream Rust tests** pass directly from the unchanged original test module.
- All **seven complete Go translations** pass, including all nested boundary loops in `insert_range` and `last_set_in`.
- **40,380 original-source records** match Go. Every record compares the return value or panic, complete ordered generic callbacks and the final state of both sets and all materialized matrix rows.

| Rust overflow checks | Rust debug assertions | Matching records |
| --- | --- | ---: |
| Enabled | Enabled | 10,095 |
| Disabled | Disabled | 10,095 |
| Enabled | Disabled | 10,095 |
| Disabled | Enabled | 10,095 |

Coverage includes every source method; included/excluded/unbounded endpoints; standard range factories and arbitrary range implementations; independent range cloning/start/end callbacks; u32 truncation and arithmetic boundaries; empty, small and machine-maximum domains; missing rows, row growth and aliased indices; mismatched union domains; one-time and repeated callback failures; independent comparison operators; exact iterator size hints; and catching a lazy iterator panic and then resuming that same iterator. Custom keys contain slice-backed metadata and therefore are not Go-comparable. The corpus includes **384 deterministic 32-operation random programs**, tested in every profile.

Additional focused Go tests verify independent deep set/matrix cloning; the optimized union's counterintuitive bool result; custom Step default/override dispatch and unordered comparisons; cloned-start/original-end range evaluation; and exhausted inclusive range bounds.

Both Go 1.23 build profiles pass live original-source comparison plus **race detection**. Both vet configurations pass. Deterministic `-check` regeneration passes without changing the fixture or manifest. Compressed/raw fixture data, original source bytes, dependency archive, original storage wrapper, request streams and response streams all have recorded checksums.

Ten seconds of two-worker fuzzing completed **24,179 executions** without failure. It checks mutable sets and matrices against an independent Boolean membership model, including insertion and append, range merging, clear/insert-all, union, superset/disjoint queries, first-unset/last-set queries, sparse-row growth and row unions. The differential fixture separately verifies source-specific return flags, panic states and generic callbacks that a mathematical membership model cannot establish.

Representative commands from the repository root (using the actual installed Go 1.23 compiler):

```sh
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -count=1 ./port/rust/compiler/rustc_index/src/interval
GOCACHE=/tmp/ferrugo-go-cache FERRUGO_RUSTC=/workspace/rust-port-go/.toolchain/rust/bin/rustc .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -race -count=1 ./port/rust/compiler/rustc_index/src/interval
GOCACHE=/tmp/ferrugo-go-cache FERRUGO_RUSTC=/workspace/rust-port-go/.toolchain/rust/bin/rustc .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -race -tags ferrugo_rustc_nightly -count=1 ./port/rust/compiler/rustc_index/src/interval
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go run ./tools/port-rust-interval -check
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go vet ./port/rust/compiler/rustc_index/src/interval ./tools/port-rust-interval/...
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go vet -tags ferrugo_rustc_nightly ./port/rust/compiler/rustc_index/src/interval ./tools/port-rust-interval/...
GOCACHE=/tmp/ferrugo-go-cache GOMAXPROCS=2 .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -run='^$' -fuzz='^FuzzIntervalState$' -fuzztime=10s -parallel=2 ./port/rust/compiler/rustc_index/src/interval
```

The fixture pins a 64-bit usize profile; ordinary focused tests remain architecture-independent. Production behavior uses the shared Go typed storage and Go standard-library packages exclusively. The parent crate's nightly module exposure and the typed-storage files' separate nightly codecs are described explicitly in `PORT.md`; they receive no completion credit from this interval validation.


## Owned-range and full-formatter follow-through

The additional compatibility corpus at `compat/` retains **22,000** native
records and **106** isolated source aborts. Every reference independently
rebuilds the complete unchanged 464-line implementation, its complete 199-line
unit tests, actual typed storage and actual pinned SmallVec 1.15.1 for each
arithmetic/debug profile. All seven original tests pass in each profile.
The original 40,380-record corpus and compressed bytes remain unchanged.
The combined ordinary comparison count is 62,380; abort records are separate.

The primary `IntervalSet.FormatDebug` streams struct/list/two-field tuple and
PhantomData Debug through full caller options and independent fallible
string/character writes. It preserves mutable formatter state and reuse after
errors. Type names come from explicit canonical Rust metadata, never a Go
reflection approximation. SparseIntervalMatrix receives no extra Debug impl.

`IntervalRangeBounds` now owns clone/original destruction independently.
Context-aware clone/start/end/destructor callbacks and context-first operation
entrypoints preserve ambient source panic state. Required local cleanup uses
the proven component abort guard; ambient unwinding alone remains catchable.
Native cases execute ordinary, inherited-destructor and panic-hook contexts.
Processing-panic sites use `context.Panic` for immediate abort before callback
locals unwind. Every expected abort runs in an isolated child and compares all
prior events; Go exit 134 is the host abort representation, without signal ABI.
Matrix arguments are owned before row growth and cleaned if row lookup/growth
fails. Cloned range destruction precedes original end lookup, including when
start conversion or clone destruction fails. Existing plain callbacks remain
normal-context conveniences.

Actual Go 1.23.12 ordinary and tagged tests and native `-check` pass for the
extended corpus. Focused tests additionally verify normal/ambient clone-Drop
failure recovery, original cleanup before any range clone when matrix row
lookup fails, and rejection of a missing canonical type-name witness.
The live compatibility tool verifies the exact source/dependency/input hashes,
normal source output and isolated Go/source abort records. It preserves valid
historical gzip bytes across Go versions. See `compat/README.md` for commands
and scope; no helper/std/dependency file receives extra completion credit.
