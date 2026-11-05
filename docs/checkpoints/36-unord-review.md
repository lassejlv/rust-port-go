# Independent re-review: final Unord bound delta

Source/API delta review: **PASS** for the exact frozen candidate
`/tmp/ferrugo-unord-bound-delta.hh4aotew`. The prior missing-bound family and
CloneFrom Self/exposure findings are closed. The earlier full-source body review
in `/tmp/ferrugo-unord-independent-review.md` carries forward because this delta
does not substitute any runtime algorithm. Full-file ledger acceptance remains
pending Root's eight declared downstream producer rebinds and integration gates.

## Exact reviewed scope

- All 46 delta paths match their recorded historical before hashes (or null for
  nine new paths) and final after hashes: nine new, 37 modified.
- All 162 unique owned paths and three unchanged shared paths match the final
  hash tables and combined structured table.
- The only modification to an existing production Go file is the guard call at
  `unord_items.go:138`. All 13 modified prior generators add only the new helper
  source binding. All 22 modified prior metadata files preserve their non-file-
  binding content exactly.
- All 35 future metadata file/reference tables match candidate bytes. All 23
  previous Unord gzip fixtures and the separate cached-ZST fixture are unchanged
  against the historical snapshot.
- Main new production file SHA256:
  `1f5ad0c8506beb67ac69950ab19e55de54bc39fcab91a449128497160aa855ec`.
  Modified Items file SHA256:
  `89c74b62b5945e5b6711e4940e67aba2772b0e4f74478578d2b0dddfaa8ec4ea`.

## Closed API findings

References below are within
`port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go` unless
otherwise stated.

| Prior obligation | Reviewed closure |
| --- | --- |
| Bag derived V:Default, source unord.rs:683 | Lines 24–26 require the actual Default method with owned result; lines 80–82 forward empty construction without invoking it. |
| Items derived T:Clone, source unord.rs:33 | Lines 30–32 carry the required owned Clone method, in addition to the existing actual I::clone slot; lines 83–88 forward Clone/CloneFrom. T::clone remains unused. |
| Copy and Clone, source unord.rs:145–156 | Lines 37–40 include Clone plus the Copy marker; lines 115–125 cover both source-reference representations. Copy does not invoke Clone or the marker. |
| T:Ord for max/min, source unord.rs:79–92 | Lines 45–49 require eq, partial_cmp, cmp and Eq laws; lines 103–108 retain the independently selected reduction. |
| Result Sum<T>/Product<T>, source unord.rs:95–107 | Lines 73–78 carry the required result trait operations monomorphized at actual S:Iterator<Item=T>; lines 109–114 retain independent I overrides. |
| Associated KeyType:Ord+Sized+StableHash, stable_hash.rs:112–114 | Lines 55–66 retain required Ord/supertrait and StableHash operations plus the actual finite source layout. Lines 127–147 cover all seven key-sort entrypoints and forward the original Used dictionary unchanged. |
| CloneFrom exact Self and receiver Clone exposure | Helper lines 94–101 rejects an opaque destination or different instantiated private cell[S,T]. Its call in unord_items.go:138 precedes source.Clone at line 139 and old Drop at line 146. Source.Clone retains its own exposure check. |

These are borrowed static trait dictionaries. There is no extra owned capture,
source destructor, unused-method invocation, nil probe, or comparison-method
substitution. Actual trait coherence and source borrowing/lifetime laws remain
obligations of lawful dictionaries. The approved Sum/Product shapes represent
the required generic method at concrete S; coherence with other trait invocations
inside an I override remains a source-type obligation. This does not demand an
unrelated complete std trait-file port.

The exact-S guard compares type descriptors, not state data, so noncomparable
states are immaterial and genuine S aliases pass. Both values are checked live
before the helper. Rejection occurs before clone, replacement or Drop and leaves
both owners intact. The previous clone-complete-before-replace/DropAndReplace
behavior is unchanged for valid same-S inputs.

## Evidence review

Read the complete new production file, new Go witness/replay, native admission
and effect programs, producer, integrity checks, updated inventory and all
generator diffs. The retained native manifests agree on all 18 admission checks
and 22 effect programs. Admissions distinguish absent Default, Clone/Copy, Ord,
Sum/Product, associated Eq/PartialOrd/Ord/StableHash/Sized, wrong Self and opaque
Clone. Positive programs include unrestricted new<NoDefault>.

The effect programs retain traps for unused required methods while observing
actual I overrides and key lt versus partial_cmp. Go replay covers both borrowed
reference forms, same-S aliases and rejected wrong-S/opaque destinations before
effects, followed by exactly-once owner cleanup. Empty Bag Default retains zero
capacity followed by the source capacity-four first push; the native allocation
observation is not a Rust ABI or Go allocator claim. The key witness retains
actual non-Copy Key metadata and shares the used partial_cmp implementation.

Reviewer independently ran the compiler-free bounds integrity suite: three
checks passed in 0.002s. Source/hash checks above also passed. The owner's
reported minimum-Go replay, all 19 groups under race (55.588s), scoped vet and
actual native-plus-Go producer --check are coherent with the reviewed evidence;
this reviewer did not rerun Go/native compilers, regenerate fixtures, edit Git
or change any repository/ledger file.

## Remaining integration boundary

The eight accepted manifests still bind prior rustvec/collect.go SHA256
`af48d40913abdf1a041d3fbb6efa21bb8216169149d1a5c4ce8ecdfd7a0aea6a` while the
unchanged candidate bridge has
`1e8f231b6af033b15370ff1737f85e4eccaf92c7447f7942338cbf1976e64212`:

- Unord panic manifest.
- Span consuming-callbacks, remaining-context, span-geometry, span-hygiene.
- CachingSourceMapView base, context, metadata.

Root must genuinely regenerate their exact companions through the actual
producers, retain original fixture bytes and complete the relevant accepted
integration gates. No further source/API blocker was found in this delta.
Full-file credit and dependent aliases remain Root decisions after integration;
this review grants no whole std, hashbrown, Vec, sort, stable_hash.rs or compiler
completion credit.
