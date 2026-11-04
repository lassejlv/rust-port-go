# Typed-index validation

Validation uses actual Go 1.23.12 and pinned
`rustc 1.99.1 (58cf8f9c3 2026-10-13)` on a 64-bit Linux host.

The complete unmodified upstream `idx.rs` produces **32,644 records** with no
Go differences:

| Source feature | usize overflow checks | Records |
| --- | --- | ---: |
| Standard | Disabled | 6,997 |
| Standard | Enabled | 6,997 |
| Nightly | Disabled | 9,325 |
| Nightly | Enabled | 9,325 |

Each record compares its result or panic class, the mutable key after the
operation (including mutation before a panic), and the complete ordered traces
of `new`, `index`, plus/increment overrides, and a custom conversion callback.
Cases include:

- actual usize/u32 implementations and four independent custom method profiles;
- zero, small values, u32 boundaries, high-bit values, and usize maximum;
- default checked arithmetic, unchecked wrapping, and u32 constructor assertions;
- independent plus/increment overrides and default increment dispatch to plus;
- injected callback panics at the first three calls of all five callback kinds;
- every standard and feature-gated core range conversion, with ordered,
  reversed, empty, open, inclusive, and maximum endpoints;
- a custom `IntoSliceIdx` implementation and slice-backed non-comparable keys;
- actual exhausted Rust inclusive ranges, capturing pre-conversion iterator
  endpoints and checking source reconstruction resets exhaustion.

Default Go tests match the 13,994 standard-profile records. Tests with
`ferrugo_rustc_nightly` match all 32,644 records. Both unit-test configurations,
both vet configurations, and deterministic full oracle `-check` regeneration
pass. The pinned source defines no original tests, so none are invented or
counted as ported upstream tests.

The oracle includes the full original file directly and requires no dependency
shims. Panic spelling differs across Rust and Go; panic classes, mutation and
callback order are compared. No slicing or standard-range iteration
implementation is claimed by these tests or adapters.
