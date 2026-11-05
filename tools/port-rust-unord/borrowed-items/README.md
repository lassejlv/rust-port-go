# Borrowed UnordItems adapters

This proof calls the unchanged pinned Rust `UnordItems::{cloned,copied}` on
`UnordBag::items`, `UnordSet::items`, and `UnordMap::keys`. The additive Go
entrypoints accept the ordinary `*T` source-reference representation used by
those real collections. They transfer the same actual iterator state to the
selected concrete constructor; they do not build another collection or invoke
`Clone` while constructing an adapter. The source return type hides `Clone`.

The native corpus contains 720 cases and 74 SIGABRT prefixes across normal
execution, destructor unwind, panic-hook execution, and fresh/resumed panics.
It records `Clone`, original and cloned value destruction, early `get_only`
consumption, and the complete underlying collection destruction. Copy values
have an observable `Clone` implementation which must never be called.

Two independent input implementations distinguish count dispatch. Their
`next` panics, `fold` calls its supplied callback twice, and `count` returns
777. Actual `Cloned::count` selects the fold override, clones and drops each
value, and returns 2. Actual `Copied::count` selects the count override and
returns 777. The Go witnesses implement these concrete selected methods
separately. The three ordinary collection-iterator folds use their actual
standard traversal; that traversal is not substituted for an overridden fold.

Five retained compiler checks prove the positive bounds, missing Clone/Copy
bounds, and the hidden Clone bound of both opaque returned adapters. Clone
values have the actual Rust size/alignment 16/8 and Copy values 8/8, explicitly
selected in the Go hash/vector configurations. These are source layout inputs,
not a claim that Go object layout or allocation matches the Rust ABI.

`generate.py` loads the verified complete graph through the shared reference
loader and binds that loader's inputs. `native_manifest.json` records native
evidence independently; `manifest.json` additionally binds the Go source and
test witnesses. Both describe this partial module proof only; they grant no
whole Unord, std, or hash-table source credit.

```sh
python3 tools/port-rust-unord/borrowed-items/generate.py --native-only --check
python3 tools/port-rust-unord/borrowed-items/generate.py --check
python3 tools/port-rust-unord/borrowed-items/test_tools.py
```
