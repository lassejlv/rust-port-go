# Original union-find Debug comparison

Run from the repository root:

```sh
go run ./tools/port-rust-union-find/format -check
go test ./port/rust/compiler/rustc_data_structures/src/union_find
go test -tags=ferrugo_rustc_nightly ./port/rust/compiler/rustc_data_structures/src/union_find
```

Without `-check`, the command regenerates the retained original Rust output.
`-rustc` or `FERRUGO_RUSTC` selects a compiler, including a bare name resolved
through `PATH`; the default is `.toolchain/rust/bin/rustc`. `-root` selects the
repository. Go 1.23 is the minimum generator/test version. The live test also
accepts `FERRUGO_GO` and runs when `FERRUGO_UF_FORMAT_ORACLE=1`.

The generator verifies the upstream checkout pin and five exact Git blobs at
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`, then copies entire original
`idx.rs`, `slice.rs`, `vec.rs`, `union_find.rs`, and `union_find/tests.rs`
without modifying their bytes. A tiny module-binding file compiles the actual
index library. The normal feature profile needs no external Rust dependencies,
proc-macro substitutes, codec shims, or Cargo packages. Both original UF tests
run against this actual library in each of four independently compiled
overflow/debug-assertion profiles.

The pinned source crate declares Rust 1.101.0 development; the recorded
available compiler is Rust 1.99.1. The wrapper uses
`RUSTC_BOOTSTRAP=1` only to access the existing public `formatting_options`
feature for direct caller formatter creation and reuse. Original source
bodies and their assertions remain unchanged. No general Rust standard
library port or Rust ABI is claimed.

The 25,040 reference records exercise the native derived UF/entry Debug
implementations with fifteen styles, empty/singleton/unified forests,
borrowed non-comparable metadata keys, independently overridden string and
character writes, every relevant prefix/body writer failure, key/local
destructor/writer panics, callbacks catching sink errors, NUL/Unicode fills,
multiline boundaries, formatter options after failure and reuse, and state
after recovery. Retained receiver addresses across formatter calls verify
same-container parent-cell identity without comparing numeric addresses
between Rust and Go. The original algorithm corpus remains separate and
unchanged.

`manifest.json` binds source hashes, compiler/profile evidence, wrapper and
request/replay inputs, and decoded/compressed corpus hashes. Ordinary tests
replay the retained Rust output offline. Missing native output is an error;
Go output is never used to construct expected responses. `-check` rebuilds
all four references and compares both fresh source records and Go replay.
It retains existing compressed bytes and historical Go version metadata so
different Go gzip implementations can verify the same corpus.

The primary Go API is
`UnionFind.FormatDebug(*rustcindex.IndexDebugFormatter, borrowedKeyDebug)`.
It passes actual `entry.parent` cells through the source DebugStruct and
PadAdapter behavior and the already verified IndexSlice list formatter.
`Debug(func(Key) string)` remains the compact string convenience. Formatter
options, writer errors, and source panic context use the index-storage bridge;
translated Rust panic sites use that bridge's `Context.Panic`. Plain Go
panic/defer callbacks retain Go cleanup timing. This fix does not replace the
UF table/snapshot adapters or earn additional source-file completion credit.

The primary snapshot API is `SnapshotVec(ValueOps[Key])`, returning the actual
validated `*rustcindex.IndexVec[Key,Key]`; `SnapshotVecWithConfig` independently
selects returned-vector overflow/debug options. This uses a static typed-index
iterator and trusted collection in source order, including the initial
`Idx::new(len)` check. The existing `Snapshot()` slice-shaped result is only
a compatibility projection of that primary path. Therefore the unchanged
3,788 algorithm records also exercise the primary snapshot path. Focused
tests exercise returned-vector Clone/Eq/Hash/Debug/index/iterator dispatch,
non-comparable key metadata, ownership independence, and zero-sized key/empty
vector capacity. No general table-storage replacement is performed.
