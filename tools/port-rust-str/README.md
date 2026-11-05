# Scoped Rust string slicing runtime

`port/support/ruststr` translates the used contracts of SDK 1.101 `str` range
indexing, `get`, inclusive indexing, UTF-8 boundary queries, and all six
`slice_error_fail_rt` branches. It is supporting compatibility code and earns
no standard-library source-file completion credit.

The source is the official dated `2026-11-04` rust-src component, commit
`708c94ef093ecc42fb8851e17d8383d999727abc`. `SOURCES.json` binds the official
channel manifest, source archive, four selected complete modules, and licenses.
The selected source is distinct from the compiler repository's source pin.

Run from the repository root:

```
python3 tools/port-rust-str/prepare.py --fetch
FERRUGO_RUSTC=/absolute/path/to/the/pinned/rustc python3 tools/port-rust-str/generate.py --check
FERRUGO_RUSTC=/absolute/path/to/the/pinned/rustc python3 tools/port-rust-str/generate_ambient.py --check
python3 tools/port-rust-str/test_tools.py
go test ./port/support/ruststr
```

`--fetch` is explicit; ordinary verification and generation never fetch missing
sources. An explicitly empty, missing or different compiler is an error.
Bare compiler names use the caller's `PATH`. Archive and copied-source bytes
are reverified on every generation. Gzip reuse requires bounded decompression,
valid CRC and exact equality with newly observed native bytes. Corrupt,
different, or symlink fixtures fail `--check`.

The selected cache's parent is canonicalized so an isolated checkout may
intentionally alias its trusted `.toolchain` base. The cache leaf itself,
ownership marker and owned source descendants still reject symlinks. The
integrity suite copies the helper/evidence into a fresh checkout with a
symlinked toolchain and reruns both live native checks there.

`Slice(text, begin, end, context, site)` is the primary range adapter. It returns
a borrowed substring and raises through `Context.PanicFormattedAt` on failure.
The context's immutable location dictionary supplies the caller's exact
tracked location for `site`; absent metadata remains unlocated. The corpus
explicitly installs the oracle's named, repository-relative caller profile.
Production never infers a Rust filename from the Go host stack.

Raw string arguments have Rust `&str`'s valid-UTF-8 precondition. `Checked`
provides the separate O(n) validation boundary for unchecked Go input.
Successful slicing, `Get`, and boundary queries take O(1); boundary searches
examine at most the current four-byte scalar. Failure formatting may allocate.
Nonempty successful substring tests retain original backing-byte identity;
no empty-string ABI, allocation-failure or raw-pointer representation claim is
made. Source characters use the independently exhaustive Unicode 18 Debug
adapter, rather than the host Go Unicode version.

`InclusiveRange` is a scoped consumed-range state witness, not a general range
iterator port. `ExhaustedInclusive` matches the actual singleton `next()` state:
start advances, and the internal exhausted flag is set on endpoint overflow.
This state affects diagnostics as well as empty-slice results.

The original SDK oracle runs standard-library operations directly without
replacing any Rust body. Its 197,024 cases include 85,911 exact payload/Any-kind,
caller-location, hook-state and flag observations. A separate 27-case native
probe runs actual inherited Drop and panic-hook paths, including eight real
abort processes. Abort records retain only observable events, never invented
post-abort state. The Go adapter routes source failures at the actual slice
failure site, before caller cleanup. Arbitrary raw Go callback defers are not
claimed to reproduce Rust panic-runtime timing.
