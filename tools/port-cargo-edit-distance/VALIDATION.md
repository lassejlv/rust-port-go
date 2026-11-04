# Cargo edit-distance validation

Validated source mapping: Cargo `src/util/edit_distance.rs` at `24de4721844c7a4d839072bab6fb66d41c9bc1f4` → `port/cargo/src/util/edit_distance.go`.

Git blob: `e67259956fc26a53b1be30efb371161249dfff8a`.
Raw source SHA-256: `073eb1a086cd2d2e74d1d8ce98b87013a8756f0187313ea1b7340b63184c6407`.
Oracle build profile: Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), Unicode 17.0.0. The separate rustc main source pin has Unicode 18; it is not this oracle profile.

Completed checks:

- Entire upstream Rust test passes when compiled directly from the pinned source.
- Entire translated upstream test passes, including its Unicode scalar identity loop and six multiline-string comparisons.
- All 1,112,064 Unicode scalar values match standalone Rust lowercase results and both contextual-final-sigma probes. This covers 1,488 changed/expanded lowercase mappings, 151 effective cased ranges and 464 case-ignorable ranges.
- 18,853 original-source distance cases match: exhaustive short ASCII/Unicode alphabets across limits, 10,000 deterministic random Unicode pairs, transpositions, prefix/suffix trimming, expansions and contextual mappings.
- 1,005 original-source generic closest/message cases match, including first-minimum ties, empty iterators, empty-string candidates, cutoff misses and exact message bytes.
- Explicit restricted-Damerau versus unrestricted-Damerau and callback-evaluation tests pass. Invalid UTF-8 is rejected explicitly.
- Live opt-in original-source compilation and byte-for-byte oracle-stream comparison pass.
- Deterministic regeneration with `-check` passes; source, generated tables and compressed/raw reference fixtures have recorded SHA-256 checksums.
- `go test -race -count=1 ./port/cargo/src/util` passes.
- Ten seconds of two-worker fuzzing completed 168,654 executions without failure.
- `go vet ./port/cargo/src/util ./tools/port-cargo-edit-distance` passes.

Representative commands, from the repository root:

```sh
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test -count=1 ./port/cargo/src/util
GOCACHE=/tmp/ferrugo-go-cache FERRUGO_RUSTC=/workspace/rust-port-go/.toolchain/rust/bin/rustc .toolchain/go/bin/go test -count=1 -run '^TestEditDistanceLiveRustOracle$' ./port/cargo/src/util
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-cargo-edit-distance -check
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test -race -count=1 ./port/cargo/src/util
GOCACHE=/tmp/ferrugo-go-cache GOMAXPROCS=2 .toolchain/go/bin/go test -run='^$' -fuzz='^FuzzEditDistance$' -fuzztime=10s -parallel=2 ./port/cargo/src/util
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go vet ./port/cargo/src/util ./tools/port-cargo-edit-distance
```

These results validate this complete Cargo utility file for the stated standard-library profile. They do not establish completion of Cargo, rustc or another Unicode runtime profile. Production execution uses only Go and its compiled-in pinned tables.
