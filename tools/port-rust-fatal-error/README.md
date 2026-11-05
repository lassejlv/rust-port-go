# Pinned fatal-error runtime

The complete 41-line `compiler/rustc_span/src/fatal_error.rs` at Rust
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` maps to
`port/rust/compiler/rustc_span/src/fatal_error.go`. Its marker is the actual
`rustc_data_structures::FatalErrorMarker` unit declaration, mirrored in the
scoped `fatal_error_marker.go` prerequisite; this grants no full-file credit for
`rustc_data_structures/lib.rs`. There is no original fatal-error test module.

`FatalError` implements the source Clone, literal Debug/Display and Error-source
behavior. Its non-Send contract remains a caller obligation because Go has no
negative Send trait. `RaiseWithContext` resumes with the exact marker, and
`CatchFatalErrorsOwnedWithContext` catches only that marker and resumes every
other payload unchanged. Neither operation dispatches a panic hook.

`FatalFnOnce` supplies the consumed callback's complete Call/Close operations.
Nontrivial captured state and its entire destructor belong in that witness;
Call must not be reused. Generic results use `SpanOwned[T]`, including source
return-place ownership: an environment destructor panic after preparing a
result leaks that prepared result instead of destroying it. Plain Go callbacks
are conveniences for trivial captured destruction. A translated callback's
complete runtime witness uses scoped ordinary source-panic operations and
compiler-generated local destructor lowering. Component-owned cleanup guards
dispatch the selected cleanup-abort diagnostic once. Raw Go panic/defer
callbacks retain Go local-cleanup behavior and are not transparently Rust panic
operations.

Ambient unwind is distinct from a panic escaping the current callback frame.
An inner catch may recover a new panic during ambient unwind. A second panic
escaping required local cleanup aborts. Processing-panic context aborts at the
primary `Context.Panic`/Raise operation before callback locals unwind. Current
caller tracing runtime metadata is borrowed through operation contexts; owning
returned values do not capture a construction-time Scope. Later contextual Drop
operations take their later caller context explicitly.

`generate.py` links the actual entire original module through the unchanged
full compiler graph from `tools/port-rust-span-reference/prepare.py`, using the
matching nightly 2026-11-04 compiler. Its 36 isolated cases include normal,
inherited-caught, inherited-escaping and processing contexts; 20 terminate.
All source hook events are retained, including Rust's compiler cleanup-abort
diagnostic hook. Scoped ordinary operations and actual component cleanup
guards compute these from locally escaping unwind state, never from request
masks or expected traces. The outer
runtime witness owns inherited-escape cleanup. Native SIGABRT and Go exit134
are compared as the abort class; signal-number ABI identity is not claimed.

Another 144 cases compare literal Debug/Display, all writer C/S calls, writer
failure and continued same-Formatter use across nine option sets. These include
pretty/radix/width/precision/sign/zero/fill options and NUL fill. Clone, Error
text and `source() == None` are also checked.

```sh
python3 tools/port-rust-fatal-error/generate.py --check
GOCACHE=/tmp/ferrugo-go123-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test ./port/rust/compiler/rustc_span/src -run '^TestFatalErrorNative'
```

Fixture and source identities are in `src/testdata/fatal_error_unwind_manifest.json`.
This is a component-local unwind/ownership adapter, not a process-global Go
panic interceptor or a port of the Rust standard panic runtime.

The generator always reruns the verified whole-reference helper rather than
trusting the existence of a cached profile. `--rustc`/`FERRUGO_RUSTC` overrides
are checked against the exact SDK and preserve executable dispatch identity.
Unexpected native exit101/segmentation faults are rejected, not labeled abort.
Compile and child timeouts are finite. Panic locations are explicit profile
metadata and remain unlocated when the caller has not supplied that profile.
