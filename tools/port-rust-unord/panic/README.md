This validates the scoped collection/sorting guards against real Rust 1.101
standard-library operations. It does not claim any std source-file completion.
The authored driver calls actual `HashMap`, `Vec::from_iter`, `slice::sort_by`,
closure and destructor implementations; no library body is reconstructed.

The 312 independent native processes cover:

- fresh callback panics versus `resume_unwind`, with normal and inherited
  unwind state and calls from an actual panic hook;
- map hashing/equality, owned entry closures and incoming key/value cleanup;
- vector next/iterator destruction and remaining collected-element cleanup;
- sort callback forwarding and the intrinsic inconsistent-order panic;
- actual map/vector capacity-overflow raises, including their payload types;
- a second escaping destructor panic and the actual compiler-generated
  cleanup diagnostic, after the correct remaining field/element cleanup;
- hook return, hook panic, hook resume, and hook-local destructor panic.

There are 258 actual SIGABRT outcomes. Each retained record contains only real
stdout observed before exit; no postmortem state is fabricated. The Go primary
matches every hook invocation, exact original payload, panic state,
`can_unwind`/`force_no_backtrace`, source location spelling, destructor event,
catch result, and abort prefix. Go exit 134 represents the abort class rather
than a SIGABRT ABI. Ordinary callback sites use `PanicAt`; forwarding uses
`ResumeUnwind`, and only a proven escaping-cleanup guard uses `AbortCleanup`.

The selected SDK's actual source locations are input operation metadata for
the Go witnesses. Callback locations come from the authored client; intrinsic
capacity/order locations come from the real std implementation. Semantic site
IDs select those immutable profile locations. No filename normalization or Go
stack inference is used. The source argument is deliberately relative during
compilation so the client's locations reproduce across checkouts.

The manifest binds the exact original SDK libraries, driver, generator, shared
SDK selector, current guard/runtime sources and fixture. Existing valid gzip
bytes are retained. Run:

```
FERRUGO_GO=/path/to/go python3 tools/port-rust-unord/panic/generate.py --check
```

`FERRUGO_RUSTC_NIGHTLY` accepts a PATH name or path to the exact recorded SDK.
The runtime's nil-hook convenience and all older reference fixtures remain
unchanged. Raw Go panic callbacks retain Go-local defer timing and lack source
operation provenance; complete translated sites supply `SourcePanicInfo`.
