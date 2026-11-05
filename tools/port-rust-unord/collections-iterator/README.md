# Ordinary Unord collection iterator traits

The additive `ExtendScoped` and `UnordMap/SetFromIteratorScoped` entrypoints
implement ordinary source `Extend` and `FromIterator` on the real Fx-backed
collections. They accept the existing actual `rustvec.IntoIterator`. The old
`Extend` and `UnordMap/SetFromIterator` entrypoints retain their same-context
consumer convenience scope. No accepted engine or convenience body is changed.

This corpus calls the unchanged original ordinary Rust traits with a Producer
which implements `IntoIterator` and does not implement `Iterator`. Its owned
optional input, conversion method, destructor, returned iterator, values, Hash,
and Eq are observable. The 720 native cases include 296 SIGABRT prefixes,
normal/destructor-unwind/panic-hook contexts, and fresh/resumed panics.

An early conversion panic destroys Producer, its still-present input, and the
input's remaining values. A panic in Producer destruction after taking its
input leaks the already prepared returned iterator. Successful conversion
destroys Producer before hint/reservation/consumption. The Go producer uses a
genuine owned Self and returned scoped iterator, preserving those boundaries.

The input has independent `for_each` and `fold` implementations. Map selects
for_each; Set's real map-to-unit adapter selects fold. A legal input stores the
borrowed insertion consumer in a guard, invokes it during a caught nested
destructor unwind, and then resumes insertion. Hash/Eq/Drop receive that call's
current panic state. The frozen per-cell scoped-consumer bridge carries this
metadata; no global current-context registry or Next-only replacement is used.
The input collect override always panics and is never selected by these traits.

Both manifests bind the verified complete-reference loader inputs. The native
manifest is independent of the Go source bindings. This partial proof grants
no whole Unord, std, or hash-table source credit.

```sh
python3 tools/port-rust-unord/collections-iterator/generate.py --native-only --check
python3 tools/port-rust-unord/collections-iterator/generate.py --check
python3 tools/port-rust-unord/collections-iterator/test_tools.py
```
