# Cargo Graph owned-contract validation

Reference source and every source/tool/result binding are recorded in
`owned-validation.json`. `audit.py --check` independently checks the complete
Graph mapping, three original test declarations, all 48 im-rc archive-file
identities, required licenses and the deliberately partial dependency inventory.

## Selected original Rust profile

- Cargo source revision `24de4721844c7a4d839072bab6fb66d41c9bc1f4`, complete
  unchanged `src/util/graph.rs`, blob `74cdc51a0c36f21ce64f291540be571c05b12d63`.
- Official Rust 1.99.1, commit `58cf8f9c35b355199727627d07ae34a4e2846d67`,
  x86_64 Linux, 64-bit usize. Cargo and original registry sources run offline.
- Four independently built configurations: debug assertions false/true crossed
  with overflow checks false/true. Each uses optimization level zero and
  unwinding panics; each executes all three unchanged original unit tests.
- Exact Cargo.lock registry versions: im-rc 15.1.0, bitmaps 2.1.0, rand_core
  0.6.4, rand_xoshiro 0.6.0, sized-chunks 0.6.5, typenum 1.20.1,
  version_check 0.9.5. Archives and every expanded compiled file are checked,
  including Cargo-normalized manifests; mutable/symlink extras are rejected.
- im-rc build output is explicitly checked to exclude `has_specialisation`.
  `RUSTC_BOOTSTRAP=1` enables only the external observation wrapper's
  `panic_can_unwind`/`panic_internals` development features. No original source
  body or feature gate is edited, and stable dependency specialization stays
  disabled.

## Retained native observations

| Configuration | Native observations | Native SIGABRT cases | Original tests |
| --- | ---: | ---: | ---: |
| d0o0 | 16,098 | 2,437 | 3 |
| d0o1 | 16,098 | 2,437 | 3 |
| d1o0 | 17,053 | 2,525 | 3 |
| d1o1 | 17,053 | 2,525 | 3 |
| Total | 66,302 | 9,924 | 12 executions / 3 declarations |

Profile-identical observations share retained rows: 18,560 JSONL rows,
122,279,876 uncompressed bytes and 7,171,080 compressed bytes.

- Raw SHA-256:
  `69d502458a583813099af633dafa31672b6c269e55c6edb5a45436846595d1d3`.
- Compressed SHA-256:
  `5839584d9b517ad609f28db7c418e231ca182d978a61d964cab52aa82a6c3ea1`.

Each profile has 2,986 baseline requests. These cover ordinary non-Copy keys and
edges, empty/isolated/chain/star/missing-child/cyclic graphs, ascending and
descending construction, separate PartialOrd/Eq/Ne overrides, sizes
0/1/3/12/65/130, and all 512 directed three-node edge sets for both path
orientations and reachability. The matrix additionally includes:

- Every actual callback failure position in the complete small-operation
  matrix; selected first/middle/last positions of each callback kind for larger
  COW/insertion and inherited/processing contexts.
- Clone, Default, Ord, PartialOrd, Eq, overridden Ne, Borrow, Display, key/edge
  Drop and nested field Drop, with guards and mutations of the actual borrowed
  cell. Weak-reference observations distinguish destroyed values from leaked
  return places.
- Lazy Rc sharing, shared-subtree equality, copy-on-write clone/drop order,
  mutable edge-place assignment, clone_from, reverse RHS-before-LHS evaluation,
  iterator hints/next/nth/count and continuation after a partial insertion.
- Owned sort marks and results, std B-tree comparisons/rebalancing, borrowed
  query/graph path identities, path-edge mutations and corresponding lookup
  aliases. Queries remain independently caller-owned.
- Eight formatter/error patterns, every selected writer failure position,
  optional independent character dispatch, actual string backing aliases,
  outer formatter flags, default nested Display flags and repeated use.
- Normal catch, caught callback panic during an inherited unwind, escaping
  panic during that unwind, and execution from the real panic hook.
