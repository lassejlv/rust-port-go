# base-N validation

The implementation's whole-file completion record has been reopened: the full
Display/writer contract and owned generic conversion behavior remain under
review. The comparisons below validate the implemented numeric operations and
source tests, not completion of the remaining contracts. See
`docs/SOURCE-CONTRACT-AUDIT.md` at the repository root.

Original-source mappings, at Rust commit `2d20b42b883dcfc014f3c2c56081eeb6091c2179`:

- `compiler/rustc_data_structures/src/base_n.rs` → `port/rust/compiler/rustc_data_structures/src/base_n.go`; Git blob `0c3d7613d4f1ff69a31c4458bd571074bbd5c618`.
- `compiler/rustc_data_structures/src/base_n/tests.rs` → `port/rust/compiler/rustc_data_structures/src/base_n/tests_test.go`; Git blob `148d8dde02af1b12433fa87509a742d1e59c3421`.

Both full original source files are read from Git and checked against their SHA-256 hashes before use. The oracle compiles their unchanged bytes using Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), x86_64 Linux, with overflow checks enabled. Its wrapper enables `ascii_char` and `ascii_char_variants`, with `RUSTC_BOOTSTRAP=1` limited to the validation compiler process.

Completed checks:

- Both complete upstream Rust tests pass, including the non-Miri 1,000-value loop for every base 2 through 36.
- Both complete Go translations pass with independent `math/big`/`strconv` parsing, including all maximum unsigned values.
- 74,781 original-source supported-base cases match exactly. Every base 2 through 64 and each of `u32`, `u64` and `u128` is checked for encoded length, ordinary encoding and fixed-length encoding. Inputs cover zero, every power-of-two boundary and its neighbors, maximum values, limb boundaries and deterministic random values retaining all 128 bits.
- 543 original-source invalid-base cases match success versus panic and exact successful output. They include zero divisors, one-base buffer exhaustion, oversized digits, oversized bases that still succeed, and native-width-to-`u32` divisor truncation.
- 1,225 additional original-source external-newtype cases match output and exact operation traces. They cover custom lengths 0, 1, 5, 128, 129 and `usize::MAX`; shortened/truncated suffixes and empty strings; independently overridden normal/fixed methods; one-time conversion; overridden default dispatch; bases zero/one; and calls that panic before later callbacks are reached. Excess custom lengths panic during the fixed-length call itself, before string access.
- Exact digit alphabet, initialized-suffix `String`/`AsRef`/display behavior, value-copy independence and fixed-length lexical ordering are checked.
- A live opt-in Go test compiles and runs the original source and compares its entire output to the committed oracle stream.
- `-check` regeneration verifies both source blobs, original tests, request stream, wrapper and compressed/raw fixture checksums without changing committed files.
- `go test -race -count=1` passes, including the live original-source comparison.
- Ten seconds of two-worker fuzzing completed **274,953 executions** without failure. It checks all 128 input bits in bases 2 through 64 with an independent arbitrary-precision decoder, minimal representation and exact fixed-length zero padding. The same fuzz target checks custom widths 0 through 255, including truncation, empty suffixes, immediate excessive-width panics and conversion/length call order.
- `go vet` passes for the port and generator.
- The package and tests cross-compile for 32-bit x86. This execution host rejects 386 binaries with `exec format error`, so no 32-bit execution claim is made. All original-source runtime comparisons were on the pinned 64-bit profile.

Unsupported base-one length computations cannot produce a normal encoded length: the maximum value is never reduced. Those loops are excluded from the terminating oracle stream; their counter-overflow behavior depends on the Rust overflow-check build profile. No supported-base case depends on this distinction. Panic diagnostic wording and native-language formatting flags are not asserted equivalent.

Representative commands from the repository root:

```sh
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test -count=1 ./port/rust/compiler/rustc_data_structures/src/base_n
GOCACHE=/tmp/ferrugo-go-cache FERRUGO_RUSTC=/workspace/rust-port-go/.toolchain/rust/bin/rustc .toolchain/go/bin/go test -race -count=1 ./port/rust/compiler/rustc_data_structures/src/base_n
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-rust-base-n -check
GOCACHE=/tmp/ferrugo-go-cache GOMAXPROCS=2 .toolchain/go/bin/go test -run='^$' -fuzz='^FuzzBaseN128$' -fuzztime=10s -parallel=2 ./port/rust/compiler/rustc_data_structures/src/base_n
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go vet ./port/rust/compiler/rustc_data_structures/src/... ./tools/port-rust-base-n
GOCACHE=/tmp/ferrugo-go-cache GOARCH=386 .toolchain/go/bin/go test -c -o /tmp/ferrugo-base-n-386.test ./port/rust/compiler/rustc_data_structures/src/base_n
```

These results validate the stated base-N operations and its complete test module for the stated profile. The production implementation imports only `math/bits`; Rust and the reference fixture are used exclusively for validation.
