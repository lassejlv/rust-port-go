# Collection conversion and extension

`generate_conversion.py` compiles `conversion_oracle.rs` against the unchanged,
verified complete pinned Rust graph. It retains 1,296 native cases, including
302 SIGABRT prefixes, across nine concrete strategies:

- Map, Set and Bag `From<UnordItems>`.
- Map, Set and Bag `ExtendUnord`.
- `From<FxHashSet>` allocation ownership transfer.
- Bag conversion and extension from the actual owned Vec iterator.

The input overrides `collect` with a failure, so conversions must select
`FromIterator` directly. Its independently observable `for_each` and `fold`
confirm the source's Map and Set dispatch difference. Real Value Hash/Eq/Drop,
input Drop, iteration and callback failures remain observable. The Vec strategy
uses actual Index storage and its existing owning iterator specialization; it
does not introduce next/hint callbacks. The FxSet strategy moves the existing
set without a new hash or equality operation.

Additional legal custom iterator implementations put the borrowed insertion
consumer in a Drop guard, raise and catch a nested panic, and continue the
original operation. Hash/Eq/Drop called from that guard must receive the
guard's current unwind context. Subsequent callbacks resume the ordinary
context. The retained matrix includes ordinary, already-unwinding, and
panic-hook operations, with fresh and resumed source failures.

`UnordCollectionIteratorOps` keeps the actual legacy iterator methods and the
primary scoped ForEach/Fold witnesses in one static dictionary. New
`rusthashmap/extend_scoped.go` helpers retain the existing reservation,
insertion, adapter and owned cleanup strategies while forwarding each
borrowed consumer invocation's current context. Existing engine bodies remain
unchanged. `rustvec/scoped_consumer.go` owns that scoped operation protocol;
its independent tests cover A/B/nil metadata, transfers, actual cloned state,
interleaving, valid recursion and concurrent distinct iterator instances.

`--check` regenerates native transcripts and runs the minimum-Go retained
comparison. `test_conversion_tools.py` checks both corpus provenance and the
current-context transition. This scoped proof grants no Rust std/hashbrown
file completion credit. Remaining constructor/reduction contracts and the
final source/API audit still govern whole `unord.rs` completion.
