# Pinned hygiene implementation evidence

The implementation remains scoped partial: owned String results and returned
collection storage still need the [public API closure](../../docs/SPAN-API-GAPS.md).
The retained comparisons do not grant full hygiene.rs completion credit.

The Go translation is split across `port/rust/compiler/rustc_span/src/hygiene*.go`.
The reference is the unchanged, complete 1,766-line `compiler/rustc_span/src/hygiene.rs`
at `2d20b42b883dcfc014f3c2c56081eeb6091c2179`, compiled within the actual complete
span dependency graph retained by `tools/port-rust-span-reference`.
The selected compiler is official nightly-2026-11-04, commit `708c94ef0`.
Reference preparation checks the original source bytes and dependency archives;
none of these clients replaces a source implementation or dependency branch.

The baseline contains 85,103 observations across the core, graph, remapper,
encode queue, decode helpers, formatting, codec, tracing, Hash and malformed
codec suites, plus inherited PartialOrd/Ord operations. The owned argument/closure
matrix contains 14,400 isolated scripts under all four assertion/overflow profiles;
8,768 scripts terminate with actual
native SIGABRT. Native statuses other than success and SIGABRT are rejected.
The Go replay uses exit134 for that abort class, without claiming signal ABI
identity. It compares complete stdout, including final complete HygieneData
states on returning paths.

The owned matrix covers consumed StableHashCtxt arguments in fresh/set/mark,
hash-disambiguation collisions and invalid parent/reset/disambiguator inputs;
owned and borrowed dollar-crate resolvers; dropped comparator contexts; and both
remapper FnOnce environments. Each observed callback position is interrupted
independently, including a second failing Drop or field destructor. Actual Arc
strong counts distinguish moved, cloned, dropped and leaked data. Generic
remapper results have real owned Arc fields: their destruction reenters hygiene
after the lock is released, and their prepared return places remain leaked if
closure destruction fails under the held lock.

The same matrix covers registry reset/parent/hash assertions and fresh/set
duplicates against safely pre-registered foreign hashes; missing local,
foreign and syntax-context lookup paths; generated index bounds, arithmetic and
Step/Idx operations; decoder reads and root/cache fast paths; every malformed codec
case and raw fingerprint length; held/shared encode-cell borrows; verbose Debug
and formatted assertions invoking user DefId Debug; unset session storage; and
invalid Ord clamp bounds. It compares each caught payload's concrete kind,
can-unwind/no-backtrace flags, exact selected source location, hook order and
final full registry state. Exact cleanup behavior is observed using real native
Arc counts and process termination, including panic-processing invocation.
Formatted assertion payloads retain their partial text if a user Debug returns
`fmt::Error`; actual user panics during formatting retain the native abort edge.

The separate 800-case non-Copy commit matrix covers the weaker `T: From<u32>`
bound on `HygieneRemapper::commit_end_of_determinism`. The complete primary
receives a From/Drop value dictionary; borrowed remapper dictionaries pass the
current Context through all four required operations and the previous value's
destruction. Real owned Arc/Vec fields distinguish installation of the new slot,
old Option cleanup, constructor failure and old/new Drop/field abort boundaries.
Its actual imported generic MIR retains both the slot-before-assertion order
and the cleanup `drop(prev)` edge with `unwind terminate(cleanup)`.

Construction uses runtime1 and a stale hook. Invocation supplies runtime2 and
the current hook explicitly. Normal, inherited-caught, inherited-escaping,
panic-processing and silent/runtime0 invocation contexts compare hooks, local
destruction, field cleanup and compiler cleanup-abort boundaries. Runtime0 uses
an explicit empty Context, preventing construction metadata from becoming the
current caller. The Go runtime does not intercept global panics or infer Rust
locations from its host stack. Actual selected source locations are profile
inputs; the source Lock's tracked caller is hygiene.rs:555:84 for the remapper
reentry witness.

