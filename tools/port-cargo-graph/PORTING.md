# Cargo graph source port

This port accounts for the entire 303-line `src/util/graph.rs` from
`rust-lang/cargo@24de4721844c7a4d839072bab6fb66d41c9bc1f4`.

- Git blob: `74cdc51a0c36f21ce64f291540be571c05b12d63`.
- Original bytes: 8,926.
- Source SHA-256: `238123cc4c8300921008db5bcec1bbdf0946a023e0acafe9997e6917b5fada51`.
- Destination: `port/cargo/src/util/graph.go`, with tests in `graph_test.go` and `graph_generic_test.go`.
- License: MIT OR Apache-2.0; the upstream notices are in `port/cargo`.

The production implementation uses only Go's standard library. The Rust oracle
is a development tool and never services production graph operations.

## Complete source mapping

| Original source | Go implementation |
| --- | --- |
| `Graph<N,E>`, `new`, `Default` | `Graph[N,E]`, `GraphOptions`, `NewGraph` |
| `add`, `link` | `Add`, `Link` (returns `*E`) |
| `reversed` | `Reversed` |
| `contains`, including `Borrow<Q>` | `Contains`, `ContainsBy` |
| `edge`, `edges` | `Edge`, comparator-sorted `Edges` iterator |
| `sort`, `sort_inner_visit` | `Sort`, `sortInnerVisit` |
| `iter`, `len` | comparator-sorted `Iter`, `Len` |
| `is_path_from_to` | `IsPathFromTo` |
| `path_to_bottom`, `path_to_top`, `path_to` | `PathToBottom`, `PathToTop`, `pathTo` |
| Debug-build path assertions | enabled by default; `DisableDebugAssertions` selects release behavior |
| `fmt::Debug` | `Debug`, `String`, `Format`, with a node Display adapter |
| `print_for_test` | `TestString`, `PrintForTest`; path failures print to stderr |
| `PartialEq`, `Eq`, `Clone` | `Equal`, `Clone` |
| `path_to_case`, `path_to_self`, `reverse` | three separate `TestGraphUpstream*` tests |

## Go adapters

`N` is unconstrained (`any`). The required `Compare` callback supplies Rust's
total ordering and stored-key identity. Storage, marks, seen sets, backlinks and
printer key sets all use comparator-searched sorted entries; no operation
requires native Go equality or hashing. Slice keys and records containing slices
are supported directly, including equal keys backed by different allocations.
`ContainsBy` takes a comparison against a borrowed query, preserving `Borrow<Q>`
lookup without requiring the query to have the node's Go type.

`EqualNode` and `NotEqualNode` independently adapt direct Rust `==` and `!=`
expressions. Their defaults are `Compare(a,b) == 0` and `!EqualNode(a,b)`;
adapters must obey Rust's Ord/Eq value laws. Reachability uses Eq, reconstruction
uses Ne, and the debug path-membership check uses Eq. Independent-map equality
matches the pinned `im_rc` DiffIter: compare key order, then compare the stored
tuple with Ne. Consequently it invokes node Ne and edge Ne, rather than
substituting Eq or ordering for those operators. Exact operator traces and
callback panic propagation are checked against the original Rust source.

`EdgeDefault`, `CloneNode`, `CloneEdge`, `EqualEdge`, `NotEqualEdge`, and
`DisplayNode` supply the corresponding Rust traits. Default and Clone fall back to Go's zero value and
value assignment; reference-bearing types need explicit adapters matching their
Rust semantics. Equality requires `EqualEdge` or `NotEqualEdge`; Ne falls back
to `!EqualEdge` when its override is absent. `NewGraph` also implements the
original Default constructor because Go needs the runtime ordering adapter.

The external ordered containers are replaced by sorted Go entry slices with
binary-search lookup. The collection representation and comparator-search
patterns differ from the Rust trees; ordering is still the sole stored-key
identity, and iteration is always ordered. `Clone` eagerly allocates independent
adjacency and edge storage and invokes the
Clone adapters. The Rust persistent map can share allocations and defer element
cloning until mutation; allocation sharing, performance, and callback invocation
timing are therefore storage adaptations. Rust's shared-subtree map equality
can also skip callbacks that an independent Go comparison evaluates; exact
operator-trace fixtures therefore compare independently constructed maps.
Graph values, mutation independence,
edge defaulting, traversal results, and failures are tested. Each edge has a
unique nonzero-sized storage cell so that within-graph pointer assertions also
work when `E` has zero size.

Go cannot enforce Rust's shared and mutable borrow lifetimes. Edge/path pointers
borrow the graph; callers must write edge values only through `Link`, avoid
mutation during iteration, avoid mutating stored key data through aliases, and
use `Clone` rather than copying a Graph struct.
These are API contracts, not runtime calls into Rust.

`PrintForTest` accepts an `io.Writer`, exposing write errors to Go callers. The
original path failure branch uses `os.Stderr`. Panic text containing Rust source
locations or formatting is represented by named Go panic constants; the oracle
compares the actual failure category and tests the printed graph bytes.

## Preserved source behavior

The implementation deliberately retains all of these source behaviors:

