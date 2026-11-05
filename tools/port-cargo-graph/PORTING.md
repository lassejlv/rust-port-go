# Cargo graph source port

The primary interface is `OwnedGraph[N,E]`, implemented in
`port/cargo/src/util/graph_owned.go`, `graph_owned_traversal.go` and
`graph_btree.go`. It translates all 303 lines of Cargo's `src/util/graph.rs`
under the explicitly selected dependency/compiler profile below. The older
`Graph[N,E]` in `graph.go` remains a value/trivial-Drop convenience. Its sorted
slices, eager cloning and historical fixtures do not establish the primary
interface's ownership or persistent-container contracts.

| Source identity | Value |
| --- | --- |
| Cargo revision | `24de4721844c7a4d839072bab6fb66d41c9bc1f4` |
| Path | `src/util/graph.rs` |
| Git blob | `74cdc51a0c36f21ce64f291540be571c05b12d63` |
| SHA-256 | `238123cc4c8300921008db5bcec1bbdf0946a023e0acafe9997e6917b5fada51` |
| Original size | 8,926 bytes; 303 lines |
| License | MIT OR Apache-2.0; notices retained in `port/cargo` |

`graph_source_manifest.json` contains the source/API/test mapping and evidence
bindings. The parent inventory alone controls source-file completion credit.
No Rust process services production operations.

## Primary source interfaces

| Original operation | Primary Go operation |
| --- | --- |
| `Graph`, `new`, `Default` | `OwnedGraph`, `NewOwnedGraph[WithConfig]` |
| `add`, `link` | consumed `*rustvec.Owned[N]`; `GraphEdgeMut[E]` borrows the actual E place |
| `reversed` | `Reversed[WithContext]` |
| `contains<Q>` | `Contains[WithContext]`, `OwnedGraphContainsBorrowed` with actual `Borrow<Q>` |
| `edge`, `edges` | shared `*E`, lazy `GraphEdgesIter` borrowing actual N/E cells |
| `iter`, `len` | `GraphNodeIter`, `Len`; source size hints and iteration comparisons |
| `sort`, `sort_inner_visit` | `Sort[WithContext]`, affine `GraphOwnedNodes[N]` |
| `is_path_from_to` | `IsPathFromTo[WithContext]` |
| `path_to_bottom`, `path_to_top`, private `path_to` | borrowed paths retaining the caller query and graph cells |
| `fmt::Debug` | `WriteDebugFormatter` with fallible writes and borrowed Display callbacks |
| debug `print_for_test` | private source-compatible stderr printer |
| `PartialEq`, `Eq`, `Clone`, default `clone_from` | `Equal`, `Clone`, `CloneFrom`, contextual variants |
| implicit field destruction | `Close[WithContext]` |

`GraphTraits` is static type metadata. Clone/Default return actual owned
witnesses; Drop and all comparison/format operations borrow the real cell.
Ord's `cmp`, PartialOrd's `lt`, Eq and overridden Ne remain independent calls.
The type metadata must stay fixed, and owned arguments/results must describe
the same Rust type's destructor. There is no hidden consumed closure environment
in these dictionaries. `OwnNode`/`OwnEdge` attach the configured Drop witness.

`GraphEdgeMut.AssignWithContext` evaluates an already prepared RHS into the
same place, including installing it when the old value's destructor unwinds.
`GraphOwnedNodes.CloseWithContext` destroys returned elements in order; `Ref`
and `Take` provide a borrowed element and an explicit ownership transfer. Its
host misuse/bounds checks are Go adapter guards, with no whole-Vec API credit.

Returned pointers, paths, iterators, formatter inputs and IO byte views are
borrows. Retain their source owner; obey Rust's shared/exclusive lifetimes,
affine moves and ordering laws. Queries stay caller-owned. Interior changes
that leave stored ordering identities intact are supported and observed by the
native probes. Copy graphs through `Clone`, never by Go struct assignment.
The Rc profile is not Send/Sync; serialize access as the Rust type requires.

## Persistent dependency and traversal containers

The production support under `port/dependencies/im-rc/v15_1_0` implements only
the operations Graph uses. It retains real Rc sharing, copy-on-write node
cloning, node capacity 64/median 32, tuple construction/destruction, initialized
Clone-prefix cleanup, iterator state and lazy Diff equality. Shared subtrees
can skip element callbacks. Occupied entries still obtain mutable storage
before destroying the owned lookup key. Insertion can modify a descendant
before a later argument/temporary Drop panics; no rollback is invented.

`source-manifest.json` inventories every file of the exact 48-file MPL-2.0
archive, with hashes, scope and preserved licenses. All seven locked registry
archives and every expanded source file actually compiled are verified against
the original Cargo lockfile. Selected profile: `im-rc 15.1.0`, Rc, no pool,
stable `build.rs` output without `has_specialisation`. Nightly specialization
(including Copy-key search), the threadsafe `im` variant, other collection APIs
and unused optional features are unsupported dependency profiles. No complete
im-rc/sized-chunks source file, archive, crate or test-suite credit is claimed.

Traversal marks/backlinks/seen sets use the required Rust 1.99.1 std B-tree
operations: capacity 11, linear query-to-stored comparisons, insertion pivot
selection, removal/rebalancing and ordered destruction. The private debug
printer preserves stable PartialOrd sorting, overridden Ne deduplication and
retention of the last equal reference. Its selected shared `rustsort.Stable`
helper comes from the separately validated pinned 1.101 implementation; this
Graph corpus also compares its observed reference-key operations to real
1.99.1 std sorting. `contracts/std-sort-reference.json` proves all seven
sorting algorithm files agree except internal abort-intrinsic spelling, and
the three alloc sort/buffer entry points are byte-identical.
`contracts/std-reference.json` records the actual 1.99.1 B-tree source identities; these scoped operations give no whole standard-library
source-file credit or physical allocator/ABI/stack-layout guarantee.