Remapping uses the translated Rust `sort_by_cached_key` implementation, with
the actual u64 key layout and a complete owned key-function dictionary whose
captures are borrowed remapper/hcx references. Compact debug output uses the
translated stable sort on the actual pair-of-references element layout.
Both proof families bind the sorting support source. Sorting support has its
own independent original-SDK comparison; this grants no standard-library file
completion credit.

Tracing evidence now retains native subscriber metadata verbatim, including
the selected build's `original/compiler/...` file and event-name spellings.
The tracing manifest records the exact sixteen-callsite metadata profile.
`WithHygieneTracingProfile` attaches that immutable table independently of the
current Scope/runtime payload and panic operations. The named
`LogicalHygieneTracingProfile` is a Go convenience; operations without an attached
selected profile retain that logical view and make no original-build metadata
claim. No observer silently strips filenames before comparison.

```sh
python3 tools/port-rust-hygiene/generate.py --prepared
python3 tools/port-rust-hygiene/owned_generate.py --prepared
python3 tools/port-rust-hygiene/commit_generate.py --prepared
python3 tools/port-rust-hygiene/generate.py --prepared --check
python3 tools/port-rust-hygiene/owned_generate.py --prepared --check
python3 tools/port-rust-hygiene/commit_generate.py --prepared --check
python3 tools/port-rust-hygiene/source_inventory.py --check
GOMAXPROCS=2 GOCACHE=/tmp/ferrugo-go123-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -p 1 ./port/rust/compiler/rustc_span/src -run '^(TestOriginalHygiene|TestHygieneTracingEvidence|TestHygieneTracingProfile)' -count=1
```

Every invocation uses the existing complete reference verifier for every
required profile, including all four owned/commit profiles and the hash
suite's independent assertion profiles. Missing cold profiles are prepared
before linkage is consumed. `--prepared` is a compatibility alias; it never
skips source/compiler/archive verification, upstream tests or the external
linkage smoke. Explicit cached profile assertions never select executable
commands or library artifacts.

Owned panic filenames include the producer's genuine absolute scoped-tls
registry source path. The checked-in corpus is historical producer evidence;
its strict `--check` is environment-specific. No observer normalizes those
filenames. Equal raw observations preserve exact historical gzip bytes, and
default owned generation refuses to replace different historical observations.
CI and other producers select an ignored fresh artifact directory instead:

```sh
python3 tools/port-rust-hygiene/owned_generate.py --output-dir "$RUNNER_TEMP/hygiene-owned"
FERRUGO_HYGIENE_OWNED_PROFILE_DIR="$RUNNER_TEMP/hygiene-owned" GOMAXPROCS=2 .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -p 1 ./port/rust/compiler/rustc_span/src -run '^TestOriginalHygieneOwned$' -count=1
```

Upload `owned-manifest.json`, `hygiene_owned.jsonl.gz` and `hygiene_owned.jsonl`
from that directory verbatim. Absent selection replays the historical paths;
explicitly empty, missing, partial, oversized or invalid selected profiles
fail without fallback. The parent verifies source and compressed/raw hashes
before spawning requests. Each child receives the captured absolute directory
and immutable manifest digest, validates that same bounded profile and never
borrows a construction-time producer selection. This avoids decompressing the
whole corpus in every abort child.

Generation remains offline with the pinned reference inputs present. Cache ownership markers,
bounded child timeouts and disabled native core dumps apply. Existing gzip
bytes are preserved when uncompressed observations are equal. Go tests replay
the retained observations without Rust or network access.

`source-inventory.json` lists all 113 explicit source functions, every source
region, generated/derived trait surface and the corresponding Go files. Its
sixteen disjoint regions cover all 1,766 lines, and its source and newtype macro
hashes are checked against the independent complete-source graph. This review
index does not replace behavioral evidence. The Go representation requires
source-valid enum variants/active fields and explicit ownership operations.
Unchecked invalid newtype values are outside the original Rust contract.

Full-file acceptance remains an independent root review decision. These reports
do not grant completion credit to supporting collections, sorting, formatting,
hashing, runtime or surrounding Span/SourceMap implementations. Normal Span
wrapper evidence is maintained separately by `tools/port-rust-span-lib`.
