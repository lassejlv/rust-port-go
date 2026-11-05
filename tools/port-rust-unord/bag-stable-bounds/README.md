# Complete Bag StableHash bounds

The unchanged source implements `StableHash` for `UnordBag<V>` only when
`V: Hash + Eq + StableHash`. Hash and Eq are not called by the body. The additive
`UnordBagStableHash` primary requires a borrowed dictionary exposing all three
actual trait methods, then forwards only StableHash to the unchanged accepted
body. `UnordBagStableHashOps` adapts existing `rusthashmap.KeyOps` and
`UnordStableHash` shapes. The dictionary is static borrowed metadata and must
describe the same lawful source V; no extra owning closure or destructor is
introduced. No Hash/Eq call or runtime nil validation is inserted to enforce an
unused source bound. The existing accepted `bag.StableHash` convenience remains
unchanged and retains those source type obligations on its callers.

Four compiler programs test the actual source's positive bound and missing
Hash, Eq, and StableHash cases. The positive program has Hash/Eq implementations
which panic if invoked. It hashes actual 0/1/2/4-item source bags and retains the
StableHash effect trace and exact fingerprints. The Go effect probe compares
the same original output for both a direct complete method dictionary and the
adapter over existing trait shapes. No Hash/Eq call may occur.

Both manifests bind the mandatory verified complete-graph loader.
`native_manifest.json` binds the original program independently; `manifest.json`
additionally binds the new Go primary and replay plus accepted body sources.
This is an Unord source-bound proof, with no standard-library/HashMap or complete
Unord file credit. Go 1.23 direct and adapted dictionary effect comparisons, the complete future
Unord race gate, scoped vet, integrity checks and the actual producer live
`--check` all pass. Whole-file acceptance remains Root-owned.

```sh
python3 tools/port-rust-unord/bag-stable-bounds/generate.py --native-only --check
python3 tools/port-rust-unord/bag-stable-bounds/generate.py --check
python3 tools/port-rust-unord/bag-stable-bounds/test_tools.py
```
