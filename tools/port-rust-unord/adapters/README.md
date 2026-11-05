# UnordItems adapter dispatch and ownership

This proof invokes the unchanged pinned Rust `UnordItems::{filter,filter_map,
flat_map,chain}`. Its 1,650 native cases include 442 SIGABRT prefixes across
normal execution, destructor unwind, panic-hook execution, and fresh/resumed
panics. The comparisons include value, input, function environment, and captured
token destruction; failures in each of those operations; early `get_only`
consumption; empty inputs; and independently overridden iterator methods.

The actual defaults select different methods. Filter and FilterMap invoke the
predicate through `FnMut`. Their count calls select the original input's `fold`,
and destroy its owned mapping function before destroying the input. GetOnly
instead drops the adapter's input before its predicate. FlatMap's internal
`move |x| f(x).0` wrapper calls the original `f` through shared `Fn`, even when
the wrapper is invoked through `FnMut`. Its count calls each inner input's
independent `count`, while GetOnly uses inner `next` and drops exhausted inner
inputs. Chain count independently selects left and right `count`; Chain next
clears an exhausted left input before advancing the right input.

Three custom iterator implementations override the actual filter, filter_map,
and chain constructors. They distinguish `FnOnce` from `FnMut` and shared `Fn`,
then fail before returning their declared concrete adapter type. Their result
method dictionaries are never invoked. Nine original compiler checks retain the
positive bounds, the source's shared `Fn` requirement, matching chain item types,
and the hidden Clone bound on all four opaque return types.

The Go test supplies concrete selected iterator witnesses for the source
operations exercised here. Count consumes a newly constructed adapter; GetOnly
consumes a different adapter and verifies destruction of its partially consumed
state. The witnesses do not claim a complete standard iterator implementation.
The production Unord bodies transfer the actual owned input and function to the
selected typed constructor and preserve that result's real method dictionary.

The producer uses the mandatory verified compiler graph loader.
`native_manifest.json` binds native evidence independently; `manifest.json`
additionally binds the Go source and test. This proof grants no standard-library
or whole Unord file credit.

```sh
python3 tools/port-rust-unord/adapters/generate.py --native-only --check
python3 tools/port-rust-unord/adapters/generate.py --check
python3 tools/port-rust-unord/adapters/test_tools.py
```
