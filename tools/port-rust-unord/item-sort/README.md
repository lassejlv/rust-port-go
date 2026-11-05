# Generic and collection sorting from pinned `unord.rs`

These observers compile the unchanged complete Rust dependency graph through
`tools/port-rust-span-lib/reference.py`. The loader verifies the pinned source,
compiler, provenance and graph outputs before constructing compiler arguments.
The live `--check` mode regenerates native cases and compares Go's retained
replays. `--skip-go` generates the original-side proof alone. `--native-only`
also writes the Rust-only manifest. No source file or Rust SDK engine is copied
into an oracle driver.

| Driver / generator | Native cases | SIGABRT prefixes | Scope |
| --- | ---: | ---: | --- |
| `oracle.rs` / `generate.py` | 864 | 268 | Generic C, actual I.collect override, observable BorrowMut, owned P and C return-place behavior |
| `vector_oracle.rs` / `generate_vector.py` | 864 | 160 | All three Items into-sorted forms, real VecIntoIter allocation reuse |
| `zero_oracle.rs` / `generate_zero.py` | 828 | 164 | Logical inhabited no-Drop ZST C, usizeMAX, cached capacity boundary, BorrowMut and return-place cleanup |
| `collections_oracle.rs` / `generate_collections.py` | 3072 | 800 | All eight borrowed/owned Map/Set key/StableCompare sorting methods |
| `values_oracle.rs` / `generate_values.py` | 2112 | 528 | Owned ValuesSorted map iterator, direct/borrowed collection and partial/caught consumption |

Every family includes ordinary execution, an operation inside an outer Drop
unwind, an operation inside a panic hook, and fresh/resumed panic payloads.
Callbacks and automatic field cleanup remain owned, separately observable
operations. Generic Items' uncached branch uses stable key sorting; the private
Map/Set helper uses unstable key sorting. Map/Set StableCompare always uses
unstable sorting, independently from its constant. Key sorting selects
PartialOrd::lt, with its real default dispatching to partial_cmp; Ord::cmp is
an independent observer.

The ValuesSorted oracle's allocator records the actual pair-vector allocation
and checks its identity against the final direct Vec result in the same native
process. Its full native stdout and allocation events are retained separately
from the Go comparison transcript. Go reproduces the source pointer-buffer
ownership and capacity decision, without claiming the Rust allocator or ABI.
Direct map collection retains twice the pair vector's element capacity;
by_ref collection has the exact TrustedLen capacity. Native cases cover zero
input, skipped prefixes, an exhausted prefix and a caught borrowed callback
panic followed by owning collection.

The concrete Go compaction accepts only `BorrowedPair[K,V]`, whose two fields
are typed pointers. It checks both offsets, size and alignment against pointers
and bounds the doubled capacity before creating its private unsafe slice. The
vector moves into its actual owning iterator with no surviving backing alias.
The i-th source read clears pair slots 2*i and 2*i+1 before output slot j<=i is
written, so no unread source is overwritten. The unused pointer tail is cleared.
After successful collection, the exhausted source iterator is deliberately
forgotten and allocation ownership passes to the compacted output. Partial or
failed operations keep ordinary owned iterator cleanup. The private pointer
view is never exposed before compaction.

These are scoped source/ownership proofs. The remaining generic constructor,
reduction, conversion and complete API review determine whole `unord.rs` credit;
none of these corpora grants Rust std, hashbrown or serialize source credit.
