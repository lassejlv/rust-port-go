# Hash wrapper validation

The retained original-source corpus contains 8,635 requests and 85,483 records:

- 6,289 arithmetic/ordering/clone/default/conversion/hash records, including a
  Cartesian boundary set and 6,000 seeded full-width pairs of 128-bit values.
- 79,194 formatting records across both value types, 23 native formatting
  patterns and boundary values. Each case tests the successful write and every
  possible failing writer position, with both default and independent
  `write_char` dispatch. Records preserve attempted string/character chunks,
  stop position, return status and post-call formatter fill/alignment.

The original-source code itself checks both Clone/CloneFrom implementations,
the zero/default equality, both `FromStableHash` implementations and exact
`WriteU64` operands. Go tests replay every retained record and compare the
current implementation, including recovered formatter state after writer errors.
Full gzip CRC/EOF and compressed/decompressed SHA-256 checks bind the fixture.
The manifest binds source identity, actual compiler/host, dependency archive and
expanded source tree, request bytes, wrapper/generator/verifier, build manifests,
formats and outputs. Existing gzip bytes are preserved when decompressed results
agree, so regeneration does not depend on a compressor version.

Prepare the exact dependency once, then work offline:

```sh
python3 tools/port-rust-hashes/generate.py --fetch
python3 tools/port-rust-hashes/generate.py --check
go test -race -count=1 ./port/rust/compiler/rustc_hashes/src
go vet ./port/rust/compiler/rustc_hashes/src
```

The tool requires Python 3.11+, pinned local source objects and Rust 1.99.1.
`--root`, `--rustc`, `--cargo`, `FERRUGO_RUSTC`, `FERRUGO_CARGO` and `CARGO_HOME`
support absolute, repository-relative and bare PATH executable layouts. Empty
or invalid explicit executable overrides fail. Offline reads disable Git lazy
fetching; Cargo builds use `--locked --offline`. Registry extraction checking
reuses the audited Edition reference helper, without using any Edition algorithm.

Native comparison and the actual minimum Go 1.23 race/vet checks pass. A 386
build compiles, but this host rejects ELF32 execution, so 32-bit runtime behavior
has not been validated. The corpus does not establish whole-compiler ABI or
standard-library completion. Independent review is recorded before ledger credit.
