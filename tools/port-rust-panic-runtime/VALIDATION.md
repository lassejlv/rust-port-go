# Retained validation

The 45 native subprocess cases compare ordinary static/owned/opaque payloads,
opaque original-cell identity, resumed payloads, formatting before the hook,
normal/inherited-caught/inherited-escaping contexts, hook recursion, hook
resume, hook-local Drop failure and genuine compiler cleanup-abort hooks.
All 24 native aborts are actual SIGABRT; unexpected process failures are errors.

The cleanup diagnostic is independently observed after the ordinary body hook
and failing Drop hook. It carries the static string `panic in a destructor
during cleanup`, `can_unwind=false`, `force_no_backtrace=true`, and the exact
selected std location. The user hook's local Drop runs before that final abort.
An Error.Debug panic while formatting an expect payload instead aborts before
the Debug local drops or any user hook runs.

The Go replay uses the real helper plus complete authored local-cleanup
witnesses. Request masks choose source client operations; they never supply
expected hook events. Forwarding uses `ResumeUnwind`; the actual source cleanup
boundary emits `AbortCleanup`. Payloads are never wrapped by the helper.

Independent `component_guards/` integration now matches 182 original component
scripts, including 107 native aborts, across lexer/literal callbacks,
fingerprints, binary search, intervals and the stable-hash callback bridge.
These directly exercise the real migrated library guards rather than only the
primitive helper. Their actual Go 1.23 race checks and vet pass.
The parent independently accepted the frozen primitive protocol and component
integration from an isolated snapshot, including fresh native regeneration.

Verified in the execution environment:

- Actual Go 1.23.12 full `rustfmt` tests: pass, 5.195 seconds.
- Actual Go 1.23.12 `go vet ./port/support/rustfmt`: pass.
- Go 1.27 race on the native-operation replay and metadata/identity regression:
  pass, 22.490 seconds.
- Every baseline formatter test continues to pass; no existing fixture bytes
  were replaced by this addition.
- Empty, missing and wrong-compiler explicit selections fail before changing
  the fixture. Bare PATH compiler selection passes live `--check`.
- Borrowed runtime metadata preserves independent tracing payloads, source
  location profiles preserve exact supplied spellings, and absent profiles
  remain unlocated.

This bounded helper proof is not a port of global hook installation, default
stderr/backtrace rendering, Rust signal ABI or the standard panic runtime.
Source components remain responsible for their exact source locations and
their own compiler-generated local cleanup witnesses.
