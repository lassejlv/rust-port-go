# Complete cache panic-site observations

This supplements the existing caching algorithms, metadata and context corpora;
their generators, raw/gzip bytes and source-binding helper are untouched.

The source is the entire unchanged 170-line
`compiler/rustc_span/src/caching_source_map_view.rs` at
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`, Git blob
`3a3f238a7e55b52ecd812d7315aba6a199e2ef39`, SHA256
`1859b2fe6376229d948ff2116ec91e15cb3ad91fedba6d1d08222c57f10d15c0`.
There is no original child test module. The client calls the real exported
CachingSourceMapView in the verified complete original rustc_span graph, using
matching nightly 2026-11-04. No library source is edited or replaced.

`generate.py` always calls the shared verified native-reference loader, including
Git/archive/compiler/artifact validation; cached profile commands alone are not
trusted. Each of four independent debug/overflow profiles runs 81 scripts:
nine public safe setup/operation cases, normal/inherited-caught/actual panic-hook
processing contexts and construction under A followed by operation A/B/nil.
All 324 scripts retain 364 complete hook observations and 84 actual SIGABRTs.
Native SIGABRT and Go exit134 are the explicit host abort representation.
Compile/child timeouts are finite, unexpected exit statuses are rejected, and
terminal host abort stderr is not compared.

The observer retains payload kind/bytes, verbatim file/line/column, panicking,
can_unwind and force_no_backtrace. Local destruction, real Arc owner counts,
successful point/span results and state after catching failures are retained.
Locations are explicitly supplied by the selected producer profile; the Go
implementation infers no filename. A separate Go check confirms missing profile
locations remain nil. Nil-context operations deliberately have no observer or
runtime metadata: no construction-time A hook may be captured. The native outer
runtime's processing hook still runs the requested operation, but has no user
observer in the nil case. The Go complete outer-runtime witness explicitly
enters processing context and resumes the outer payload after successful work;
it installs no global Go panic hook. Preparation/priming under A remains visible.

| Original failure | Evidence |
| --- | --- |
| constructor `files[0]`, 34:43 | empty real SourceMap; owned-string bounds payload identifies the caching caller, while its files guard remains live |
| `lookup_line(pos).unwrap()`, 46:53 | imported nonempty files with an empty line vector or first line after the query; static-string payload |
| low upper-bound assertion, 133:9 | second file starts at1; sorted lines `[0,u32::MAX]` wrap the line end when overflow checks are off |
| high upper-bound assertion, 135:9 | second file starts at1; sorted lines `[0,5,u32::MAX]` wrap the high line end |
| high lower-bound assertion, 134:9 | a caught lookup failure installs file B but retains A's cached bounds; a later span uses the original stale state |
| `file.contains(lo)` assertion, 136:9 | same real post-catch state, with B's line start `u32::MAX` |
| lib absolute/relative arithmetic, 2427:27 / 2432:35 | the same safe cases with overflow checks on raise at the actual lib operator instead of reaching the unchecked assertion |

The stale-state control case also records Rust returning file B through the old
cached bounds after catching the lookup failure. The port preserves that source
behavior; this change only restores panic dispatch. `BytePos - BytePos` delegates
to the actual lib operation (2747:1); the point method's raw `u32` subtraction is
a separate caching site (77).

The following branches are preserved but are not claimed as publicly reached:

- The low lower-bound assertion (132) follows either a numeric cache hit or
  lookup bounds whose start is at/before the queried position.
- The final `file.contains(hi)` assertion (137) is already checked by file
  selection; if stale state retains the wrong file, `contains(lo)` fails first.
- The raw point-column subtraction cannot underflow after a numeric cache hit
  or successful bounds lookup. The span subtractions follow their hit tests or
  explicit lower-bound assertions.
- `line_index + 1` (48) cannot overflow: it follows a successful lookup into a
  real Vec of nonzero-sized RelativeBytePos values, so the index is below len.
- The later file index (151) remains within a monotonically growing map after
  lookup; valid immutable files guards cannot shrink the list between reads.
- Go use-after-Close checks are outside Rust's valid borrow/ownership domain.

No unsafe/private cache mutation is used to fabricate these branches. Tests use
the real source map, imported SourceFile metadata and line locks; the large
values are scalar metadata and never request a huge allocation. This adds no
Rust standard-library or supporting source-file credit.

```sh
python3 tools/port-rust-caching-source-map-view/panic-sites/generate.py --check
go test ./port/rust/compiler/rustc_span/src -run 'TestCachingSourceMap(FullPanicNativeOracle|PanicLocationRequiresProfile)$'
```
