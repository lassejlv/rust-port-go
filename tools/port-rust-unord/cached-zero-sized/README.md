# Logical zero-sized cached-key sorting

This is an additive supporting adapter for source slices whose element type is
actually zero-sized. It provides no Rust std-file completion credit. Existing
sorting engines, dictionaries and collector bodies are unchanged.

The retained driver calls the actual unchanged `sort_by_cached_key` in the exact
1.101 nightly SDK (`708c94ef093ecc42fb8851e17d8383d999727abc`). Its input is a
sound `Vec<InputZero>` of an inhabited empty struct without Drop. Rust permits
its full `usize::MAX` logical length without allocating element storage.
The driver, source library hashes, complete compiler version, and every output
are retained. `native_manifest.json` contains the independently generated
Rust-only evidence; it does not depend on the Go implementation.

There are 624 native processes and 208 real SIGABRT prefixes. They cover normal,
inherited unwind, and processing-panic contexts, fresh panic versus resume,
three key layouts, independent tuple partial-comparison/default/override paths,
key generation/comparison/destructor failures, actual FnMut environment and
field destruction, moved captures, and full-uint preflight failure. Exact native
hooks, locations and partial stdout are compared, with no invented post-abort
state. The surrounding owned collection and its BorrowMut/return-place behavior
are separate Unord integration work and remain pending.

A development-only global allocator observes the first native key-vector
allocation before any key callback. For four ZST inputs, a one-byte key uses a
32-byte allocation aligned to four (u32 indices); a 16-byte/alignment-eight key
uses 96 bytes aligned to eight (usize indices). The Go helper's selected source
layout is compared with this actual allocation. ALLOC lines are native-only
evidence; Go allocator callback/ABI equivalence is not claimed.

The Go helper holds a logical uint length and the actual borrowed ZST value,
without creating an impossible physical Go slice. It composes the committed
trusted collector, tuple comparator, unstable engine, and cleanup guards. The
nonzero (key,index) vector still allocates. Source capacity failure occurs inside
the real collector before the first key, with the current panic metadata.
Key/Function dictionaries include complete field destruction and temporary
borrow laws; no call-local runtime scope is retained after this operation.

Run `python3 tools/port-rust-unord/cached-zero-sized/generate.py --check` for live
Rust regeneration and Go replay. Explicit `FERRUGO_GO` and
`FERRUGO_RUSTC_NIGHTLY` selections must resolve; invalid selections fail.
Existing valid gzip bytes are preserved when raw records are identical.
