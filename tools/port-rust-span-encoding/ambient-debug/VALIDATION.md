# Closed production review finding

The independent source/API review found one production mismatch beyond the
large frozen corpus: primary Debug helpers could unwind an internal failure
and release the interner lock before consulting the formatter's processing-
panic context. The unchanged Rust source aborts at the failure while the lock
is still held. The fix threads the explicit context to actual internal
missing-session, bounds, arithmetic/assertion and constructor failure sites;
normal calls retain ordinary behavior. One-shot mapper paths share those sites.
Runtime context and build profile are snapshotted before caller writes.

The rest of the audit found no omitted source APIs or behavior: packing prefix
priority and fields, constructor/query/mutation fast paths, tracking/reentry,
encoded Eq/typed Hash versus location-only OrdSpan equality/order/negative Hash,
Clone/default replacement, all four profiles, nil-session failures, canonical
Option/interner keys, owned capture-cell mutation/destruction, temporary borrows,
moved fields and caller-owned complete Drop witnesses match the source boundary.
Surrounding identity/session/hygiene/RawSpan files remain explicitly partial.

Completed validation after the fix:

- **1,344** separate actual native Debug cases match Go across all four build
  profiles, three actual ambient contexts, Span/OrdSpan/SpanData, returned first-
  write errors, sink dispatch, tracking and panic/cleanup ordering.
- **140** native processing-panic failures really SIGABRT and skip caller scope
  destruction. **48** actual original interner-lock observations at SIGABRT are
  held. The Go private structural panic witness independently observes the
  mutex at the exact raise site before defer unlock; no mutable/global hook or
  fabricated Go post-abort state is used.
- Actual **Go1.23.12** full span package **race passed (64.510s)** and **vet
  passed**, including the large native replay, new ambient replay and site-
  ordering tests.
- Portable bare-PATH **Go1.27.3** live companion **-check passed**, then the
  unchanged large **15,264-request** original-source **-check passed**. Both
  verified every native build profile and ran the actual Go replayers.
- The matrix-control test passes. The original14 source/registry/driver
  integrity regressions and source verification remain unchanged in the parent.

The companion raw SHA-256 is
`df369dbac36fb284e1785bc3a3b66fd2169e6ba45b0cb11997769ae9e2da4d66`;
preserved gzip SHA-256 is
`4a6a8bffcf4ea6fef7cbc77e928010ca7149f679143a44eff520a15735c22bd8`.
The main gzip remains unchanged at
`c1448d919c7496e7be2bc190480c32a60099a8cf3087ba3b73ea3e26492eb27b`.
Its frozen builder and manifest were not edited. The companion manifest binds
the parent manifest, exact original-source preparation, staged inputs, registry
sources, reference helper and pure-input case generator. Native lock audit data
is labeled reference-only and excluded from fabricated Go responses.

This closes an existing source-file review gap and adds no new source credit.
Production code uses only Go; the Linux64 signal audit is development-only.