## Failure and formatter contracts

Each `WithContext` operation receives explicit ambient panic state. Translated
panic sites use `Context.PanicAt`/`PanicFormattedAt`; caught payloads propagate
through `ResumeUnwind` without another hook. A proven second panic escaping a
local cleanup invokes `AbortCleanup`. Ambient outer unwinding alone does not
skip locally required cleanup. Processing-panic source raises abort at the
raise site. Raw Go callback-local defers keep Go timing before an escaping panic
reaches a component boundary.

Hook payload kind, flags and location come from the selected native profile.
Callers supply actual borrowed source cells through `PanicOperations.LocationCells`
or scalar metadata through the `Locations` convenience;
unknown locations remain nil. The production code does not invent a Rust
build filename from a Go stack frame. The retained profile records remapped
`src/util/graph.rs`, registry crate-relative paths, actual `oracle.rs` compiler
argument spelling and the compiler's standard-library paths.

Debug formatting streams the original fragments, stops at the first writer or
Display error, and supplies nested node Display with default Formatter options.
The caller's outer flags remain unchanged. Character dispatch and borrowed
string backing are preserved. The shared `ruststdio` runtime holds the complete
statement's reentrant lease, preserves capture ownership/capacity, retries
interruption and handles actual std stderr EBADF behavior. Its guard ends before
the component formats an IO-error panic. `StderrBinding` supplies the current
source thread's complete operation for captured or already-held scopes; the
common host convenience uses an uncaptured fresh scope. The old `Stderr`
raw-writer option is an embedding/test convenience. Unix error text comes from
actual libc through `rusterrno`, with an explicit binding for no-cgo profiles;
`StderrErrorDisplay` supplies another selected backend's complete display.
These scoped adapters grant no whole std/Arc/TLS source-file credit.

Source details retained include RHS E cloning before Reversed's LHS key clones,
missing-child Sort panics, reachability's early missing-node termination,
cycle backlink removal, both eagerly evaluated debug `Option::or` lookups,
terminal-path stderr diagnostics and prepared-return-place leaks when a normal
local destructor panics. Debug assertions and overflow checks are independent
configuration choices; both default to true.

## Original tests and reproduction

The original `path_to_case`, `path_to_self` and `reverse` remain mapped in
`graph_test.go` in declaration order. The native helper executes the unchanged
three tests in each profile. `TestGraphOwnedOriginalSourceCases` additionally
runs those cases through the primary API, without extra upstream-test credit.
All 12 previous implementation/harness/fixture hashes are checked unchanged.

Normal Go tests replay the retained corpus without Rust or network access:

```sh
GOCACHE=/tmp/ferrugo-go123-cache go test ./port/cargo/src/util -run '^TestGraph' -count=1
GOCACHE=/tmp/ferrugo-go123-cache go test -race ./port/cargo/src/util -run '^TestGraphOwned' -count=1 -timeout=20m
GOCACHE=/tmp/ferrugo-go123-cache go vet ./port/cargo/src/util ./port/dependencies/im-rc/v15_1_0/...
```

The ownership helper requires the exact Rust 1.99.1 comparison compiler
`58cf8f9c35b355199727627d07ae34a4e2846d67`, 64-bit host, Python 3.11+ and the
verified offline source/registry caches. It never silently substitutes a new
compiler. Repository-local Cargo/rustc are defaults; `-cargo`/`-rustc` or
`FERRUGO_CARGO`/`FERRUGO_RUSTC` accept PATH names, relative or absolute paths.
`CARGO_HOME`/`-cargo-home` select the registry cache. Explicit bad selections fail.

```sh
python3 tools/port-cargo-graph/contracts/generate_owned.py --check
python3 tools/port-cargo-graph/contracts/audit.py --check
python3 tools/port-cargo-graph/contracts/generate_equality.py --check
# Optional offline source re-audit when the reference/SDK source caches are present:
python3 tools/port-cargo-graph/contracts/audit_sort_sources.py --check
FERRUGO_CARGO_GRAPH_CONTRACTS=1 go test ./port/cargo/src/util -run '^TestGraphOwned(LiveRegeneration|EqualityLiveRegeneration)$' -count=1
```

Omit `--check` to regenerate owned evidence/audits. Original source bytes are
read offline from the verified Git object; no source body is patched. The
wrapper alone enables panic-inspection development features with
`RUSTC_BOOTSTRAP=1`; stable dependency specialization remains disabled. Valid
existing gzip bytes are retained when the raw observations are unchanged, so
Go/Python compressor differences do not rewrite fixtures. See
`contracts/VALIDATION.md` and `owned-validation.json` for counts, bounds,
compiler flags, dependency pins and source/tool/output hashes. The supplemental `equality-validation.json` independently binds 1,360 actual-source
observations for differently shaped equal trees, equal-length distinct keys,
shared-prefix COW, equal replacement edges and Eq-default fallback. Its native
client reuses the original external non-Copy type declarations and includes
the complete Graph source unchanged. It adds no upstream test declarations.
The historical
value tool remains `go run ./tools/port-cargo-graph -check`.
