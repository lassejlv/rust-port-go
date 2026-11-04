# Thousands validation

Pinned compiler: `rustc 1.99.1 (58cf8f9c3 2026-10-13)`, Linux x86-64.
Minimum Go validation uses the actual cached `go1.23.12 linux/amd64` toolchain.
The reference source and original tests are identified completely in
`port/rust/compiler/rustc_data_structures/src/thousands/oracle_manifest.json`.
The package also cross-compiles for `GOARCH=386`; this container cannot execute
32-bit binaries (`exec format error`), so no 32-bit runtime result is claimed.

All four unchanged original Rust tests pass. The Go port passes their complete
35 assertions and 47,827 retained original-source reference cases:

| Kind | Cases | Coverage |
| --- | ---: | --- |
| `f64` | 42,181 | Every exponent with six mantissa boundaries and both signs; 12,000 seeded arbitrary bit patterns; halfway rounding and adjacent representable values; powers of ten and neighbors; zero, subnormals, maximum finite, infinities and signed quiet/signaling NaNs with payload boundaries |
| `usize` | 2,297 | Host extremes, powers of two/ten and neighbors, seeded values |
| `isize` | 2,297 | Signed reinterpretations of the same boundaries, including minimum and maximum |
| Private string helper | 1,052 | Original cases; digit-run lengths 0–64; signs, fractions, exponents, underscore suffixes, Unicode and embedded controls |

Uncompressed original-output SHA-256:
`0335da6963720f688b0fb68b48fe0095cadd21f6925725ef9f3df94030adbbb7`.
The complete gzip fixture is retained in the Go package, so its behavior stays
testable without a reference compiler. No reference cases are skipped on the
64-bit validation host.

From the repository root, regenerate with:

```sh
go run ./tools/port-rust-thousands
```

CI or a reviewer can verify the live original without rewriting evidence:

```sh
FERRUGO_RUSTC=rustc go run ./tools/port-rust-thousands -check
FERRUGO_RUSTC=rustc go test -race ./port/rust/compiler/rustc_data_structures/src/thousands ./tools/port-rust-thousands -count=1
go vet ./port/rust/compiler/rustc_data_structures/src/thousands ./tools/port-rust-thousands
GOARCH=386 go test -c -o /tmp/ferrugo-thousands-386.test ./port/rust/compiler/rustc_data_structures/src/thousands
```

`-check` compares the decoded reference bytes and complete manifest. Both check
and regeneration preserve any existing valid gzip representation of identical
raw results. A tool regression test uses `gzip.NoCompression` to verify this
independently of the current Go compressor implementation and checks that
changed raw output is rejected without modifying the file. Go 1.23 generation
and Go 1.27 checking both preserve the same retained compressed fixture.

Additional Go tests cover invalid UTF-8 rejection, native integer limits,
negative zero, NaN signs, exact halfway rounding and neighboring floats. The
production implementation and reference tests pass race-enabled testing and
vet under Go 1.23. This evidence applies to the complete thousands utility and
its original tests, not to overall compiler diagnostics or the full Rust port.
