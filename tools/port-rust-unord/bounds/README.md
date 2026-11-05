# Complete Unord source bounds and concrete Self

The additive `unord_bound_primaries.go` family represents every required source
bound separately from the operation the selected iterator body actually uses.
All dictionaries are borrowed static metadata describing the same lawful source
types as the selected operation. They add no owned closure or destructor. Unused
methods are never invoked, probed, or nil-validated.

| Source bound | Complete borrowed witness | Primary family |
| --- | --- | --- |
| Bag derive Default, unord.rs683 | `UnordDefaultValue[T].Default` returns actual owned T | `DefaultUnordBagWithBounds` forwards empty New; Default is unused |
| Items derive Clone, unord.rs33 | Actual I::clone slot plus `UnordCloneValue[T].Clone` | `CloneUnordItemsWithBounds` / `CloneFromUnordItemsWithBounds`; extra T::clone is unused |
| max/min T:Ord, unord.rs79–92 | Required eq, partial_cmp, cmp and Eq marker laws | `Max/MinUnordItemsWithBounds`; selected I max/min/reduce/fold remains independent |
| cloned/copied, unord.rs145–156 | Clone method; Copy marker plus Clone supertrait | Both `rustvec.Ref[T]` and ordinary `*T` `...WithBounds` primaries |
| sum/product R:Sum<T>/Product<T>, unord.rs95–107 | Required actual R::sum/product monomorphized at concrete S:Iterator<Item=T> | `Sum/ProductUnordItemsWithBounds`; independent I override remains selected |
| ToStableHashKey::KeyType:Ord+Sized+StableHash, stable_hash.rs112–114 | Required eq, partial_cmp, cmp, StableHash and actual finite source layout (including ZST) | `UnordStableKeyBounds` and all seven key-sort `...WithBounds` entrypoints |

Rust generic trait methods are represented by monomorphized operation
witnesses. For Sum/Product, static coherence between the required R trait
implementation and every invocation inside the selected I override remains a
lawful source-type obligation. The same applies to the independently selected
key `lt`, cached partial_cmp/chaining overrides, and complete key bound. The
used slots remain unchanged; unused Eq/Ord::cmp/StableHash do not replace them.
No unrelated standard trait default-method APIs are ported here.

The legacy selected-operation conveniences remain intact with those same
source type obligations on their callers. The existing complete borrowed Bag
Hash+Eq+StableHash primary remains unchanged. The historical frozen 153+3 scope
and snapshot remain an independently identifiable baseline for this explicit
delta; they are not silently rewritten as a new acceptance claim.

Items clone_from additionally requires the same source Self. Its new guard
checks the actual instantiated private cell[S,T] type before source cloning or
old destination Drop; Go aliases of S pass, distinct named/embedding S reject.
It checks destination Clone exposure as well: opaque returned adapters do not
expose Clone. The existing clone/replace/DropAndReplace body is unchanged.

The native producer uses the mandatory verified complete graph. Eighteen
compiler programs retain positive admission, new<NoDefault>, missing Default,
wrong clone_from Self, opaque Clone exposure, missing T/I Clone, Copy, Ord,
Sum/Product, and associated key Eq/PartialOrd/Ord/StableHash/Sized negatives.
Twenty-two original effect programs exercise the complete family and all seven
key-sort methods. Unused methods panic if called. Actual lt versus partial_cmp
and iterator overrides remain observable. The empty Default allocates nothing;
its first push makes one actual 32-byte allocation, observed without IO while
enabled. The Go probe checks source Vec capacity, without claiming Go allocator
or Rust ABI identity.

Go replay also covers both borrowed reference representations, same-I aliases,
and rejection of wrong-I or opaque destinations before clone/drop effects.
The old retained native ownership/sorting fixtures remain unchanged; they are
rebound only through their actual producers to the approved guard/helper delta.
Final Go 1.23 ordinary replay, all nineteen future Unord groups under race
(55.588s), scoped vet and the actual native-plus-Go live producer check pass.
All thirty-five future proof tables and thirty-four integrity checks pass.
Whole-file credit remains pending independent re-review and Root integration.

```sh
python3 tools/port-rust-unord/bounds/generate.py --native-only --check
python3 tools/port-rust-unord/bounds/generate.py --check
python3 tools/port-rust-unord/bounds/test_tools.py
```
