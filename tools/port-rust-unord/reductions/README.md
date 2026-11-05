# UnordItems reductions

This proof calls the unchanged pinned Rust `UnordItems::{max,min,sum,product}`
against independently selected iterator and element implementations. It retains
2112 native cases, 234 SIGABRT prefixes, and four compiler bounds checks. The
corpus covers normal execution, destructor unwind, execution inside a panic
hook, and fresh or resumed panics.

The pinned SDK's default `Iterator::max/min` selects `I::reduce` with the
actual `Ord::max/min` method. It does not select `I::max_by/min_by`. Native
types with independent reduce, fold, max/min, and unused max_by/min_by trap
implementations prove that distinction. The ordinary reduce selects the first
item with next, then the actual fold; an overridden reduce need not call next.
The tested Ord implementation records its max/min call before calling cmp.

Default `Iterator::sum/product` selects the actual `Sum/Product` implementation.
That implementation owns an accumulator and selects the input's fold. Direct
iterator sum/product overrides instead call those traits with a real
`std::iter::Once` owning a value. A distinct overridden fold yields 98 and 99
without calling next. The Go witnesses retain these concrete paths and actual
owned iterator types; the production Unord entrypoints transfer ownership to
the selected max/min/sum/product implementation.

Every input, value, accumulator and comparison is observable. The native
failures distinguish a locally prepared result, which is destroyed if a
preceding explicit item destruction throws, from a prepared return, which
leaks if the accumulator or iterator parameter destruction subsequently throws.
Both destructor order and current panic state must match the original trace.

The two manifests independently retain native evidence and Go source bindings.
The shared complete-reference loader validates the compiler, crate graph, and
pinned source provenance before compilation. This scope proves the existing
Unord reduction methods and selected exercised trait paths; it grants no std
or whole Unord source credit.

```sh
python3 tools/port-rust-unord/reductions/generate.py --native-only --check
python3 tools/port-rust-unord/reductions/generate.py --check
python3 tools/port-rust-unord/reductions/test_tools.py
```
