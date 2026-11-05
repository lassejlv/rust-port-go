# Owned source IntoIterator for ordinary Bag extension

`UnordBagExtendIntoIterator<S,T>` is an additive ordinary Extend primary. It
owns the actual source S and selects the real `Vec::extend::<S>` implementation
with the existing static specialization dictionary. S can move, take, or
deliberately forget itself during conversion. The source parameter destructor
runs if S remains owned. The accepted IndexIntoIterator-based Bag Extend keeps
its convenience scope; no Bag or Index storage engine body is changed.

The unchanged original Rust Bag Extend is exercised in 768 native cases with
184 SIGABRT prefixes. A genuine Producer implements IntoIterator without
implementing Iterator. It returns either an ordinary custom Input or the actual
Vec::IntoIter specialization. Its optional input and destructor are observable.
Cases cover empty/nonempty destination, empty/one/multiple input values,
source next/input/element/producer failures, early conversion failure, legal
forget(Self), normal execution, destructor unwind, panic-hook execution, and
fresh/resumed panics.

An early conversion failure destroys the producer and its present input. Once
the input has been taken, producer Drop failure leaks the already prepared
returned iterator. Forgetting Self suppresses producer Drop while preserving
the returned input and its later destruction. The generic vector strategy
selects the actual Input next/hint; Vec::IntoIter extension bypasses both.

The independent allocation driver records twenty additional native cases.
It enables its actual global allocator only around extension, with no output
or unrelated allocation during observation. Value is a real inhabited Drop
type with size/alignment 16/8. All nonempty vector allocations have at least
four slots, so alignment 8 and size at least 64 isolate those requests. The
driver derives capacity from the observed allocation, known initial allocation,
or exact input allocation reuse. An ordinary legal Bag::items().all closure
records the result address; no vector header or private Rust field is read.
Input/result identity is compared within the native process and retained as a
boolean. Actual IndexVec capacity and within-Go identity must match these
source observations. This makes no claim about Go allocator requests or Rust
ABI compatibility.

The verified complete-reference loader is used before compiling either driver.
Both manifests bind its inputs; the native manifest remains independent of Go
source bindings. This is partial Unord evidence and grants no std, Index, or
whole Unord source credit.

```sh
python3 tools/port-rust-unord/bag-iterator/generate.py --native-only --check
python3 tools/port-rust-unord/bag-iterator/generate.py --check
python3 tools/port-rust-unord/bag-iterator/test_tools.py
```
