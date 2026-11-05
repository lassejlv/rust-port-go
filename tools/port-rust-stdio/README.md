# Scoped Rust stderr runtime

This is supporting compatibility code for Cargo Graph and compiler SourceMap
printing. It grants **no Rust standard-library source-file credit**.

`port/support/ruststdio` implements the used `_eprint`/`write_fmt` operation:
one process-wide reentrant stderr lock, explicit source-thread identities and
leases, real shared capture buffers, temporary capture removal/restoration,
poisoning, exact Vec byte-capacity growth, IO write-all, EINTR retry,
per-argument EBADF suppression, and the
formatter-only error raise while its current guard is held. Actual IO errors
return only after the statement guard is released. Each component then performs
its own `print_to` IO-error panic and owns the returned IO error.

`Statement` and `Binding.PrintStderr` are borrowed operations, with no invented
closure environment destructor. They always receive the current `rustfmt.Context`.
`Binding` includes the complete capture/lock operation; embeddings may resolve
their current source thread in that callback. `Runtime.Binding(scope)` borrows
one **specific** source thread. A shared multi-threaded component must not use
that same scope concurrently. Use separate scopes sharing one Runtime, and
explicitly share Capture where the source runtime does so. Plain `thread::spawn`
does not automatically inherit capture in the selected SDKs; the native test
sets each child's capture explicitly.

`ruststdio.PrintStderr` uses the shared native Runtime and a fresh uncaptured
scope. Its declared convenience domain has no pre-held stderr lease or ambient
capture. Captured and pre-held scopes use an explicit binding. No Go goroutine
ID, OS thread ID, goroutine-local state, or fake thread-local storage is inferred.
The helper does not port the surrounding TLS destructor/access-error API, Arc
allocation counts, global allocator behavior, fair scheduling, or stdio handles
outside the used stderr operations. A `ThreadScope` is exclusively borrowed on
one logical source thread; copying the pointed-to scope, runtime, capture, or
lease values is invalid. Capture bytes returned by `Bytes`/`TrySnapshot` are
independent copies. `Capacity` and snapshots expose the actual scoped Vec
capacity, including the source min8/doubling/max-required growth. The SourceMap
named-constant59+10+7 pattern ends with length76/capacity118 in both SDKs.
Literal-number `format_args!` folding can instead produce one76-byte append;
the authored Statement must preserve the actual source's format-argument form.

The owning primary is `NewCaptureOwnerWithContext`, taking the caller's actual
`rustvec.Owned[S]` Arc resource and borrowed Capture dereference.
`SetOwnedOutputCapture` consumes and returns those tokens. Removal/restoration
does not clone; unwind destroys the removed Arc after unlocking its capture;
successful restoration destroys any replacement slot only after restoring the
original identity. `CaptureOwner.CloseWithContext` and
`ThreadScope.CloseWithContext` supply explicit destruction, always with the
current caller context. The native tests compare external actual Arc counts at
each reachable point. This composes complete caller Arc implementations without
pretending the helper ports Arc's complete API, physical allocation identity or
arbitrary reference-count limits. The `*Capture` setter is a separate trivial-
destructor convenience, has no Arc-count credit, and cannot replace an owning
slot. Do not retain borrowed Capture views beyond their source owners.

The reentrant counter is source `u32`, not host usize. Its increment is checked
even in release, and overflow leaves the held count unchanged before the
`Option::expect` OwnedString panic. Exact archived source establishes the
branch; the private Go limit test is explicitly synthetic and does not claim
to construct four billion native public guards. Its optional semantic location
key is `rust.std.sync.reentrant_lock.lock_count_overflow`; the source expect
callsite is reentrant_lock.rs291:45 in both selected SDKs.

Source panic metadata comes only from the caller's declared location profile.
The formatter-only failure has semantic key
`rust.std.io.write_fmt.formatter_error`; the actual selected SDKs report
`/rustc/<commit>/library/core/src/io/write.rs:411:17`. Missing metadata remains
unlocated. Source callbacks raise through the current Context. Raw Go callbacks
retain Go-local defer behavior; an escaping processing-mode panic is guarded
before source stderr/capture/caller-lease cleanup.

The Unix default uses the actual fd2 operation. The Windows default is the used
ASCII Graph/SourceMap console/redirected boundary; general Unicode console
conversion is a separate selected raw Write backend contract and is not claimed
by this adapter. The original runtime proof below is Linux-native only. Production
uses Go stdlib and the already ported neutral `rustfmt` runtime.

## Original-source proof

The generator builds the unmodified installed SDKs' public APIs, not a rewritten
stdio oracle. Official matching rust-src archives verify the exact used source
files: `std/io/stdio.rs`, `core/io/write.rs`, `std/sync/reentrant_lock.rs`,
`std/sync/poison.rs`, `std/sync/poison/mutex.rs`, and the used alloc Vec/raw Vec,
byte IO and Arc definitions. Their complete archive and
member hashes are retained in `manifest.json`; the source archives remain in
the owned ignored `.toolchain/stdio-sdk` cache. The SDK source license is Rust's
MIT OR Apache-2.0, retained in the repository's upstream Rust license copies.

The two exact profiles are rustc1.99.1 commit
`58cf8f9c35b355199727627d07ae34a4e2846d67` and official nightly1.101.0
`708c94ef093ecc42fb8851e17d8383d999727abc`. No SDK body is patched. A child-only
libc `write` observer records raw fd2 calls and injects exact partial writes,
EINTR, EBADF, WriteZero, and EIO. It has no production role.

The retained178 cases include20 actual SIGABRT processes. They cover pre-held
and nested locks; outer capture with recursive global fallback; successful and
failed capture replacement; normal/inherited/processing contexts; poisoned
capture reuse; empty/Unicode chunks; independent formatter-only errors; ignored
IO failures; and release-before-IO-panic-hook. The latter hook spawns and joins
another thread that acquires stderr, proving the original guard has been
released. Each SDK also runs64 statements of512 yielding formatter writes, both
to stderr and a shared capture. All records stay whole; no particular scheduler
order is claimed.

The fixture retains actual Arc counts and Vec capacities before/during/after
formatting and panic cleanup. During a held capture guard, actual safe try_lock
reports locked; no inside-lock data snapshot is fabricated. It also retains
exact source hook payload kind/value, file spelling,
line/column, can-unwind/force flags, capture identity, capture poison before the
hook and after cleanup, byte contents, write attempts, and caller scope Drop.
The manifest explicitly excludes terminal panic-abort diagnostics from the
observer after their positively identified prefix, preserving all preceding
operation events and making no post-abort state claim. SDK location strings are
never silently normalized. The Go replay supplies those declared source cells.

Run from the repository root:

```sh
python3 tools/port-rust-stdio/generate.py --offline --check
python3 tools/port-rust-stdio/test_tools.py
go test -race ./port/support/ruststdio
go vet ./port/support/ruststdio
```

Omit `--offline` only when fetching a missing exact source archive. Compiler
selection is explicit via `--rustc-199`, `--rustc-1101`, or
`FERRUGO_STDIO_RUSTC_199`/`FERRUGO_STDIO_RUSTC_1101`. Empty, missing or differently
pinned selections fail. `--cache` selects an owned source cache. A shared trusted
`.toolchain` parent symlink is supported; cache-root/descendant symlinks and bad
ownership/checksums are rejected. Valid existing gzip bytes are reused only
after bounded CRC-checked decompression equals the new native raw oracle. This
keeps `--check` stable across supported Go/Python compression implementations.