- Real hook payload kind/text, can_unwind/force_no_backtrace flags, selected
  source locations, forwarding without duplicate hooks, cleanup-abort hook
  ordering and hook panic/resume/destructor aborts. Isolated Go subprocesses
  must match status 134, the full component event trace and Graph-produced
  stderr prefix. Native runtime abort messages/backtraces are outside this
  component comparison; hook diagnostics themselves remain in the event trace.
- Actual closed stderr (EBADF success), broken pipe (owned-string panic), and
  the checked iterator-counter underflow exposed after an insertion destructor
  fails. Unchecked profiles retain the corresponding wrapping state.

Finite fixtures do not claim exhaustive arbitrary graph sizes or callback
programs. The implementation follows the source algorithms and static trait,
borrow, affine-ownership and ordering preconditions described in `PORTING.md`.
Nightly specialization, pooling, the threadsafe im variant, unused dependency
APIs/features and complete std/im-rc source-file credit are excluded.

The default Unix error formatter calls the actual system libc through the
scoped rusterrno adapter. A separate actual stable1.99.1 comparison retains
4,451 errno/writer observations, including unknown and negative i32 errors,
and every selected writer failure. It complements the complete graph's real
EPIPE/EBADF observations. Go's errno table/capitalization is not used as Rust
error text. Current-context explicit bindings are tested without cgo; other
host I/O backends supply the complete StderrErrorDisplay witness. The native
host proof runs independently on Linux and macOS.

The actual complete graph has a separate72-case stdio composition proof:
empty/1/3/12-node graphs, capture on/off, pre-held leases, repeated prints and
initial capture capacities0/16. Eight real EIO cases prove another thread can
acquire stderr inside the IO panic hook; subsequent prints continue after the
first failure. Capture bytes/capacity/Arc counts and every raw write match.
The shared stdio dependency separately retains178 both-SDK cases and20 real
aborts, including capture replacement, poison, formatter errors and concurrent
complete statements. Full std/Arc/TLS implementation credit stays excluded.

## Gates

Go 1.23.12 on x86_64 Linux:

- `go test ./port/cargo/src/util -run '^TestGraph' -count=1`: PASS, 84.790s.
  This includes all retained owned observations and unchanged historical cases.
- `FERRUGO_CARGO_GRAPH_CONTRACTS=1 go test ./port/cargo/src/util -run
  '^TestGraphOwnedLiveRegeneration$' -count=1 -v`: PASS, 206.065s. Rebuilds all
  four original-source profiles, reruns the native corpus and original tests,
  checks all bindings and reruns the source/archive audit without rewriting.
- `go vet ./port/cargo/src/util ./port/dependencies/im-rc/v15_1_0/...`: PASS.
- `GOARCH=386 go test -c ./port/cargo/src/util`: PASS, compile only. There is
  no 32-bit native/runtime validation claim.
- Source/archive `audit.py --check`: PASS. All 12 frozen historical evidence
  hashes remain byte-identical; generation checks them before compiling.

- Historical `go run ./tools/port-cargo-graph -check` with explicit Go 1.23.12
  and Rust 1.99.1 executables: PASS. The unchanged 1,416-case/profile value
  corpus, vector/record keys and eight Eq/Ne probes/profile still match.

The initial owned race command reached Go's default ten-minute package timeout
while actively replaying setup in d1o1, with no mismatch or data-race report.
Native and historical checks were running concurrently for part of that attempt.
The unchanged complete retained race corpus then PASSED alone with
`-timeout=20m`, 590.049s. No cases or assertions were removed. The supplemental
equality file was introduced separately and also passed its complete targeted
race replay, 102.129s; the two race commands cover all retained observations.


## Supplemental lazy equality shapes

