# Source contract completion review

Full-file completion includes the source's derived traits and observable
interactions with its dependencies. Matching an algorithm and default Debug
text is insufficient when a caller can supply other formatter options, a
fallible writer, independent character writes or owned callback state.

Review of the typed-storage block and follow-up API review exposed gaps in
seven earlier implementation records. Six remain reopened as `unported`,
retaining all existing port and comparison evidence. Union-find's gaps closed
in checkpoint 18. The reopened status means full-file work remains; it does
not remove the implemented algorithms or their tests.

| Upstream file | Remaining contract |
| --- | --- |
| Rust `compiler/rustc_lexer/src/lib.rs` | Complete derived Debug formatter, options, writer errors and caller state |
| Rust `compiler/rustc_data_structures/src/base_n.rs` | Display formatter/writer and owned generic conversion behavior |
| Rust `compiler/rustc_data_structures/src/binary_search_util/mod.rs` | Owned extracted-key temporaries, borrowed key operator cells and owned extractor cleanup |
| Rust `compiler/rustc_parse_format/src/lib.rs` | Complete derived Debug formatter for all source types and fallible output |
| Cargo `src/util/graph.rs` | Borrowed Display/Debug writer dispatch and remaining generic persistent-container observations |
| Cargo `crates/cargo-util-schemas/src/restricted_names.rs` | Complete derived Debug and Display writer contracts |

Union-find now exposes full mutable Debug formatting on actual parent cells
and a primary snapshot API returning the validated `IndexVec<Key, Key>`, with
the earlier slice-shaped snapshot retained as a convenience. Its 25,040 native
formatter records complement the existing 3,788 algorithm records. The new
slice/vector implementation and all five original vector tests have their own
completed file records; no general standard-library source credit is inferred.

The separately inventoried `rustc-literal-escaper 0.0.8` implementation is marked
`partial` for its formatter and owned callback contracts. Its complete archive
inventory, nine mirrored tests, 27 benchmark workloads and escaping/scalar
comparisons remain retained. Archive test and benchmark files do not gain
completion credit for their implementation dependency.

Concrete reference tests demonstrate why this matters. Rust integer Debug can
call `write_char` independently from `write_str`; alternate formatting can emit
an empty string write; a failing zero-padding operation can leave fill/alignment
changed in the caller's formatter. Compact builders share formatter state while
pretty builders isolate child state. Replacing that behavior with one computed
string loses callbacks, errors and later observable state.

The new hash and storage ports compare those interactions directly against
unchanged original source. A shared supporting builder runtime is being
extracted from independently checked implementations so earlier ports can use
the same complete contracts. Supporting adapters receive no extra upstream
source-file credit. Each reopened row can return to `validated` only after its
missing behavior is implemented and reference evidence is retained.

This is an ongoing source/API review, not an assertion that undiscovered gaps
cannot exist. Historical checkpoint counts describe records at that time; the
current inventory includes these corrections. Production Rust compilation
still requires completion and integration of the broader compiler and Cargo.
