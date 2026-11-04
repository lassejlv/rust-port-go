# Validation boundary

The scoped backing is validated; the entire Unord source file is **pending**.
There is no std/hashbrown/rustc-hash source-file credit for these dependencies.
The partial real UnordMap foundation is explicitly marked partial in its source
and separate `../map/manifest.json`.

Completed gates in this workspace:

- Actual Rust1.99.1 and1.101-nightly regenerate every table proof independently.
  `check.py` validates retained expected records without rewriting manifests.
- Actual complete original `rustc_data_structures` linked from the shared full
  compiler graph reproduces2,000 map/read-items/filter/map/collect operations.
  All43 original span tests were passed by that shared graph's preparation.
- Actual minimum Go1.23.12 replays all expected records. Race validation passes
  for scoped backing79.890s and data-structures original tests6.169s; vet passes.
- Previous semver4,142 collector records and30 real abort-prefix fixtures still
  replay unchanged after adding independent fold/for_each constructor dispatch.
- Five isolated tool tests cover empty/invalid executable overrides, bare and
  path executable resolution, changed archives, and source archive symlinks.

The fixtures bind their original driver and generator bytes, compressed/raw
SHA256 values and coverage counts. Map state proof is retained as1,500 SHA256
blocks over96,000 complete original state/output lines; remaining proofs retain
their complete original records. Intentional aborts retain actual event prefixes
and SIGABRT only. No after-abort state is inferred.

Source SDK audit identities are in `STD-SOURCES.json`. The SDK copies and exact
registry archives were read unchanged. `cfg(miri)` in the scalar-only dependency
probe selects original generic-group code on the native host; no source body is
rewritten and no Miri interpreter run is claimed. Original source warnings are
left intact when the reference compiler diagnoses renamed internal lints.

Validated execution is the recorded64-bit little-endian native domain. The
source-size and pointer/group configuration adapters preserve the translated
arithmetic, but other host execution domains are not claimed by these fixtures.
Memory addresses/ABI and custom allocator APIs are outside this supporting
dependency boundary. Source borrow, ownership and truthful trait dictionaries
remain mandatory; hidden Go-capture destruction is not invented.

Remaining Unord work includes the general owned iterator/closure algebra and
its default/override dispatch, stable/unstable/cached-key sort callback behavior,
generic collection conversions, deterministic encoder ContextOrd ownership,
decoding, and order-independent stable hashing. Those operations are not exposed
as placeholder methods or treated as complete by this milestone.
