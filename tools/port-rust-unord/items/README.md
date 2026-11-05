# UnordItems ownership and dispatch proof (work in progress)

These are supporting native proofs for the pending whole `unord.rs` port. They
provide no additional std, Iterator, Fn, or collection source-file credit.
The original `unord.rs` is compiled unchanged in the complete source-matching
compiler graph prepared by `tools/port-rust-span-reference`. The Fn-only driver
uses the same exact 1.101 SDK without the compiler graph.

Current retained observations:

| Driver | Actual processes | SIGABRT prefixes | Contract observed |
| --- | ---: | ---: | --- |
| `generate_function.py` | 147 | 74 | Independent Fn/FnMut/FnOnce, actual mutated environment and argument cells, clone, parameter destruction, prepared-result leaks, complete field glue |
| `generate_core.py` | 126 | 40 | `new`, `empty`, derived Clone/default CloneFrom, selected count override, non-fused `get_only`, failures and cleanup |
| `generate_boolean.py` | 1224 | 332 | `all`/`any` override versus default/try_fold dispatch, short-circuiting, real FnMut state, field/parameter drop ordering, owned try_fold check closure early Drop/forget/FnOnce |
| `generate_constructor.py` | 408 | 180 | Actual default std Map with `get_only` and count, distinct fold versus field drop order, overridden Iterator::map/flat_map calling FnOnce, shared underlying flat-map Fn and prepared inner-iterator leaks |
| `generate_collections.py` | 2016 | 374 | Actual map/set/bag owned and borrowed items plus map keys; all/any/count/get_only and remaining stored value destruction |

Each process uses normal, inherited-unwind, or panic-hook-processing context.
Fresh panics and resume-unwind are distinct. Exact native hooks, panic locations,
attempted operations and complete stdout prefixes are retained; the Go runner
uses subprocesses for abort cases. No post-abort state is invented.
`generate_core.py` also compiles the original Clone bounds: non-Clone elements
can be counted, but prevent derived UnordItems Clone even when the underlying
Empty iterator implements Clone. Opaque map/filter returns do not expose Clone.

The Go adapters use actual owned iterator/environment cells. Temporary shared
and mutable borrows obey their source trait's laws; dictionaries cannot retain
those borrows, a runtime scope, or hidden destructible captures. Function Drop
witnesses include all source automatic field destruction. Iterator all/any and
try_fold operations remain independently selected. The check(f) closure passed
to try_fold owns its actual predicate; a custom implementation may invoke,
destroy or forget that owned FnMut/FnOnce value. `DefaultTryFold` explicitly
witnesses the actual default rather than authorizing replacement of an override.

Go's lack of generic methods/impl-Trait return types is represented by free
functions carrying static monomorphized constructor/reduction dictionaries.
The dictionary retains the actual receiver/result types and selected source
trait implementation. It is not an arbitrary user callback or a Next-only
substitute. Constructor parameters are owned; returned iterators retain their
actual operation dictionaries. Opaque outputs do not gain Clone. The selected
From<UnordItems> operation differs from FromIterator.

The remaining adapter-specific, reduction, general collection and sorting
integration proofs are pending. The presence of a delegate signature is not a
claim that every possible external Iterator or collection implementation has
been ported. There is no production Rust fallback.

Run each generator with `--check` for a read-only live regeneration and Go replay.
`FERRUGO_GO` chooses Go explicitly; unset it to use the common verified selector.
`FERRUGO_SPAN_REFERENCE_PROFILE` may select the matching prepared graph. Invalid
explicit selections fail. Valid existing gzip bytes are preserved when the raw
native output is identical, allowing different Python/zlib compression versions.

Every whole-original UnordItems driver now uses the complete verified reference
loader; cached profile commands/artifact paths are never trusted. Loader source
inputs are bound in the per-driver manifests. The Fn-only driver uses the exact
SDK directly and binds its core/alloc/std libraries.