`equality-validation.json` binds an independent original-source client and
`graph_owned_equality_test.go`. The native client appends its entry point to
the existing external non-Copy type/probe declarations, before the old client's
operation/main functions. The complete actual Graph is still included unchanged;
its three original unit tests run again in all four configurations, without
new declaration credit. All registry/source/profile checks are retained.

There are 1,360 additional native observations, grouped into 340 rows, with
9,274,975 raw bytes and 691,632 compressed bytes. All four profiles produce the
same component observations for these inputs.

- Raw SHA-256:
  `db80efd9bb5b25ebeb60aea01cf006b8db6ba61464a597bdb9d0541b4c03c64d`.
- Compressed SHA-256:
  `2e1b7ca397e245a5f82972c5979aaaa8c5f66d21da445f603a92464982c90b82`.

The additional cases cover shifted key sets of equal length in both comparison
directions; equal sets inserted ascending, descending and rotated; independent
versus shared trees; COW changes at a last/middle outer node; and a last inner
edge replaced with either an unequal or equal value. Explicit Ne and inherited
Eq-default fallback are both observed. Selected first/middle/last callback
failures retain partial state and full cleanup traces. Sizes are
0/1/3/65/130, with larger star cases exercising nested maps.

Actual successful equal-tree traces contain negative, zero and positive cmp
results. For the 130-node witnesses, independent differently shaped equality
performs 130 node-Ne calls, whereas a last-node COW difference performs 66;
these are native observations of subtree skipping. The Eq-default star witness
records 99 key-Eq and 65 edge-Eq calls, with no key/edge-Ne override calls.
These counts describe the chosen fixtures, not hardcoded production results.

Go 1.23.12 supplemental gates: normal replay PASS 10.575s; race replay PASS
102.129s; opt-in fresh original-source `--check` PASS 12.115s; final vet PASS.
The original 66,302-observation fixture and every historical artifact remain
byte-identical. Total retained component coverage is 67,662 native observations,
including the same 9,924 isolated abort cases; this adds no project/file/test
completion credit for dependencies.

## Stable-sort dependency source identity

`audit_sort_sources.py` compares the exact reference compiler's seven sorting
algorithm files against the checksum-verified official 1.101 SDK sources already
used by the shared helper. `std-sort-reference.json` records original URLs,
Git blob identities, complete hashes, the SDK archive checksum and audit tool
hash. Four algorithm files are byte-identical; the remaining three differ only
in internal invariant abort-intrinsic spelling. A text-only canonical digest
confirms there are no other changes; canonicalized bytes are never compiled or
written over originals. The alloc slice `sort`, `stable_sort` and Vec `BufGuard`
implementation items are individually byte-identical. Other alloc-slice changes
are outside this support scope. This supplies source identity in addition to
the actual 1.99 native comparison, with no whole std-file credit.

The offline source re-audit PASSED. `--fetch` explicitly permits filling missing
source caches from the recorded official SDK/archive and immutable Git URLs;
ordinary Go/native replay does not require these additional source caches.

## Resource and reproducibility bounds

Each native case has a five-second deadline, output acceptance limits of
8 MiB stdout/1 MiB stderr and core dumps disabled. Only SIGABRT is accepted as
the reference abort class. The primary retained reader checks compressed/raw SHA-256,
64 MiB compressed and 192 MiB raw bounds, JSONL row/profile/observation counts,
and an 8 MiB maximum record. Each Go abort subprocess has a ten-second deadline;
the opt-in native generator and audit each have a five-minute deadline. The
supplement uses a 32 MiB compressed/96 MiB raw reader bound and its own
five-minute native regeneration deadline.

The generator defaults to two build/observation workers (explicitly limited to
1..4). Explicit Cargo/rustc selections and CARGO_HOME are respected; missing or
wrong-profile tools fail. Network and Git lazy fetch remain disabled. Existing
valid gzip bytes are preserved when raw oracle bytes match, including when a
checking host's compression implementation differs. The older value-interface
fixture bytes and its tool remain unchanged and independently checkable.
