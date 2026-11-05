This is a scoped supporting dependency for the still-pending `unord.rs` port.
It grants no standard-library source-file completion credit. Production code
uses only Go and the already-validated ownership/collector support; Rust runs
only in this development reference tool.

`rustsort.Cached` implements the normal Rust 1.101 `sort_by_cached_key` path:
owned `FnMut(&T)->K`, exact trusted collection, the source's conditional
u32/usize index choice, actual unstable tuple sorting, stable value permutation,
and temporary-key destruction. The function owns one actual mutable environment
cell. Calls and complete environment/field destruction observe that same cell.
The returned key owner must use the same complete destructor as `CachedKeyOps`.
Source borrows are temporary and must not escape or overlap incompatible
calls, moves or destruction. Hidden destructible Go captures are forbidden.

The tuple's less-than operator calls `K::__chaining_less`, using its independent
override when present. The actual default invokes `partial_cmp` once, continues
only on equality, and returns false for an unordered result. It does not call
the key's `Ord::cmp` or `PartialOrd::lt`. Index comparison follows only when the
key requests continuation. This is distinct from uncached `sort_by_key`.

The native observer calls the unchanged original SDK implementation. Its real
custom key types and custom `FnMut` have independently observable trait methods,
actual captures, moved tokens, destructors, interior mutation and failures.
The `FnOnce` implementation deliberately panics if incorrectly selected. All
5,768 retained normal cases match, including 121,352 function calls, 5,768
function destructions, 81,600 key destructions and 121,472 comparisons, split
equally between default partial comparison and explicit chaining overrides.
Neither key `lt`, `cmp`, nor equality is spuriously called.

Ninety-nine additional isolated native processes cover normal, inherited
unwind and panic-hook contexts. Thirty-six actually abort, with recorded
SIGABRT status and only the stdout prefix observed before termination. There
is no fabricated post-abort state. The Go primary uses explicit `Context`
metadata and `Context.Panic`, and reproduces those prefixes and the completed
results; exit 134 represents the abort class, not a SIGABRT ABI. The explicit
context is a caller/runtime witness, not discovery of Go panic state or an
implementation of Rust's global panic-hook runtime. Callback-local raw Go
panic/defer timing has the same limitation documented by the other ownership
adapters; translated panic sites must use `Context.PanicAt`.

An additional 198 original-SDK processes cover ordinary panic hooks, resumed
payloads, and complete function-field and element cleanup. Seventy-two actually
abort. The retained hook records include payload kind, the original source
location, `can_unwind`, `force_no_backtrace`, and each observed destruction
before termination. A resumed payload invokes no fresh hook; recovered payloads
use `ResumeUnwind`. The exact compiler cleanup boundary invokes
`AbortCleanup`, including a token-field panic while its function body is
already unwinding. Cleanup cannot continue into the outer key vector after
that abort. Caller-provided immutable source-location metadata retains the
compiled reference's spelling; it is not inferred from a Go stack.

Empty/singleton inputs still destroy the uncalled function. For larger inputs
the collected iterator destroys the function before sorting. A throwing
function destructor destroys collected keys during unwind; key-function and
comparison failures preserve the correct cleanup order. A throwing key
destructor after permutation leaves the caller's values permuted and finishes
the remaining keys' destruction. The context corpus checks field cleanup and
double-panic aborts, including a moved token held by the caller.

Run `python3 tools/port-rust-unord/cached/generate.py --check`. Environment
selection uses the frozen sort tool's PATH/local resolution and explicit
override validation. The manifest binds the exact SDK libraries, source audit,
observer, generators, helper sources and fixtures. Existing valid gzip bytes
are retained; check mode rewrites neither results nor machine-version evidence.
Run `python3 tools/port-rust-unord/cached/generate_hooks.py --check` for the
independent hook/resume/cleanup companion.
