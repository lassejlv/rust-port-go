# Source contract completion review

Full-file completion includes the source's derived traits and observable
interactions with its dependencies. Matching an algorithm and default Debug
text is insufficient when a caller can supply other formatter options, a
fallible writer, independent character writes or owned callback state.

Review of the typed-storage block and follow-up API review exposed gaps in
seven earlier implementation records. All seven have now closed: union-find in
checkpoint18; lexer, binary search, format parser and restricted names in19;
Base-N in23; and Cargo graph in33. Historical evidence remained retained while
those rows were reopened. Independent source/API review of the larger compiler
files continues before their completion credit.

Cargo graph now uses a primary owned/borrowed interface backed by genuine Rc
persistent containers, exact source comparisons/cleanup and complete fallible
formatting. Its main/equality proofs retain67,662 observations across all four
assertion/overflow profiles and all three original tests. Seventy-two actual
whole-graph stdio cases additionally check shared capture bytes/capacity/Arc
counts, pre-held leases and IO guard release before panic hooks. Supporting
stdio, libc and persistent-container operations remain scoped dependencies;
this closure gives no entire std/im-rc/Arc/TLS source-file credit.

Base-N now consumes arbitrary owned `Self`, dispatches independent method
overrides, completes conversion/destruction before static length calculation,
and returns actual borrowed buffer views. Its 26,752 full formatter records
and 16,656 ownership records cover all four arithmetic/assertion profiles,
including 8,952 native abort observations. Existing numeric evidence is retained.

Union-find now exposes full mutable Debug formatting on actual parent cells
and a primary snapshot API returning the validated `IndexVec<Key, Key>`, with
the earlier slice-shaped snapshot retained as a convenience. Its 25,040 native
formatter records complement the existing 3,788 algorithm records. The new
slice/vector implementation and all five original vector tests have their own
completed file records; no general standard-library source credit is inferred.

The separately inventoried `rustc-literal-escaper 0.0.8` implementation now
includes its complete formatter and owned callback contracts. Its complete
archive inventory, nine mirrored tests, 27 benchmark workloads and escaping/
scalar comparisons remain retained. These dependency records stay separate
from top-level Rust/Cargo source counts.

The lexer additionally has its lazy consumed iterator, structured ordering,
independent build profiles and contextual internal failures checked against
unchanged Rust. Binary search owns each extracted temporary and extractor,
borrows actual query/operator cells, and handles full-width zero-sized input.
Format parser and name errors now preserve full mutable formatter state,
writer errors and independent character/string dispatch. Each closure has
retained native evidence under its source-specific tool directory.

Concrete reference tests demonstrate why this matters. Rust integer Debug can
call `write_char` independently from `write_str`; alternate formatting can emit
an empty string write; a failing zero-padding operation can leave fill/alignment
changed in the caller's formatter. Compact builders share formatter state while
pretty builders isolate child state. Replacing that behavior with one computed
string loses callbacks, errors and later observable state.

The new hash and storage ports compare those interactions directly against
unchanged original source. A shared supporting builder runtime is being
extracted from independently checked implementations so earlier ports can use
the same complete contracts. That shared runtime is now integrated. Supporting adapters receive no extra upstream
source-file credit. Each reopened row can return to `validated` only after its
missing behavior is implemented and reference evidence is retained.

This is an ongoing source/API review, not an assertion that undiscovered gaps
cannot exist. Historical checkpoint counts describe records at that time; the
current inventory includes these corrections. Production Rust compilation
still requires completion and integration of the broader compiler and Cargo.