- `Link` adds only its source node, not a child-only node. Calling `Add` again
  preserves its existing edges, and linking an existing edge preserves its value.
- `Sort` is marked DFS postorder. It returns an order on cycles and panics when
  visiting a child without a node entry.
- Reachability requires a traversed edge, including for `from == to`. A missing
  node popped from the stack ends the entire `while let` search, even when other
  nodes remain pending.
- The path search uses the original queue and inserts backlinks for the starting
  node when a cycle reaches it. Reconstruction removes backlinks as it walks;
  the first returned entry may therefore contain an edge on a cycle.
- The debug edge check prefers the reverse-direction edge before the forward
  edge, exactly as the source's `or` expression does. Reciprocal edges can
  consequently trigger a pointer assertion.
- The debug terminal-path assertion and its stderr test-case printer remain in
  place. Release mode omits both assertions, matching `cfg(debug_assertions)`.

## Retained validation

`oracle.rs` includes the original pinned source without rewriting its methods.
Its standalone Cargo lockfile selects the exact `im-rc 15.1.0` and six transitive
dependency versions/checksums found in the pinned Cargo lockfile. The generator
verifies both the Cargo revision and original source blob/SHA-256, verifies all
seven dependency selections, and runs the three original Rust tests in both
debug and release profiles.

The corpus has 1,416 graphs per profile:

- Eight targeted graphs, including self-loops, missing children, duplicate links,
  weighted edges, and the upstream shortest-path example.
- Every directed three-node graph (512 edge sets), once with all nodes explicitly
  added and once with only `Link`-created sources: 1,024 cases.
- 384 larger deterministic graphs from xorshift seed `0x73f9ba21d8c645e1`, with
  disconnected nodes, missing children, cycles, edge overwrites and weights.

For each case, the harness compares length, sorted iteration and edges, all
query-pair edge lookups and reachability results, sorting or its panic, both
path directions or their panics, Debug bytes, stderr test-printer bytes,
reversal, equality, clone changes, and original-graph preservation. Both debug
and release results are retained in `testdata/graph_cases.json.gz`. Normal Go
tests regenerate the deterministic inputs, check their SHA-256, and replay the
stored oracle results without requiring Rust, Cargo, or network access.

The retained run matched 428 missing-node sort panics per profile. Debug mode
also matched 1,783 edge-pointer assertion failures and 1,688 terminal-path
assertion failures. No panic had an unclassified category. There are no skipped
source methods or upstream test cases. Root review remains required before an
inventory row is marked validated.

Additional original-source fixtures in `testdata/graph_generic_{debug,release}`
exercise actual `Graph<Vec<i32>,i32>` and records containing vectors, compared to
Go `[]int` and structs containing slices. They preserve borrowed slice lookups,
independent but equal key allocations, ordering, missing-child failures, paths,
clone/reverse behavior, Debug and normalized test printing. Eight operator probes
per profile preserve direct node Eq/Ne traces, `im_rc` map equality's node/edge Ne
traces, node Eq/Ne panic propagation, edge Eq bypass when Ne is overridden, and
edge Ne panic propagation. These fixtures are regenerated and replayed alongside
the unchanged 1,416-case integer corpus.

Additional Go tests exercise nil versus allocated empty slice keys, a record's
complete key identity, comparator lookup, retained keys and pointer stability as
entry slices move, custom ordering/display, default edges, zero-sized edge
identity, iterator early stops, reference-bearing Clone/Eq adapters, Sort's cloned
node outputs, and independent clone/reversal mutation.

## Reproduction

With the pinned Go/Rust tools and dependencies cached:

```sh
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-cargo-graph
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-cargo-graph -check
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test ./port/cargo/src/util -run '^TestGraph' -count=1
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go vet ./port/cargo/src/util ./tools/port-cargo-graph
```

The first command regenerates evidence only after the Go comparisons pass; the
second rebuilds/runs the original source and verifies all retained bytes and
report hashes. `-offline=false` permits an initial fetch into
`.toolchain/port-cargo-home`; committed lockfile checksums still apply.
`validation.json` records tool versions, host, source identity, dependency pins,
input/output digests, panic counts, key/operator profiles, and all
implementation/harness/fixture hashes.

For CI installations without the repository-local toolchain, use ordinary PATH
names. Flags override the corresponding environment variables:

```sh
CARGO_HOME="$PWD/.toolchain/port-cargo-home" \
FERRUGO_CARGO=cargo FERRUGO_RUSTC=rustc FERRUGO_GO=go \
go run ./tools/port-cargo-graph -check
```

Each executable is selected from its `FERRUGO_*` override, then an available
repository-local toolchain, then PATH. `-cargo`, `-rustc`, and `-go` accept bare
PATH names, paths relative to `-root`, or absolute paths. An explicit invalid or
empty override fails; it never falls back to another executable. `CARGO_HOME`
is respected unless `-cargo-home` overrides it; without either override the
isolated cache above is used. In check mode the report's original Go version and
host remain generation provenance rather than restricting the validating host;
all source, harness, dependency and result hashes remain checked, and the actual
current Go version and host are printed by the command.
