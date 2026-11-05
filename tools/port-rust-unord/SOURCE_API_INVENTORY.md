# Complete pinned Unord source/API review inventory

Status: **implementation candidate; complete-bound delta gates pass; independent full-file acceptance remains pending**.
This inventory covers every source function declaration/body, all four source
types, explicit trait implementations, derived traits, inherited defaults,
private helpers, and negative IntoIterator implementations. It does not grant
source credit to HashMap/hashbrown, Vec, standard iterators/sorts, serialization,
or the compiler generally. Root owns final acceptance and the progress ledger.

## Pinned source and configuration

- Repository revision: `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.
- Source: `compiler/rustc_data_structures/src/unord.rs`, **796 lines, 23,193 bytes**.
- Git blob: `42dbe37db7951fb619afc11054ad20952e7a5098`.
- SHA-256: `5b40df5f08b9cd2ec1f7806f76937b623f2de60d3de31dae12c009bee5509e99`.
- Native compiler: `rustc 1.101.0-nightly (708c94ef0 2026-11-03)`, SDK
  `nightly-2026-11-04`; verified source graph d1/o1/x86_64 profile.
- This source file has no local `cfg` arms. Imported traits, macro derives,
  standard library specialization, target sizes, and feature configuration are
  taken from that exact verified graph. Runtime Go dictionaries explicitly
  witness the selected source types/traits/layouts; they do not assert a Rust
  ABI for Go values. Native observations are for the pinned profile. Go 1.23 is
  the declared minimum and the pending final gate uses that compiler.

## Every source function and trait method

Each row points at the precise Rust declaration and its Go primary. Type
parameters in Go retain the actual concrete source Self/result types where Go
cannot express Rust generic trait methods or opaque return types. The supplied
operation is the selected monomorphized source implementation, including real
independent overrides and ownership; it is not permission to replace the source
method with a Next-only traversal. Method names without the full signature below
are link labels, not additional public APIs.

| Rust source | Source operation | Go primary | Contract retained |
| --- | --- | --- | --- |
| [L38](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L38) | `Items::new` | [`NewUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items.go#L53) | Owned actual S; required selected methods |
| [L43](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L43) | `Items::map` | [`MapUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go#L75) | Selected I::map<S,F,R>; opaque Clone hidden |
| [L48](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L48) | `Items::all` | [`(items *UnordItems[T]) All`](../../port/rust/compiler/rustc_data_structures/src/unord_items_boolean.go#L137) | Independent all/try_fold; owned check(f) |
| [L53](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L53) | `Items::any` | [`(items *UnordItems[T]) Any`](../../port/rust/compiler/rustc_data_structures/src/unord_items_boolean.go#L140) | Independent any/try_fold; owned check(f) |
| [L58](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L58) | `Items::filter` | [`FilterUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go#L79) | F: shared Fn; actual adapter invokes FnMut |
| [L63](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L63) | `Items::filter_map` | [`FilterMapUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go#L87) | F: shared Fn; owned Option result |
| [L71](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L71) | `Items::chain` | [`ChainUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go#L120) | Own both actual iterators; selected constructor |
| [L79](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L79) | `Items::max` | [`MaxUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L103) | T: Ord; independent selected I::max/reduce/fold |
| [L87](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L87) | `Items::min` | [`MinUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L106) | T: Ord; independent selected I::min/reduce/fold |
| [L95](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L95) | `Items::sum` | [`SumUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L109) | Independent I::sum, including S::sum default |
| [L103](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L103) | `Items::product` | [`ProductUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L112) | Independent I::product, including S::product default |
| [L111](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L111) | `Items::count` | [`(items *UnordItems[T]) Count`](../../port/rust/compiler/rustc_data_structures/src/unord_items.go#L148) | Independent selected I::count |
| [L116](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L116) | `Items::flat_map` | [`FlatMapUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go#L95) | Actual owned move wrapper invokes original F through shared Fn |
| [L124](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L124) | `Items::collect` | [`CollectUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go#L186) | C::From<UnordItems>; never substitutes FromIterator |
| [L130](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L130) | `Items::get_only` | [`(items *UnordItems[T]) GetOnly`](../../port/rust/compiler/rustc_data_structures/src/unord_items.go#L167) | Exactly two selected next calls; track_caller metadata |
| [L140](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L140) | `Items::empty` | [`EmptyUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items.go#L186) | Actual Empty<T>; Clone additionally requires T: Clone |
| [L147](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L147) | `Items::cloned` | [`ClonedBorrowedUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L121) | Actual *T collection references; rustvec.Ref form also in delegate |
| [L154](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L154) | `Items::copied` | [`CopiedBorrowedUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L124) | Actual *T collection references; Copy never invokes Clone |
| [L161](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L161) | `Items::into_sorted` | [`IntoSortedUnordItemsWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L130) | Vec collector; cache key true |
| [L169](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L169) | `Items::into_sorted_stable_ord` | [`IntoSortedUnordItemsStableOrd[`](../../port/rust/compiler/rustc_data_structures/src/unord_items_vector.go#L70) | Identity projection and StableCompare |
| [L177](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L177) | `Items::into_sorted_stable_ord_by_key` | [`IntoSortedUnordItemsStableOrdByKey`](../../port/rust/compiler/rustc_data_structures/src/unord_items_vector.go#L54) | Owned shared-Fn projection, borrowed returned key |
| [L186](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L186) | `Items::collect_sorted` | [`CollectUnordItemsSortedWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L127) | Actual I::collect<C>; BorrowMut always; false branch STABLE |
| [L207](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L207) | `Items::collect_stable_ord_by_key` | [`CollectUnordItemsStableOrdByKey`](../../port/rust/compiler/rustc_data_structures/src/unord_items_sort.go#L133) | Actual I::collect<C>; branch on CAN_USE_UNSTABLE_SORT |
| [L262](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L262) | `Set::Default::default` | [`NewUnordSet[`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L27) | Fixed FxBuildHasher, empty storage; const source impl |
| [L269](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L269) | `Set::new` | [`NewUnordSet[`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L27) | Same concrete default field value |
| [L274](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L274) | `Set::with_capacity` | [`NewUnordSetWithCapacity`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L30) | Fixed FxBuildHasher |
| [L279](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L279) | `Set::len` | [`(s *UnordSet[T]) Len`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L42) | Borrowed inner length |
| [L284](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L284) | `Set::is_empty` | [`(s *UnordSet[T]) IsEmpty`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L43) | Borrowed length == 0 |
| [L290](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L290) | `Set::get_only` | [`(s *UnordSet[T]) GetOnly`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L47) | Borrowed first element only at length 1 |
| [L295](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L295) | `Set::insert` | [`(s *UnordSet[T]) InsertOwned`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L60) | Primary owned argument; scalar Insert convenience |
| [L300](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L300) | `Set::contains<Q>` | [`UnordSetContainsBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L75) | Borrow<Q>, Q: ?Sized + Eq + Hash witness |
| [L309](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L309) | `Set::remove<Q>` | [`UnordSetRemoveBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L80) | Borrowed lookup and actual removed-value Drop |
| [L318](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L318) | `Set::items` | [`(items *UnordSetItems[T]) General`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L197) | Items().General() transfers borrowed concrete state |
| [L323](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L323) | `Set::into_items` | [`(collection *UnordSet[T]) IntoItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L277) | Opaque owned actual SetIntoIter |
| [L334](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L334) | `Set::to_sorted` | [`UnordSetToSortedWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L139) | Borrowed references; false branch UNSTABLE |
| [L347](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L347) | `Set::to_sorted_stable_ord` | [`UnordSetToSortedStableOrd`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_sort.go#L150) | Always UNSTABLE regardless of CAN_USE_UNSTABLE_SORT |
| [L361](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L361) | `Set::into_sorted_stable_ord` | [`UnordSetIntoSortedStableOrd`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_sort.go#L165) | Owned elements; always UNSTABLE |
| [L377](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L377) | `Set::into_sorted` | [`UnordSetIntoSortedWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L142) | Owned elements; false branch UNSTABLE |
| [L386](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L386) | `Set::clear` | [`(s *UnordSet[T]) Clear`](../../port/rust/compiler/rustc_data_structures/src/unord_set.go#L85) | Actual hash-set clear and element Drop |
| [L397](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L397) | `ExtendUnord::extend_unord declaration` | [`ExtendUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L23) | Typed selected Extend<S>; exact private marker bound |
| [L405](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L405) | `blanket ExtendUnord::extend_unord body` | [`ExtendUnordItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L23) | Takes actual concrete I, never wraps items as IntoIterator |
| [L412](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L412) | `Set::Extend::extend` | [`(s *UnordSet[T]) ExtendScoped`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_iterator_scoped.go#L20) | Actual IntoIterator Self; mapped fold and current consumer context |
| [L419](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L419) | `Set::FromIterator::from_iter` | [`UnordSetFromIteratorScoped`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_iterator_scoped.go#L42) | Actual FxHashSet::from_iter, independent of collect override |
| [L425](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L425) | `Set::From<FxHashSet>::from` | [`UnordSetFromFxSet`](../../port/rust/compiler/rustc_data_structures/src/unord_items_conversion.go#L15) | Move same allocation; no hash/eq/rebuild |
| [L431](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L431) | `Set::From<UnordItems>::from` | [`UnordSetFromItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_conversion.go#L47) | Actual I next/hint/independent fold; scoped consumer |
| [L438](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L438) | `Set::StableHash::stable_hash` | [`(s *UnordSet[T]) StableHash`](../../port/rust/compiler/rustc_data_structures/src/unord_stable_hash.go#L64) | Order-independent helper; length and per-element StableHash |
| [L461](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L461) | `Map::Default::default` | [`NewUnordMap[`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L34) | Fixed FxBuildHasher, empty storage; const source impl |
| [L468](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L468) | `Map::Extend::extend` | [`(m *UnordMap[K, V]) ExtendScoped`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_iterator_scoped.go#L14) | Actual IntoIterator Self; independent for_each/current consumer context |
| [L475](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L475) | `Map::FromIterator::from_iter` | [`UnordMapFromIteratorScoped`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_iterator_scoped.go#L28) | Actual FxHashMap::from_iter; no intermediate map |
| [L482](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L482) | `Map::From<UnordItems>::from` | [`UnordMapFromItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_conversion.go#L36) | Actual I next/hint/independent for_each; scoped consumer |
| [L489](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L489) | `Map::with_capacity` | [`NewUnordMapWithCapacity`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L37) | Fixed FxBuildHasher |
| [L494](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L494) | `Map::len` | [`(m *UnordMap[K, V]) Len`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L49) | Borrowed inner length |
| [L499](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L499) | `Map::insert` | [`(m *UnordMap[K, V]) InsertOwned`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L56) | Own key/value; return actual owned replaced value |
| [L504](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L504) | `Map::try_insert` | [`(m *UnordMap[K, V]) TryInsertOwned`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L66) | Actual OccupiedError owns rejected V and borrows entry |
| [L509](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L509) | `Map::contains_key<Q>` | [`UnordMapContainsBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_map_traits.go#L34) | Borrow<Q>, Q: ?Sized + Hash + Eq witness |
| [L518](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L518) | `Map::is_empty` | [`(m *UnordMap[K, V]) IsEmpty`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L50) | Borrowed length == 0 |
| [L523](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L523) | `Map::entry` | [`(m *UnordMap[K, V]) EntryOwned`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L76) | Real Entry and owned input key |
| [L528](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L528) | `Map::get<Q>` | [`UnordMapGetBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_map_traits.go#L26) | Shared V reference; actual borrowed query type |
| [L537](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L537) | `Map::get_mut<Q>` | [`UnordMapGetMutBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_map_traits.go#L31) | Exclusive V reference; actual borrowed query type |
| [L546](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L546) | `Map::remove<Q>` | [`UnordMapRemoveOwnedBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_map_traits.go#L38) | Actual owned removed value; scalar convenience available |
| [L555](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L555) | `Map::items` | [`(items *UnordMapItems[K, V]) General`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L45) | Items().General() transfers actual borrowed pair iterator |
| [L560](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L560) | `Map::into_items` | [`(collection *UnordMap[K, V]) IntoItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L251) | Opaque owned actual IntoIter with pair field cleanup |
| [L565](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L565) | `Map::keys` | [`(collection *UnordMap[K, V]) Keys`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L81) | Opaque borrowed key iterator |
| [L576](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L576) | `Map::to_sorted` | [`UnordMapToSortedWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L133) | Borrowed pairs; false branch UNSTABLE |
| [L588](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L588) | `Map::to_sorted_stable_ord` | [`UnordMapToSortedStableOrd`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_sort.go#L120) | Always UNSTABLE |
| [L604](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L604) | `Map::into_sorted` | [`UnordMapIntoSortedWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L136) | Owned pairs; false branch UNSTABLE |
| [L616](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L616) | `Map::into_sorted_stable_ord` | [`UnordMapIntoSortedStableOrd`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_sort.go#L135) | Owned pairs; always UNSTABLE |
| [L633](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L633) | `Map::values_sorted` | [`UnordMapValuesSortedWithBounds`](../../port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go#L145) | Actual IntoIter.map value iterator, methods/collection specialization |
| [L648](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L648) | `Map::clear` | [`(m *UnordMap[K, V]) Clear`](../../port/rust/compiler/rustc_data_structures/src/unord_map.go#L114) | Actual inner map clear |
| [L661](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L661) | `Map::Index<&Q>::index` | [`UnordMapIndexBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_map_traits.go#L48) | Actual borrowed query, missing-key panic metadata |
| [L668](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L668) | `Map::StableHash::stable_hash` | [`(m *UnordMap[K, V]) StableHash`](../../port/rust/compiler/rustc_data_structures/src/unord_stable_hash.go#L44) | Pair hashes via order-independent helper |
| [L690](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L690) | `Bag::new` | [`NewUnordBag`](../../port/rust/compiler/rustc_data_structures/src/unord_bag.go#L52) | Default actual Vec storage |
| [L695](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L695) | `Bag::len` | [`(bag *UnordBag[T]) Len`](../../port/rust/compiler/rustc_data_structures/src/unord_bag.go#L68) | Actual inner vector length |
| [L700](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L700) | `Bag::push` | [`(bag *UnordBag[T]) PushOwned`](../../port/rust/compiler/rustc_data_structures/src/unord_bag.go#L77) | Own actual input value; scalar Push convenience |
| [L705](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L705) | `Bag::items` | [`(items *UnordBagItems[T]) General`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L120) | Items().General() transfers actual slice iterator |
| [L710](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L710) | `Bag::into_items` | [`(items *UnordBagOwnedItems[T]) General`](../../port/rust/compiler/rustc_data_structures/src/unord_items_collections.go#L157) | IntoItems().General() transfers actual Vec::IntoIter |
| [L718](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L718) | `Bag::Extend::extend` | [`UnordBagExtendIntoIterator`](../../port/rust/compiler/rustc_data_structures/src/unord_bag_iterator_owned.go#L15) | Explicit owned actual IntoIterator S; selected Vec::extend<S> |
| [L724](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L724) | `Bag::From<UnordItems>::from` | [`UnordBagFromItems`](../../port/rust/compiler/rustc_data_structures/src/unord_items_conversion.go#L80) | Actual Vec::from_iter<I>, independent of I::collect |
| [L731](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L731) | `Bag::StableHash::stable_hash` | [`UnordBagStableHash`](../../port/rust/compiler/rustc_data_structures/src/unord_bag_stable_bounds.go#L43) | Complete borrowed Hash + Eq + StableHash dictionary; unused bounds have no body effects |
| [L737](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L737) | `private to_sorted_vec` | [`unordCollectionSort`](../../port/rust/compiler/rustc_data_structures/src/unord_collections_sort.go#L62) | Actual iterator collect + shared key extraction; cached/UNSTABLE branches |
| [L758](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L758) | `private hash_iter_order_independent` | [`hashUnordBorrowed`](../../port/rust/compiler/rustc_data_structures/src/unord_stable_hash.go#L23) | Exact size; 0/1/many fingerprint commutative combination |

## Types, explicit traits, derives, and inherited defaults

| Source declaration | Go representation and required behavior |
| --- | --- |
| [L33](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L33) `UnordItems<T,I>` and derive `Clone` | `UnordItems[T]` owns a typed `unordIteratorCell[S,T]`. The complete bound primary requires the actual I::clone slot and a borrowed T::Clone method dictionary; legacy `CloneItem` remains exposure metadata. It calls only I::clone. Opaque returned adapters have `clonePublic=false` even if concrete R is Clone. |
| [L242](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L242) private `UnordCollection`; implementations at L258/L457/L715 | `UnordCollection` is a private-method constraint plus the exact-owner guard in `unord_collection_marker.go`. Genuine Go aliases pass; distinct embedding wrappers, including noncomparable values and nil outer pointers, are rejected before conversion or selected Extend effects. |
| [L253](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L253) `UnordSet<V>` | Actual Fx hash set, V: Eq + Hash supplied by concrete key dictionary. Its derived Debug/Eq/PartialEq/Clone/Encodable_NoContext/Decodable_NoContext reside in `unord_set.go`, `unord_codec.go`, and accepted concrete support. |
| [L260](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L260) Set const `Default` | `NewUnordSet` constructs the same empty field value and fixed Fx builder. Go executes the value construction at runtime; this is not an implementation of Rust const evaluation. |
| [L391](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L391) `ExtendUnord<T>`; blanket bound L403 | Generic primary and three family adapters take the actual source collection and concrete I. It requires both actual Extend and exact private collection Self; ordered Vec and arbitrary wrappers do not acquire this bound. |
| [L452](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L452) `UnordMap<K,V>` | Actual Fx hash map, K: Eq + Hash; V's selected traits are supplied only where required. Derived Debug/Eq/PartialEq/Clone/codec behavior is in `unord_map.go`, `unord_map_traits.go`, `unord_codec.go`, and accepted concrete support. |
| [L459](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L459) Map const `Default` | `NewUnordMap` constructs the same empty field value and fixed Fx builder; runtime value construction, not Rust const evaluation. |
| [L653](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L653) `Index<&Q>` | `UnordMapIndexBorrowed` retains K: Borrow<Q>, Q: ?Sized + Eq + Hash and references the actual stored V. |
| [L683](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L683) `UnordBag<V>` and derives | `UnordBag[T]` owns actual `IndexVec`/Vec storage. Derived Default now has a complete unused T::Default dictionary in `DefaultUnordBagWithBounds`; Debug/Eq/PartialEq/Clone and empty construction stay in accepted `unord_bag.go`; codec in accepted `unord_bag_codec.go`; actual source value operations and vector specialization remain selected. |
| Inherited `Clone::clone_from` on all four derived types | Default clone-complete-new-before-replace/drop-old, not a call to inner's independent clone_from. `CloneFrom` on each family and Items retains replacement when old Drop fails and prepared-result ownership. |
| Inherited `PartialEq::ne` on Set/Map/Bag | `NotEqual` calls the derived Eq path and negates it. The actual inner equality remains selected: in particular the Vec/slice equality loop may invoke the element's independent `ne`. The wrapper's inherited `ne` itself negates its derived `eq`; Eq is the source marker. |
| Automatic field/parameter destruction | Explicit `Owned` cells and call-current Context model source moves, partial cleanup, caught unwind, panic-hook execution, and prepared-return leaks. Close is the explicit Go owner cleanup operation. |
| [L793](https://github.com/rust-lang/rust/blob/2d20b42b883dcfc014f3c2c56081eeb6091c2179/compiler/rustc_data_structures/src/unord.rs#L793)–L796 negative `IntoIterator` for Bag/Set/Map/Items | These unordered types and item handles expose no public Next/IntoIterator. They support only source unordered methods. `values_sorted` intentionally returns an ordinary iterator because its order is determined by sorted keys. |

The source's shared `Fn` requirement is represented by `UnordFunction` with real
independent Fn/FnMut/FnOnce slots and an owned environment. TryFold's internal
owned checker exposes only FnMut/FnOnce. FlatMap constructs its actual owned
`move |x| f(x).0` wrapper in production; all wrapper call traits invoke the
original F through shared Fn. Borrowed inputs/results are actual references to
original storage with source mutation/lifetime obligations, not copied values.

Source `#[inline]` has no behavioral branch. `#[track_caller]` on get_only is
represented through call-current panic metadata. `Encodable_NoContext` and
`Decodable_NoContext` encode/decode the single `inner` field through its actual
selected collection implementation, including exact nominal opaque-byte Vec
specialization and complete collection/value cleanup.

## Selected methods, source defaults, and concrete adapters

- Source `all`/`any` independently select I's own all/any, or the actual default
  through its own try_fold. A default try_fold witness is explicit. The owned
  check(f) closure may be called, moved, forgotten, or dropped by a valid override.
- Source iterator constructors select I::map/filter/filter_map/flat_map/chain or
  cloned/copied. The actual S/R ownership and dictionary survive construction.
  Filter/FilterMap's default count uses I::fold; Cloned count uses I::fold;
  Copied count uses I::count. FlatMap count uses outer fold and inner count;
  Chain count uses both independent counts. None is silently replaced with next.
- Default extrema select I::reduce with `Ord::max/min`; their actual methods,
  reduce and fold can each be overridden independently. Sum/Product select
  I::sum/product and its actual S::sum/product trait default when appropriate.
- `Items::collect` selects C::From<UnordItems>, while sort collection selects
  I::collect<C>; Map/Set/Bag From<Items> select inner FromIterator on I. These are
  distinct source calls with distinct overrides, destruction, and prepared outputs.
- Generic Items cached-key sorting and its logical ZST path preserve BorrowMut,
  capacity-before-key failure, index-width selection, stable/unstable branch,
  shared projection calls, actual PartialOrd::lt versus Ord methods, complete
  container/key/function Drop, and prepared-container leaks. Map/Set false-cache
  sorts always use unstable key sorting; generic Items false-cache uses stable.
- `values_sorted` owns the actual sorted borrowed-pair vector and returns the
  typed value-reference map iterator. Its approved backing reuse proves exact
  Go two-pointer pair layout/alignment, exclusive consumed storage, forward safe
  compaction, bounded capacity arithmetic, and clearing the unused pointer tail.
  Transient key pointers are never exposed as V pointers. It makes no Rust ABI
  claim. Native allocation/capacity and within-process identity are retained.
- Ordinary Map/Set Extend and FromIterator use the scoped consumer primary, so a
  valid input may invoke insertion from a destructor while a nested unwind is
  caught. Actual Hash/Eq/Drop see that invocation's current context. Legacy
  same-context conveniences remain intact. The shared borrowed consumer is
  per-cell and carries real typed state; there is no global registry.
- Bag StableHash's primary requires the complete borrowed Hash + Eq + StableHash
  method dictionary. It forwards only StableHash to the unchanged accepted body;
  Hash/Eq are unused and are not probed or nil-validated. `UnordBagStableHashOps`
  reuses the existing KeyOps and StableHash method shapes. The accepted
  `bag.StableHash` convenience retains these source obligations on its caller.
- Ordinary Bag Extend's primary owns actual IntoIterator S and selects actual
  Vec::extend<S>, including conversion, Self take/forget/drop, prepared iterator
  leaks, generic traversal or real Vec::IntoIter specialization, allocation and
  capacity behavior. Existing IndexIntoIterator wrappers are conveniences.

## Explicit complete-bound delta after the historical freeze

Independent review identified source admission obligations which the historical
selected-operation conveniences documented without carrying all required trait
methods. The new coherent `unord_bound_primaries.go` family closes those API
boundaries without changing any selection, sort, collection, or ownership engine.
The prior 153+3 snapshot and exact hash tables remain historical evidence. This
is a separately hashed delta; its final Go 1.23 gates now pass. Independent re-review and Root-owned
downstream integration remain **pending** before whole-file credit.

- Bag derive Default at L683 now has `DefaultUnordBagWithBounds` and the complete
  borrowed Default method dictionary. Empty construction never calls it.
- Items derive Clone at L33 now has complete borrowed T::Clone on
  `CloneUnordItemsWithBounds` / `CloneFromUnordItemsWithBounds`, in addition to
  the real I::Clone slot. Extra T::Clone is unused. CloneFrom checks the actual
  private cell[S,T] type and receiver Clone exposure before clone/drop effects;
  true S aliases pass, distinct S or opaque destinations reject. Its existing
  clone/replace/DropAndReplace body is unchanged.
- Max/Min require complete Eq/PartialOrd/Ord membership methods while keeping
  independently selected I max/min/reduce/fold and Ord max/min operations.
- Cloned/Copied require complete Clone or Copy+Clone dictionaries, for both
  borrowed reference representations. The Copy marker is never invoked, and
  copying never calls Clone.
- Sum/Product carry required R trait operations monomorphized at actual
  S:Iterator<Item=T>, separately from the selected I override. Static coherence
  of that override's internal trait calls remains a lawful source-type
  obligation, as for other monomorphized source operation dictionaries.
- ToStableHashKey::KeyType (stable_hash.rs112–114) carries complete required eq,
  partial_cmp, cmp and StableHash methods, Eq laws, and actual finite Sized
  source layout (including ZST). All seven key-sort `...WithBounds` primaries
  keep the existing used projection, lt, partial_cmp/chaining and Drop slots
  unchanged. Unused Eq/Ord::cmp/StableHash methods are never called or probed.

All dictionaries are borrowed static metadata with no additional owning Fn
environment or destructor. No nil validation is inserted for unused methods.
The existing complete Bag Hash+Eq+StableHash primary remains unchanged.
Eighteen actual compiler admission programs and twenty-two actual native effect
programs pass; missing bounds, wrong iterator Self, opaque Clone exposure, and
unused-method traps are retained in `tools/port-rust-unord/bounds/`. Go replay passes for all twenty-two original effect programs, both borrowed
reference representations and rejection before clone/drop effects. The renewed
nineteen-group race passes in 55.588s; scoped vet and the actual native-plus-Go
live producer check pass. All thirty-five future binding tables and thirty-four
integrity checks across twelve suites are current/green.

## Historical frozen evidence and pending delta gates

Every graph producer uses the mandatory verified complete-graph loader and binds
its current inputs. Retained gzip fixture bytes are compared directly with Go
observable traces, including every abort prefix. Native fixtures are distinct
from Go witnesses; bounds compile the original unchanged types. Native-only
manifests do not bind or claim the Go implementation.

| Historical 153+3 proof group | Original native cases / abort prefixes | Historical gate state |
| --- | --- | --- |
| Function ownership | 147 / 74 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Items core + clone bounds | 126 / 40 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Boolean/owned TryFold | 1,224 / 332 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Map constructors/custom FlatMap constructor | 408 / 180 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Concrete collection item integration | 2,016 / 374 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Generic C sort | 864 / 268 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Vec sort convenience | 864 / 160 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Logical ZST generic C sort | 828 / 164 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Map/Set eight sorts | 3,072 / 800 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Collection From/Extend conversions | 1,296 / 302 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| ValuesSorted methods/storage collection | 2,112 / 528 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Borrowed cloned/copied | 720 / 74; five bounds | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Extrema/Sum/Product reductions | 2,112 / 234; four bounds | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Ordinary Map/Set IntoIterator + scoped insertion | 720 / 296 | Native, final coherent Go 1.23 race, source bindings and scoped vet PASS |
| Ordinary owned-S Bag IntoIterator | 768 / 184; 20 allocation/identity observations | Native, Go 1.23 ordinary/race/live producer, integrity and scoped vet PASS |
| Exact private collection marker | Five compile checks plus actual positive/alias programs | Native, Go 1.23 ordinary/race/live producer, integrity and scoped vet PASS |
| Default/overridden filter/FM/flat_map/chain | 1,650 / 442; nine bounds | Native, Go 1.23 ordinary/race/live producer, integrity and scoped vet PASS |
| Complete Bag StableHash bound | Four actual compiler bounds; four original fingerprint/effect observations | Native, Go 1.23 ordinary/race/live producer, integrity and scoped vet PASS |
| Cached logical ZST prerequisite | 624 / 208 | Native/Go 1.23 race/vet green; separate support scope, no std credit |

The historical frozen isolated snapshot combines Root's read-only 624-path accepted base
with exactly 153 owned candidate paths and the frozen three-file scoped iterator
bridge. All 33 future manifest source/reference binding tables match that
snapshot. Original retained fixture bytes are preserved. The Go 1.23 ordinary
four-group gate passed in 1.536s. All 18 candidate DDS groups, including 18,927
native trace cases and 4,452 abort prefixes, passed race in 58.995s. Focused
scoped-consumer and cached logical-ZST races passed in 1.019s and 3.325s. Scoped
DDS/rustvec/rusthashmap/rustsort vet passed. The four new actual native plus Go
producer `--check` gates also passed. Integrity checks across all 11 future
suites passed. These are completed gates, not a whole-file acceptance decision.

The new complete-bound delta passes its own Go 1.23 compile/replay, complete
nineteen-group race (55.588s), scoped DDS/vec/hashmap/sort vet, actual native-plus-Go
live producer and integrity gates. Its first and live Go replays each completed
in 0.004s. All twenty-three preexisting retained fixtures are byte-identical.
These new results are separate from the historical frozen results above; neither
set alone grants full-file credit before independent review and integration.

Final full-file acceptance is explicitly **pending** these Root-owned gates:

1. Independent review of the complete source/API/trait/ownership/default
   inventory and exact frozen owned/shared hashes. Root decides whether this
   entire source file can receive credit and whether its aliases can advance.
2. Coherent downstream integration of the scoped bridge. Older accepted Unord
   panic, Span consuming-callback/context/geometry/hygiene, and
   CachingSourceMapView base/context/metadata manifests bind the previous
   `rustvec/collect.go` bytes. The future bridge adds two private optional fields
   there. Those accepted manifests and fixture bytes remain untouched here;
   Root must independently accept and regenerate the necessary exact source-hash
   companions through their actual producers and complete their relevant gates.
   They must not be widened to include unrelated future files.
3. Root's staging, progress-ledger decision, commit, and continuous progress
   push. This inventory grants neither Unord completion nor dependent
   NodeId/alias completion before those reviews and integration gates close.
