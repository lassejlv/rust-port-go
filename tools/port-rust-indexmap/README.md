# Supporting IndexMap operations — in progress

This is a **partial compatibility implementation**, with no whole dependency
or `def_id.rs` completion credit. The source is official `indexmap=2.14.0`,
archive SHA-256
`d466e9454f08e4a911e14806c24e16fba1b4c121d1ea474396f396069cf949d9`.
The full 42-file archive inventory (32 Rust files, 15,532 lines) is retained in
source-manifest.json. Its applicable feature is default `std`; other optional
features receive no implementation/validation claim.

The supporting Go package keeps the source's two real containers: a hashbrown
SwissTable of `usize` indices and an insertion-ordered dense stored-hash vector.
It does not substitute a Go map or a linear-search table. It accepts arbitrary
Go key/value representations through explicit genuine source operation
dictionaries, including independent CloneFrom implementations. Target layouts
and profiles describe Rust values, rather than Go ABI. Private allocation and
GC timing are not a Rust allocator/ABI claim.

Implemented core operations include constructors, capacity/reservation,
required/default BuildHasher dispatch, equivalent queries, indexed access,
insertion, swap/shift removal, pop, clear, reverse, Clone/CloneFrom, equality and
streaming Debug and the implemented occupied/vacant Entry transitions. The
scalar corpus checks these operations and full hash-one callback order. A second
corpus retains 2,016 baseline and 32,040 one-shot failure cases using actual non-Copy
keys/values and owned FnOnce capture environments. It checks normal and inherited
unwinding, independent CloneFrom overrides, initialized return-place leaks, Drop
order and partial map state after failures. Concrete borrowed Iter/IterMut/Keys/Values/ValuesMut cursor operations,
Clone/provided CloneFrom/Default, remaining slices and mutable slice transfer
now have 506 original cases / 9,108 steps, including integer formatting flags.
Remaining generic iterator fold/collect, sorting, raw-entry,
mutable-key, allocation-error and additional map/slice APIs are pending. `StableCrateIdMap` binds these genuine supported operations through its exact
fixed key/value/builder types; its remaining inherited interfaces are pending.

Constructors impose no extra Hash/Eq/BuildHasher/Clone bounds. The new
OwnedBuilder[S] protocol retains the actual arbitrary non-Copy S cell, with
independently conditional BuildHasher/Clone/CloneFrom operations and complete
owned destruction. Unavailable traits reject invalid Go adapter calls. Its 432
native cases cover keys without Hash/Eq, stored S without BuildHasher/Clone,
non-Clone H, deep Clone, shared-state interior mutation, custom CloneFrom and
the exact default drop-and-replace on old Drop panic. The native compiler also
rejects the corresponding unavailable Clone call. A separately proved scoped
rustvec.ReplaceOwnedWithContext helper supplies same-cell replacement and
current-scope cleanup; its 72 native cases include 30 actual SIGABRT outcomes.

Owning IntoIter now has 1,360 original baselines + 10,648 one-shot failure cases.
IntoKeys/IntoValues have 2,560 baselines + 24,768 failure cases. These preserve
builderDrop prepared-Vec leaks; cursor updates before skipped-field destruction;
forward-memory order for skipped backward tails; eager opposite-field Drop;
prepared retained-field leaks after discarded-field panic; prefix cleanup before
remaining input; Count/Last return-place leaks; conditional Clone traits; and
normal/inherited Drop states. Both projection types use the actual same-layout
in-place transformation, with private logically uninitialized fields that are
never exposed, cloned or destroyed as initialized values. Generic collect/fold
and remaining map/slice/raw/sort APIs are still pending.

Ranges, truncate and Drain now have 2,156 original baselines + 1,928 caught
callback/destructor failures. A separate corpus with 164 hook cases retains 39 actual
native SIGABRT outcomes. The owned RangeBounds adapter calls the actual state
and destroys the complete consumed range before changing the map; reversed
range diagnostics call start/end again before formatting. Drain retains the
exclusive map borrow, cursor/drop order, restoration of the shifted tail after
partial Drop failure, Count/Last prepared-return leaks, and mem::forget's actual
truncated state. Calls during panic-hook processing abort before unwinding
local library cleanup. The real hash table performs all three erase_indices strategies;
its sweeping path snapshots only bucket numbers, ends iterator/cell borrows,
then mutates the original buckets in source order. This preserves temporary
duplicate index values under colliding hashes. No general std Vec/RangeBounds
or whole IndexMap implementation is claimed.

SplitOff, MoveIndex and SwapIndices have 1,568 original baseline cases and
120 builder-clone failure cases. Keys and values have no Clone implementation;
splitting requires only the source builder's Clone. The proof retains exact
left/right capacities, the already-split original prefix after Clone failure,
right-side cleanup, stored-index updates with colliding hashes, out-of-bounds
behavior and normal/inherited contexts. These operations add no key hashing or
key-equality calls while adjusting the stored index table.

IndexedEntry now covers indexed/first/last entry acquisition, conversion from
OccupiedEntry, borrowed key/value access, replacement, consuming mutable access,
removal and position changes. Its 3,360 baselines and 14,544 injected failures
retain post-failure map/borrow state and prepared-value leaks. Entry enum,
OccupiedEntry, VacantEntry and IndexedEntry Debug have 120 baseline and 2,376
writer-failure cases, including character dispatch, caller formatting options,
pretty indentation and mutable formatter state after errors. Separate native
compile checks preserve the weaker VacantEntry Debug bound.

The source's zero/singleton lookup and removal shortcuts avoid hashing; its
Core Clone/CloneFrom reserve and overwrite sequence is preserved. Future sort
and callback operations must retain original partial state after panics;
`Core::with_entries` rebuilds its index only after successful callback return.

The first retained corpus contains 60 original cases and 18,000 stateful
operations, checking values, order, capacities, Clone/CloneFrom, default Debug,
arbitrary Equivalent callback order and the exact default
Build → Hash → Finish → owned Hasher Drop protocol. It builds the whole unchanged
official crate using the independently committed def-id reference builder;
all pinned archives/expanded sources and local compiler inputs are verified.
The development compiler is Rust 1.99.1 on Linux x86-64. Production Go operations
never call Rust.

```sh
python3 tools/port-rust-indexmap/generate.py --offline --check
python3 tools/port-rust-indexmap/owned_generate.py --offline --check
python3 tools/port-rust-indexmap/iterator_generate.py --offline --check
python3 tools/port-rust-indexmap/builder_generate.py --offline --check
python3 tools/port-rust-indexmap/into_generate.py --offline --check
python3 tools/port-rust-indexmap/projection_generate.py --offline --check
python3 tools/port-rust-indexmap/drain_generate.py --offline --check
python3 tools/port-rust-indexmap/position_generate.py --offline --check
python3 tools/port-rust-indexmap/indexed_generate.py --offline --check
python3 tools/port-rust-indexmap/entry_format_generate.py --offline --check
python3 tools/port-rust-indexmap/replace-owned/generate.py --offline --check
go test -race ./port/support/rustindexmap ./port/support/rusthashmap
```

FERRUGO_GO, FERRUGO_CARGO and FERRUGO_RUSTC support explicit paths or PATH names,
preserving symlink invocation names. Explicit missing/empty tools fail.
CARGO_HOME is honored. The Go minimum is 1.23.
