# Public context and consuming session entry points

This companion closes explicit runtime/ownership routes in the complete pinned
`rustc_span/lib.rs` port. It grants no additional dependency source-file credit.
`native_remaining_context.rs` invokes the unchanged complete crate; it contains
no copied replacement library body.

The current-context methods in `lib_remaining_context.go` cover the three mark
site constructors, four edition predicates, `substitute_dummy` and
`SourceFile::count_lines`. Each delegates the actual existing operation in the
same source evaluation order. Interned span access with no scoped session now
raises the original static scoped-TLS failure through the primary context at
the access site. Private structural panic witnesses retain their testing path.
The old span fixture bytes remain unchanged; exact newtype-assertion and
scoped-TLS messages have explicit existing phase-category mappings.

The full consuming session primaries are:

- `SetSessionGlobalsThenConsuming`
- `WithSessionGlobalsConsuming`
- `CreateSessionGlobalsThenConsuming`
- `CreateSessionIfNotSetThenConsuming`
- `CreateDefaultSessionGlobalsThenOwned`
- `WithMetavarSpansOwned`

They take the actual `rustvec.Owned[E]` and a borrowed static call dictionary
which receives ownership of Self and the current context. Every owned capture
belongs in E. The call can Take/Move Self, return a moved owner, or deliberately
forget it. Ref-only prior APIs remain compiler-closure conveniences; they do
not model arbitrary manual consuming trait implementations.

A transparent owning envelope in `lib_session_fn_once.go` reuses the original
scoped-session guards and constructor algorithms. The envelope owns the actual
callback cell and destroys it if unused. It does not erase the callback behind
a trivial-drop function. `rustvec.ConsumeValue` supplies the established call
boundary: ordinary destruction precedes return, required cleanup uses the
current context, and a prepared return value is not destroyed if argument
cleanup panics. Full manual FnOnce strategies can consume the descriptor so
that this automatic destructor does not run. No constructor-time runtime is
used for later callbacks or destruction.

The retained matrix has 3,520 original processes in four independent debug and
overflow profiles, with 1,324 actual SIGABRT outcomes. It covers 13 session
branches: present/absent TLS, overwritten TLS, duplicate-symbol constructor
failure, both conditional-session branches, default edition and real metavar
mutation. Nine other public context routes use genuine expansion IDs,
interned spans and metadata-decoded lazy source lines, including malformed
width-zero and unreachable-width states through safe public decoder input.

Custom non-Copy FnOnce values retain a real boxed state cell. The normal call,
late `mem::forget(Self)`, and early `ManuallyDrop(Self)` strategies exercise
successful calls, panic before/after consumption, unused callbacks, Drop
failures, field cleanup and within-process cell identity. Actual Arc counts
prove prepared-return leakage and forgotten Self ownership. Constructors run
under runtime A; calls and Drop run under A, B, or nil, plus real inherited
unwinding and panic-hook processing. Drop reads the real scoped edition, and
post-call public operations prove restoration. Hook payload kind, text,
can-unwind, force-no-backtrace and selected native locations are retained;
locations are explicit per-operation source profiles, never invented defaults.
No source-path observer normalization is performed.

Abort records retain only events actually emitted before termination. There is
no fabricated post-abort state, cross-language address comparison, allocator
ABI or transparent timing claim for arbitrary raw-Go-local defers. Translated
panic sites use the current `Context.PanicAt`; raw Go callbacks remain the
explicit convenience boundary.

```sh
python3 tools/port-rust-span-lib/generate_remaining_context.py --check
go test ./port/rust/compiler/rustc_span/src -run '^TestLibOriginalRemainingContexts$'
```

All four original declarations from the separate 119-line `tests.rs` are now
in canonical `src/tests_test.go`. Their full bodies, 36 textual assertions and
56 executed comparisons are retained; no compatibility shim remains at the
old filename.

Fresh producers retain environment-specific SDK/registry filenames verbatim.
They do not rewrite historical observations to fit a new machine. CI and other
fresh checkouts should use a separate absolute output folder:

```sh
python3 tools/port-rust-span-lib/generate_remaining_context.py \
  --output-dir /tmp/ferrugo-remaining-producer
FERRUGO_LIB_REMAINING_PROOF=/tmp/ferrugo-remaining-producer \
  go test ./port/rust/compiler/rustc_span/src -run '^TestLibOriginalRemainingContexts$'
```

The producer writes `remaining-context-manifest.json`,
`lib-remaining-context.jsonl.gz`, and the matching raw `.jsonl`. A later
`--check --output-dir` regenerates all native observations and requires exact
agreement with that producer. Default historical generation refuses any raw
observation/filename drift and asks for a fresh output folder. Empty, file,
symlink and historical-overlapping output choices fail before native work.
No observer strips prefixes or normalizes a filename.

The Go parent validates both gzip/raw checksums and all selected source/replay
bindings before execution. An explicit empty, missing, nonregular or invalid
`FERRUGO_LIB_REMAINING_PROOF` choice fails without historical fallback. The
parent captures the exact manifest SHA and absolute explicit directory; every
abort child receives and validates those captured choices through
`FERRUGO_LIB_REMAINING_MANIFEST_SHA256`. Children never silently read a different
historical producer or an ambient later choice. Source locations remain the
selected producer's explicit profile, including its actual absolute paths.
