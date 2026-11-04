# Original semver dependency oracle

This development tool builds the unchanged official `semver` 1.0.28 crate and
compares its observations with the Go port. The source/API port and primary owned
IntoIterator/Vec collector passed independent review and native validation;
no general stdVec module port is claimed. It has no
production Rust fallback.
`report.json` records the locked dependency checksums and retained fixture hashes;
`port/dependencies/semver/v1_0_28/source-manifest.json` inventories all 25 archive
members and maps all 15 Rust source files.

Run with Go 1.23 or later and Rust 1.99.1 (the pinned formatter/Unicode reference):

```sh
cargo fetch --locked --manifest-path tools/port-semver/oracle/Cargo.toml
go run ./tools/port-semver -check
```

The first maintainer generation omits `-check`. Existing gzip bytes are retained
whenever their complete decompressed data match the new original output. Check
mode never rewrites data; it validates gzip CRC/end-of-stream and exact native
bytes, so the same committed artifacts work with Go 1.23 and newer compressors.
Native executables and outputs are built under ignored `.toolchain/oracles`.

`-cargo`, `-rustc`, `-go` accept absolute/relative paths or bare PATH names;
`FERRUGO_CARGO`, `FERRUGO_RUSTC`, and `FERRUGO_GO` select the same executables.
Without a selection the installed local toolchain is preferred, then PATH.
An explicitly invalid or empty override fails instead of falling back.
`-cargo-home` or `CARGO_HOME` selects the registry cache. With neither set the
existing project cache is used if available, otherwise the host's `.cargo`.
Fetch and check must use the same selected cache; the checker stays offline.
Example for this workspace:

```sh
CARGO_HOME="$PWD/.toolchain/port-cargo-home" \
  go run ./tools/port-semver -check -cargo cargo -rustc rustc -go go
```

All 12 locked registry archives are SHA-256 verified, and every expanded file
is compared with the corresponding archive before and after native builds.
Symlinks and unexpected injected files are rejected. The exact semver archive
checksum is independently fixed in the generator. Metadata and support files
are retained verbatim with both upstream licenses; Go production code uses
its standard library and neutral Go formatting/ownership helpers. Transport-only serde_json and its locked dependencies
belong to the development oracle, not the production implementation.

The deterministic core corpus has 33,982 requests covering all parser kinds,
error order, numeric overflow, invalid Unicode lexemes, randomized UTF-8 inputs,
all requirement operators and raw public comparator states, normal/precedence
ordering, long numeric segments, Clone/equality, native hash bytes and optional
primitive/prefix-free callbacks, Display options, and failing writers.

The serde corpus has 3,136 requests covering all original Serialize/Deserialize
implementations, caller overrides and errors, all inherited visitor input kinds, default invalid-type composite Display
writes into caller-owned fallible C/S sinks, lazy Expected invocation,
128-bit values, f32 forwards and float extremes, exact expected messages, plus
JSON bytes, data/syntax error classes, lone surrogates, receiver retention, and
early rejection order, including invalid trailing/container-child UTF-8 and
UTF-16 data that must not preempt an earlier visitor error. JSON syntax message wording is deliberately represented
by its class in the cross-codec probes; this does not claim a serde_json port.

The owned collector corpus adds 4,142 unchanged semver requests covering actual
input/iterator cells, IntoIter construction and Drop, empty/nonempty generic
collection, size hints and growth, trusted lengths, independent for_each/fold
callbacks, concrete Vec::IntoIter reuse, and a static in-place/map specialization
witness. Resulting values, physical slice capacities, logical reuse, panic and
normal/inherited/processing callback order match the original. Another 30 native
subprocess cases compare abort classes and exact prefixes for processing panics
and second destructor panics. Prepared iterator return values remain leaked when
input destruction panics, as in Rust. The native Comparator size/alignment is
recorded and checked against the selected collector profile.
Another 108 original-crate cases check Clone and the provided CloneFrom for all
six Clone types. Requirement clones preserve exact source length/capacity and
independent mutable storage, including replacement of a larger destination.

The `VecFromIterator` callback is a concrete iterator type's monomorphized std
operation witness, not a user-overridable Rust algorithm. Nil must only describe
an actual generic/trusted strategy; known extra private std specializations
supply a witness. The supporting adapter covers the used contracts and does not
claim all stdVec code, allocator ABI, or every standard iterator-type adapter.
`rustvec` is neutral supporting code, with no compiler-package dependency and no
additional source-file completion credit.

The original crate is compiled with each of `std`, `no_std`, `serde`, and
`no_std + serde`. Every profile produces identical core observations, and both
serde profiles produce identical visitor observations. The prefix-free hasher
probe uses `RUSTC_BOOTSTRAP=1` only for the oracle's implementation of the std
experimental `write_str`/`write_length_prefix` hooks; the unchanged semver source
itself uses no unstable language feature or production shim.

Six retained Debug corpora contain 10,500 original native records. They compare
not just rendered text but every attempted sink call, accepted prefix, injected
failure, string-versus-character callback, and before/after formatter state.
They cover all seven public Debug types, compact/pretty/hex/width/sign/fill/zero/
precision options, nested Vec/Option and empty writes, Unicode character errors,
and the source's error-path padding-state retention. The character escape table
is independently regenerated from all valid Rust Unicode 17 scalar values.

The checker also executes the Go test packages under all four feature-tag
combinations. Minimum Go 1.23, race/vet, preserved Rust tests, optional Node tests,
and benchmark validation are recorded separately by
`original-tests/{README.md,audit.json,validation.json}`. The latter records that
386 artifacts cross-compile but this kernel cannot execute ELF32; no native
32-bit run is claimed. Native-word hash defaults have architecture-aware tests.

Additional direct commands:

```sh
go test -race -tags semver_serde ./port/dependencies/semver/v1_0_28/...
go vet -tags semver_serde ./port/dependencies/semver/v1_0_28/... ./tools/port-semver/...
python3 tools/port-semver/original-tests/audit.py --check
python3 tools/port-semver/original-tests/run.py --offline --node
```
