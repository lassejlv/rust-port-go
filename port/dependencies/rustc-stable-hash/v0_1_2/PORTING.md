# rustc-stable-hash 0.1.2

This port translates all four implementation files from the exact official
`rustc-stable-hash` 0.1.2 archive, with the two original test files and three
source doctest examples preserved. Independent source/API review and final native/minimum-Go gates passed.
Production code uses the Go standard library and neutral `rustfmt` support;
it never invokes Rust or substitutes another hashing implementation.

Official archive SHA-256:
`781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08`.
Its VCS revision is `bda19e8681a381715fc3f32ee95aa5fda0a7f5d0`.
`source-manifest.json` records all 17 archive members, byte/line counts and hashes,
including all 6 Rust files (1,441 lines/50,695 bytes). The crate has no external
production dependencies and no benchmark files. Both original licenses and all
non-Rust archive support files are retained. This dependency inventory does not
add files to the Rust/Cargo project-source completion ledger.

The public Rust `hashers` module's reexports are the same top-level Go types:
`SipHasher128`, `SipHasher128Hash`, and `StableSipHasher128`. The common stable
alias is the non-generic Go alias of
`StableHasher[*SipHasher128, SipHasher128Hash]`, valid on Go 1.23. `UInt128` and
`Int128` hold low/high two's-complement limbs; no integer precision is lost.

The buffered Sip1-3 algorithm preserves the 64-byte buffer plus 8-byte spill,
short/slice processing paths, batching, processed counters, and 1/3-round
schedule.
The private Sip13Rounds policy is the Go zero-state sip13Rounds type, retaining
its single/triple round helpers, value-copy Clone/default replacement, zero-value
Default and unit-name Debug. State Clone/Copy maps to ordinary Go value copy.
Typed raw input uses native byte order and native usize width, while
StableHasher performs the source little-endian conversions and usize widening.
Signed isize keeps its original one-byte/0xFF-marker compression. Unsigned
128-bit stable input uses two u64 short writes; signed 128-bit stable input uses
one 16-byte slice write. These paths remain distinct for custom H callbacks and
buffer state even when their final digest coincides on a little-endian host.

`Finish64` is the source borrowed Hasher result `lo * 3 + hi` with wrapping
arithmetic. `Finish128` consumes a concrete SipHasher128; `FinishOwned` and
`FinishInto` consume generic StableHasher state/results. `FinishHash` is a
plain-value transfer convenience. Consumed witnesses reject subsequent value
use. The Go zero SipHasher128 lazily acquires the source Default state; ordinary
constructors initialize immediately. Go storage initializes the entire buffer,
where Rust leaves its unused bytes uninitialized. Those unused bytes are never
hashed and Debug prints only `MaybeUninit<u64>`, preserving its source padding
and precision behavior. No Rust ABI, uninitialized-memory contents, allocator
timing, or out-of-bounds pointer-copy behavior is claimed.

`ShortWrite(length, bytes)` retains const-LEN identity separately from the
payload. A Go-only mismatched length is rejected, not truncated. Debug source
assertions reject LEN above eight. Defined release cases above eight are
preserved, including nine-byte spill cases and larger arrays that remain in the
small-buffer branch. Cases that would perform undefined Rust pointer copies
fail explicitly. `ExtendedHasherOps.ShortWriteArray` preserves arbitrary LEN
for generic H and defaults to its byte Write when no short-write implementation
is supplied.

`ExtendedHasherOps[H, Hash]` is the static ExtendedHasher/Hasher trait dictionary.
H and its associated Hash may be arbitrary non-Copy/non-comparable values.
`Ref[H]` points to the actual state cell for every borrowed callback, carrying
explicit ambient `rustfmt.Context`. Typed method slots retain independent
source overrides/default forwarding; `HasherRef` exposes the borrowed H trait
without copying it. Conditional Default/Clone/Debug bounds require their
explicit witnesses. These dictionaries model static operations, not additional
owned closure environments: destructible fields/captures must live in H or a
stated owned value; callback captures may contain borrowed state or metadata.
Write's bytes are a temporary shared read-only borrow. ShortWrite's array is
passed by value in Rust and becomes an independent owned byte array for its Go
override, which may mutate or retain it; default forwarding borrows it for Write.

`Owned[T]` models consuming parameters and outputs. Move invalidates old
witnesses; Borrow is a temporary source borrow, and Take transfers the value.
Drop describes complete value destruction, including fields and local cleanup
rather than just the body of a user Drop::drop. References must not outlive a
move/drop or incompatible borrow. `FromStableHash[Hash, W]` consumes the exact
associated Hash witness and returns an owned W, preserving arbitrary result
ownership. Plain conveniences cover trivial-drop values; explicit witnesses
cover nontrivial output and callback lifetimes.

Clone invokes H's Clone on the actual borrowed cell. CloneFrom performs whole
StableHasher replacement, not H.clone_from: clone source, drop old H, then
install the replacement in the existing receiver cell even if old Drop panics.
Raw SipHasher128 replacement and lazy zero initialization preserve the caller's
ambient panic context; that support metadata is not part of the cloned Rust
hash state. Native normal/inherited/processing cases cover replacement followed
by a failing source short-write assertion.
Borrowed interior mutation remains observable across Finish64 calls. Consuming
H.finish drops its owned H before FromStableHash runs. If H's destructor panics
after preparing a Hash result, that result remains leaked as in Rust. Likewise,
if the consumed Hash destructor panics after preparing W, W remains leaked.
Normal, inherited-unwind, and processing-panic contexts are distinct. A second
panic in locally escaping cleanup aborts; a panic that a caller can still catch
inside an inherited unwind is not automatically an abort. Context.Panic supplies
source panic behavior inside callbacks, including immediate abort during panic
processing. Exit 134 represents that abort class, not a claim of SIGABRT ABI.

Debug uses the primary mutable `rustfmt.Formatter` and a fallible writer, with
independent string/character sink overrides. Concrete derived fields preserve
pretty/hex/sign/width/fill/precision and post-error formatting state. The generic
StableHasher Debug body invokes a fresh default-options formatter around H,
matching its source `write!(f, "{:?}", self.state)`; caller options do not leak
into H. Ambient panic context and actual H-cell identity are retained.

The `rustc_stable_hash_nightly` build tag supplies exactly the source crate's
nightly-only concrete WriteString/WriteLengthPrefix overrides. Stable/default
HashString and HashLengthPrefix helpers still support std Hash's inherited
operations. A borrowed custom-H trait dictionary exposes caller-provided std
hooks independently; the concrete crate tag controls its source delegation.

`ArithmeticProfile` represents debug-assertions and overflow-checks separately.
DefaultProfile selects both; ReleaseProfile selects neither; all four
combinations are supported explicitly. The original int_overflow debug branch
uses ordinary arithmetic, so debug=true/overflow=false wraps. Its release
branch explicitly wraps even when overflow-checks=true. The Go helpers preserve
this actual source behavior rather than changing it to match the broader prose
comment in the original file.

The development oracle verifies the archive and every expanded registry file,
then builds the unchanged crate in all eight feature/arithmetic combinations.
It checks streaming/digests/state snapshots, generic default/override calls,
fallible Debug writes and formatter state, ownership/cleanup/abort traces, and
the exact original private arithmetic helper. See
`tools/port-rust-stable-hash/README.md` and `report.json` for commands and counts.
Original test/literal/benchmark accounting is in `original-tests/audit.json`;
there are no source benchmarks. Native amd64 execution is validated. Complete
386/ARM packages cross-compile, with no 32-bit runtime claim on this kernel.
