# Retained original-source validation

The standalone runner compiles complete, unchanged pinned source and exact
checksum-verified registry files. It executes all 17 original lexer unit tests.
The existing algorithm oracle and scalar suites remain separate and unchanged.

| Contract | Native observations |
| --- | ---: |
| All seven lexer derived Debug types and all variants | 477,032 |
| All literal derived Debug types and used NonZero primitive wrappers | 35,658 |
| Three structured lexer Ord/PartialOrd implementations | 2,581 |
| All seven literal callbacks, all eight check_for_errors modes | 4,032 |
| Owned lexer predicate | 224 |
| Owned lazy iterator ForEach/Fold and accumulator cleanup | 672 |
| Independent checked/wrapping × debug/nodebug cursor, iterator and u32 arithmetic | 15,480 |
| Callback panic processing in the native panic hook | 36 |
| Native default fold Next-panic cleanup | 3 |
| Internal failure sites in normal, ambient-unwind and panic-processing state | 42 |

Debug observations cover every source variant, relevant zero/min/max/Option
boundaries, 19 patterns, ordinary and independent character writers, every
native writer failure cut, caller error identity, post-error fill/alignment
state and reuse of that same formatter. Failure cuts come from the native
successful operation counts. They are not derived from Go output.

Ownership scripts include initially empty input, absent error callbacks,
Unicode/NUL/raw/C-string behavior, escaped continuations and warning recovery,
partial callback consumption, return ordering, actual borrowed environment
cell identity, borrowed closure lifetime, Call and Drop panics, field glue,
ambient outer unwinding with an inner catch, final accumulator return-place
leaks and isolated unrecoverable cleanup. Literal scripts include 524 native
abort cases; predicate scripts 48; iterator-consumer scripts 162. All 36 panic
hook cases abort immediately. The three Next-failure probes include two aborts.
The 42 internal-site cases include 14 immediate aborts. Native `eat_until` and
byte advance fail at a UTF-8 continuation boundary; public guarded/raw entry
points exercise assertions and unwrap. Exact native checked u32 operations and
bounded Go private-site injection cover inherited ForEach/Fold and standalone
Next/Nth/Count/Last failure timing. Processing failures run no accumulator,
environment or scope destruction; ordinary/ambient failures retain source
cleanup. Wrapping arithmetic and disabled debug assertions remain independent
of panic context. Context inheritance/restoration is also checked directly.

Supporting char Debug proof covers all 1,112,064 Unicode scalars in both writer
profiles, with 4,344 operation-hash blocks each. String Debug covers every
scalar in two contexts in both profiles, with 4,344 blocks each. Unicode 17 and
the exact compiler identity are checked. Native full-write scalar evidence
complements the per-write failure/reuse cases; no standard-library source-file
completion credit follows from these adapters.

Every fixture has compressed/raw hashes and manifests binding original Git
objects, complete registry dependencies, native clients, generator and Go
production/replay files. Go baseline tests verify these bindings without
runtime Rust or source-cache fallback. `--check` verifies native output and
preserves equivalent committed gzip bytes.

Final gates pass on actual Go 1.23.12: all neutral formatter, lexer and literal
tests, including source tests/benchmark workloads, plus vet. Go 1.27 race passes
the three complete target packages, including subprocess abort cases
(`GORACE=atexit_sleep_ms=0` avoids a process-exit delay). The bootstrap syntax
package passes with the extended production lexer.

The complete original-source `--check` passes with bare PATH Rust/Cargo and
repository-relative source/cache overrides. Those relative tool/source/cache
overrides also work from `/tmp`. Primitive generator checks pass under actual
Go 1.23 with bare PATH Rust, retaining the same Go 1.27-produced gzip bytes.
Nineteen rejected CLI/tool selections preserve every evidence file. Four Python
guard tests cover bare/relative/empty tool selection and expanded source
tampering (changed, extra, missing, symlink and corrupt archive cases).

The pre-existing live lexer oracle also passes under actual Go 1.23 and bare
PATH tools after these changes: 231 committed cases, 6,000 seeded UTF-8 cases,
1,024 each raw/guarded/frontmatter cases, and 693 hydrated Rust source files
(7,663,897 bytes) across upstream Rust, Cargo and this repository. It reruns all
17 unchanged source unit tests. This supplements the new lazy/configuration
contract evidence and preserves the earlier algorithm baseline.

The original literal generator also passes under Go 1.23 with bare PATH tools:
all nine unchanged Rust tests, 27 benchmark workloads, 121,536 algorithm cases
and all 1,112,064 scalars ×16 operations. The 302,482,110-byte scalar trace
retains SHA-256
`3d37900f0331e0071d790a7335984975df4b15fb62403d1355822213b1a26263`.
Its refreshed complete implementation record binds the new helper and native
contract evidence hashes. Repository inventory/ledger updates remain separate.

No broader compiler completion claim is made by this component report. Runtime
and large-input boundaries are documented in `PORT.md`.
