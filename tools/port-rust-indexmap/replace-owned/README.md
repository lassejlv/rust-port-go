`rustvec.ReplaceOwnedWithContext` is a scoped support operation for a live
destination and an independent initialized replacement. It preserves the actual
destination cell and installs the consumed replacement plus its destruction
operations even if old Drop panics. An immediate processing-panic abort runs no
replacement cleanup. It does not claim every Rust assignment case or Rust ABI.

Run `python3 tools/port-rust-indexmap/replace-owned/generate.py --offline --check`.
The observer uses actual Rust assignment and the standard `Clone::clone_from`
provided body, not a rewritten algorithm. Seventy-two cases cover normal,
inherited and processing contexts, old body/field destructor failures (including
double cleanup panic), actual source hook metadata, same-cell observations and
A/B/no-observer ambient metadata. Native aborts require actual SIGABRT; Go's
corresponding operation exits134. Source locations retain the actual compiled
profile spellings. Missing profiles never manufacture source locations.

The helper also has supplemental Go protocol regressions for dynamic destructor
operation transfer, generation invalidation, current-scope validation and
independent replacement ownership. The original ownership.go is unchanged.
