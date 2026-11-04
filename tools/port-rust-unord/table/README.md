# Scoped backing for Unord collections

This supporting implementation is a real SwissTable, Fx hasher, standard-map
entry path, and owned collection/iteration adapter. It uses no Go map and no
production Rust fallback. It grants no completion credit for std, hashbrown,
rustc-hash, or the still-partial `unord.rs` port.

The original `unord.rs` uses `std::collections` through rustc-hash 2.1.1.
Its observed backing is hashbrown 0.17.1. The hashbrown source files in the
official Rust 1.99.1 and 2026-11-04 nightly source archives are byte-identical
to that exact registry archive. The two compiler/standard-library profiles are
retained separately; the generator actually runs both before sharing expected
records. Host profiles are Linux x86-64, little endian, with sixteen-byte SSE2
groups. The separately forced eight-byte scalar group proof compiles the
unchanged hashbrown crate with `cfg(miri)` to select that original implementation;
it does not claim Miri interpretation or another target execution.

The real table retains bucket position, mirrored control bytes, triangular
probing, deleted tags, source-size-dependent minimum allocation, resize versus
in-place rehash, equality-callback order, clone rollback, and source drop order.
The source Rust layout is explicit metadata; a Go value's size is insufficient.
Even zero-sized and aligned Rust elements use their actual source layout.

Std's `rustc_entry` path differs from hashbrown's ordinary Entry: it looks up
before reserving and calls stored-key equality first. `insert` reserves before
equality and calls incoming-key equality first. Std OccupiedError retains both
the uninserted key and value. Prepared return values can leak if parameter/key
destruction panics. These distinctions are exercised against the original.

`KeyOps.HashOne` and `BorrowedLookup.HashOne` are typed monomorphized witnesses
for an overridden `S::hash_one::<&Q>`; nil means the actual trait default, not an
omitted implementation. Actual FxBuildHasher and BuildHasherDefault<Unhasher>
use that default. Hasher defaults preserve typed and string/length dispatch.
Unhasher must keep its actual concrete type for Fingerprint specialization.

Ownership operations consume live `rustvec.Owned` cells. Drop/clone/equality/hash
dictionaries represent complete source operations, including fields and local
cleanup. No destructible source captures may hide in Go callbacks. Shared
borrows may mutate only actual source interior-mutable state, and references
cannot survive incompatible mutation, move, relocation, or destruction. Rust
Send/Sync/aliasing requirements remain obligations of these type dictionaries.
There is no automatic GC-based source destructor.

The scoped neutral iterator additions distinguish original `for_each` from
original `fold`. Map Extend chooses the former; Set's fixed Map adapter chooses
the latter. The optional `MapToUnit` slot represents the selected monomorphized
constructor, including actual key consumption through its fixed mapping Fn.
Its receiver and result are real owned state cells. Internal consumers capture
only borrowed state and perform complete moved-value cleanup on panic.

Reproduce all backing proofs after installing the two recorded compilers and
preparing the exact Cargo registry archives:

```sh
FERRUGO_GO=go FERRUGO_RUSTC=rustc \
FERRUGO_RUSTC_NIGHTLY=/path/to/rust-1.101-nightly/bin/rustc \
python3 tools/port-rust-unord/table/check.py
```

Explicit executable overrides must resolve, and compiler versions must match
the manifests. Checks are offline and do not rewrite expected evidence. Exact
archive hashes come from the pinned upstream Rust lock. Original archives are
extracted into temporary workspaces and compiled unchanged. Existing valid
gzip bytes are preserved when Python implementations produce different headers.

The reference evidence contains:

| Proof | Observations per actual compiler profile |
| --- | ---: |
| Map operations, capacity, entry and clone state | 96,000 operations / 1,500 SHA256 blocks |
| Owned callbacks, panic cleanup and partial state | 1,098 |
| Panic-hook and escaping-second-panic processes | 864, including 837 real SIGABRT prefixes |
| Full typed/byte/string Fx operations and clone independence | 5,548 |
| Narrow/aligned/bool/zero-sized sets | 4,096 |
| Map/Set/Entry/OccupiedError Debug writes/errors/state/reuse | 43,160 |
| Extend/FromIterator method dispatch and owned cleanup | 1,494 |
| Deliberate hash_one override skipping all hasher/key methods | direct original trace |
| Scalar lookup false-positive equality calls | three original trace families |

Aborted records contain only actual observed prefix events and the native abort
class. They never invent post-abort table state. Go uses immediate exit134 for
the corresponding source abort, as other source-runtime adapters do.

The partial real UnordMap/read-items foundation has a separate proof under
`../map`, linked to the complete unchanged compiler crate graph. Higher Unord
iterator algebra, sorting, codecs, and order-independent stable hashing remain
pending and receive no whole-file source credit.
