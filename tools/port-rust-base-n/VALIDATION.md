# Base-N validation

The reopened Display and owned-generic contracts now have separate native
oracles and Go replays, in addition to the unchanged numeric corpus. These
results cover `base_n.rs` and its original test module, not the surrounding
compiler crate or any Rust ABI/layout.

Source revision: `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.

| Original | Git blob | SHA-256 |
| --- | --- | --- |
| `compiler/rustc_data_structures/src/base_n.rs` | `0c3d7613d4f1ff69a31c4458bd571074bbd5c618` | `090d62952cb4515c9de66489d6ce0e0f3cd5c3becd7e8e2c2bf39d119d4f9890` |
| `compiler/rustc_data_structures/src/base_n/tests.rs` | `148d8dde02af1b12433fa87509a742d1e59c3421` | `e4a58e24148ca8120d30d974fc4084ab08e0aa9ea2fd1d4b1101eb21c9155cde` |

Each reference compiles the complete original bytes without body edits. The
explicit comparison compiler is Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), x86_64
Linux. `RUSTC_BOOTSTRAP=1` enables `ascii_char` and `ascii_char_variants` for
validation only. The new clients independently run all four combinations of
`-C debug-assertions={false,true}` and `-C overflow-checks={false,true}`, and both
original tests pass in every profile. Debug assertions have no sites in this
source; arithmetic checking still affects invalid custom starts.

## Retained numeric and original-test evidence

The existing fixture, original test translation, numeric wrapper, generator and
manifest were left unchanged:

- 74,781 supported-base observations cover all bases 2 through 64 and all three
  unsigned widths. They retain every u128 bit, limb and power-of-two boundaries,
  neighboring values, maxima, zero and seeded inputs.
- 543 invalid-base observations retain success/panic and exact successful text,
  including zero divisors, base-one buffer exhaustion, out-of-alphabet digits,
  oversized bases that still work and u32 divisor truncation.
- 1,225 Copy-newtype observations retain custom widths 0, 1, 5, 128, 129 and
  `usize::MAX`, independent ordinary/fixed overrides and callback order. Their
  historical profile is checked arithmetic; unchecked behavior is now covered
  separately below.
- The two exact source-mapped Go tests retain the full non-Miri 1,000-value loop
  for every base 2 through 36 and maximum-value round trips using independent
  `math/big`/`strconv` decoding.
- The previous numeric fuzz run remains historical evidence: 274,953 executions
  in ten seconds, using two workers. It was not rerun for this contract-only
  restoration.

Frozen compressed SHA-256:
`98c322602c43542644357be7b7c727c9e8f3904cbda75e117e06ad4d9485e65d`.

## Borrowed Display

`contracts/format_oracle.rs`, `contracts/generate.py`, and
`base_n_format_contract_test.go` retain 26,752 original-source records. The four
native profiles produce identical raw answers. Cases include actual buffer
identity for Deref/AsRef/Display, empty suffixes, one write_str call even when
empty, all selected writer failure positions, repeated formatter use and mutable
formatter-state preservation. Nineteen format patterns cover width, precision,
fill (including Unicode and NUL), alignment, sign, zero pad, alternate and both
Debug-number flags. Display ignores those options exactly as upstream.

Frozen compressed SHA-256:
`1bbcae68d43132928a820165954b10008fd2619e720960c16595f7c44a7ff843`.

## Owned conversion and trait operations

`contracts/owned_oracle.rs`, `contracts/owned_generate.py`, and
`base_n_owned_contract_test.go` add 16,656 original-source records, 4,164 per
native profile. They use an actual non-Copy Rust Self with a destructor, two
ordered fields with their own destructors and local Drop guards. No Go-produced
answers are used as expected data.

The cases prove:

- Into and both independent ToBaseN overrides borrow their actual owned local;
  mutation is observed by Self Drop and field glue. Remaining consumed Self is
  destroyed before encoding and before the static `encoded_len` callback.
- Every combination of six failure bits (body, local Drop, Self Drop, field A,
  field B and static length) is tested for all four override combinations, both
  operations and all four ambient contexts.
- Normal execution, an operation inside an outer destructor with its own catch,
  an escaping operation inside an outer destructor, and actual panic-hook
  processing preserve thread-panicking observations and exact cleanup prefixes.
  Source-local second panics abort; inherited state alone does not force abort.
- Explicit argument destruction, forgetting and moving into longer-lived
  storage preserve ownership. Stored values are dropped after the outer catch,
  with a fresh normal runtime scope rather than stale operation metadata.
- Static widths 0, 1, 5, 128, 129 and `usize::MAX`, invalid bases and actual
  u32/u64/u128 implementations retain phase distinctions and divisor casts.
  Unchecked width 129/MAX constructs successfully and then fails at Deref;
  checked arithmetic fails during construction.
- Successful returned strings retain actual-object buffer identity across
  Deref, AsRef and Display. The Go replay also retains selected empty-buffer
  borrows across GC. Temporary argument borrows are never retained after drop.

Each profile has 1,384 successful observations and 2,238 isolated aborts. The
unchecked profiles have 502 construction panics and 40 Deref panics; checked
profiles have 542 construction panics. All 8,952 native abort observations retain
the complete before-abort trace. Go replays both arithmetic configurations and
shares only duplicate debug-profile abort executions, since the Go source has
no debug-only operations. The corpus remains complete in the fixture.

Owned compressed SHA-256:
`d399f784386fb7c4d7ae8c18c3a8f1f26756ae16d83185cc2ff8a7209b891f3b`.
Raw SHA-256:
`5188213502742ab0ba0d68ee0a753a1baf09255cf46a5371492787d6440f1d2c`.
`contracts/owned-manifest.json` also binds the exact source, request stream,
production/test/tool files and frozen shared ownership/context helpers. It
records the two retained fixture hashes without recompressing them.

## Current gates

The following passed with minimum Go 1.23.12 on this x86_64 host:

- Parent-package BaseN contract tests under `-race -count=1` (44.350 seconds).
- Complete original child test package under `-race -count=1`, including its
  live Rust source/test and unchanged numeric-fixture regeneration (6.271 seconds).
- `go vet` for the parent package, original child tests and legacy generator.
- Opt-in live formatter/owned regeneration, both with `--check`, through the Go
  test entry point (28.884 seconds). Both native profiles/source checksums and
  every observation matched without tracked writes.
- Full parent package/test binary cross-compilation for GOARCH=386. This host
  cannot execute 386 binaries, so this is compile-only evidence. The 64-bit
  native fixture is explicitly skipped on other pointer widths; no 32-bit
  ownership/runtime comparison is claimed.

Representative commands (choose the minimum Go binary on PATH):

```sh
GOCACHE=/tmp/ferrugo-go123-cache go test -race -count=1 ./port/rust/compiler/rustc_data_structures/src -run '^TestBaseN'
GOCACHE=/tmp/ferrugo-go123-cache FERRUGO_RUSTC=/absolute/rustc-1.99.1 go test -race -count=1 ./port/rust/compiler/rustc_data_structures/src/base_n
GOCACHE=/tmp/ferrugo-go123-cache go vet ./port/rust/compiler/rustc_data_structures/src ./port/rust/compiler/rustc_data_structures/src/base_n ./tools/port-rust-base-n
GOCACHE=/tmp/ferrugo-go123-cache FERRUGO_RUSTC=/absolute/rustc-1.99.1 FERRUGO_BASE_N_CONTRACTS=1 go test -count=1 ./port/rust/compiler/rustc_data_structures/src -run '^TestBaseNContractsLiveRegeneration$'
GOCACHE=/tmp/ferrugo-go123-cache GOARCH=386 go test -c -o /tmp/ferrugo-base-n-owned-386.test ./port/rust/compiler/rustc_data_structures/src
```

The live entry point gives each native generator a three-minute bound. Each
isolated Rust observation has a three-second bound and 64-KiB stdout/stderr
limits; core dumps are disabled. Go abort subprocesses have ten-second bounds.
Raw fixture reads are capped at 64 MiB. New generators preserve an existing
valid gzip stream when its decompressed bytes match, including across Go/gzip
versions. Ordinary Go tests require no reference compiler or network.

Primitive length computations at base one (or u32 cast-to-one) never reduce the
maximum and are excluded from the finite stream; no terminating result or
practical counter-overflow execution is claimed. Panic diagnostic wording and
raw Go callback-local defer timing are outside the source panic protocol;
translated panic sites must use the explicitly supplied Context.Panic.
