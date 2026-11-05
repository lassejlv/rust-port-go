# Explicit panic operations

This is scoped runtime support for the source ports. It grants no Rust standard
library file or crate credit and installs no process-global Go panic hook.

`port/support/rustfmt/panic.go` exposes an ordinary source raise (`PanicAt`),
formatted raise (`PanicFormattedAt`), hook-free propagation (`ResumeUnwind`),
and proven hook-then-abort operations (`AbortPanicAt`, `AbortCleanup`). Recover
receives the exact original payload, without a tagged wrapper. Rust's dynamic
payload kind lives separately in `SourcePanicInfo`; a Go string alone does not
distinguish Rust `&str` from `String`.

The caller borrows `PanicOperations` through `Context.Runtime.Panic` for the
current operation. `WithPanicOperations` preserves the independent tracing
metadata in `Runtime.Value`. Copies retain the same borrowed runtime pointer;
`Context` remains comparable even when `Value` contains a slice. The helper does
not own or implicitly destroy the hook environment. A source adapter for an
owned hook must provide that environment's actual lifecycle separately.

Complete source raise sites supply their actual compiled-source location,
payload kind and hook flags. Tracked callers must be supplied explicitly when
the source operation uses them. No Go stack or automatic canonical Rust
filename is substituted. Plain `Context.Panic` is the ordinary opaque,
unlocated convenience. A nil observer does not claim to reproduce Rust's
default stderr/backtrace hook.

`PanicOperations.Locations` is an optional caller-owned immutable map from
component-owned semantic site IDs to actual compiled `SourceLocation` values.
`Context.SourcePanicLocation` returns nil for missing sites and preserves the
supplied spelling verbatim. A normal context does not select a logical source
path profile implicitly. Returned scalar copies cannot mutate the profile map.

Formatted panic payloads are rendered with processing-panic context before the
hook runs. This preserves `expect`/assert Debug and its local destruction. A
translated Debug panic in that phase aborts immediately, before the hook or
local destruction. The hook itself also receives processing-panic context.
Complete translated callbacks use that context's primary panic operations;
raw Go panic/defer callbacks retain Go-local cleanup timing before the helper's
escape guard aborts.

Ambient unwind is independent of a second panic escaping local cleanup: an
inner catch during ambient unwind can still recover. A true compiler-generated
cleanup-abort boundary owns its diagnostic hook. Neither the user Drop body nor
another enclosing guard should invent the same event. `CleanupAbort` is
explicit metadata for the selected compiler/std profile; nil does not invent a
diagnostic or location. `CleanupPanicInfoNightly20261104` records the actual
observed nightly 1.101 operation at core `panicking.rs:233:5`.

The retained authored Rust observation client uses the exact official nightly
`1.101.0-nightly (708c94ef0 2026-11-03)`, distributed as nightly-2026-11-04.
Its official archive checksums and compiler selection are retained by
`tools/port-rust-span-reference/toolchain.json`. No Rust source body is changed.
The unstable hook-flag accessors are enabled only in the observation client.

```sh
python3 tools/port-rust-span-reference/install.py --fetch
python3 tools/port-rust-panic-runtime/generate.py --check
GOTOOLCHAIN=local .toolchain/go/bin/go test ./port/support/rustfmt -run 'TestNativePanicOperationOracle|TestPanicMetadataBorrowAndResumeIdentity'
```

`--rustc` and `FERRUGO_RUSTC` select the compiler, with bare PATH names and
repository-relative paths supported. Explicit empty/invalid selections fail.
Executable symlink basenames are preserved for Rustup dispatch. Compilation
and children have timeouts; native core dumps are disabled. Only native exit0
or actual SIGABRT is accepted. Native SIGABRT and component-local Go exit134
are compared as the abort class, without claiming signal-number ABI identity.

The committed primitive fixture contains 45 independent cases, including 24 aborts.
The baseline Go test needs neither a Rust executable nor a network connection.
Its manifest binds the primitive implementation, test witness and native
prototype. Existing unrelated formatter fixtures remain unchanged.

`component_guards/` independently integrates this protocol with 182 real source
component scripts, including 107 native aborts. It covers owned generic
callbacks, fields/temporaries, forwarding guards and current caller metadata;
the primitive protocol and component witnesses are distinct retained proofs.
