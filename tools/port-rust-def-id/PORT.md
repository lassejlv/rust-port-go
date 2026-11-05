# def_id.rs source port (in progress)

Pinned Rust revision: `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.
The original 579-line file has Git blob
`9134e652987c9fee1cdc82eecc4314c2c16ec6eb` and SHA-256
`6f40f30da61a682ceb75898f2b021bff0c75e438824f9860d612043ce3168b1e`.

This work has **no full-file completion credit yet**. The six Unord collection aliases now bind the real backing owners and Entry
objects. StableCrateIdMap now binds the genuine partial indexmap2.14 adapter
with the actual Unhasher builder. Higher Unord and remaining IndexMap APIs
still block complete inherited interfaces. No Go map alias,
dummy crate ID, fabricated stable-hash key, or production Rust fallback supplies
those dependencies. The surrounding span/session group is also in progress.

The implementation keeps the existing DefIndex and LocalDefId value APIs.
Struct-valued Rust constants map to immutable value getters LocalCrate,
CrateDefIndex, CrateDefId, CrateModId, and the newtype Zero/Max getters. Both actual
newtype_index invocations are expanded, including checked conversions,
Add/AddAssign, Step required/provided operations and Idx adapters. Unsafe input
outside the source pattern-type invariant is rejected as a Go-only domain error.
Step checked arithmetic may still panic when its usize result exceeds the
newtype maximum; it does not turn that constructor assertion into None.

DefId, LocalDefId, ModId and LocalModId get no invented ordinary ordering.
Source From conversions map to ToDefId/ToModId/ToLocalDefId. DefId Hash preserves
its one-u64 expression on64-bit hosts and its index-then-crate u32 sequence on
other hosts. HashWithHasher primaries provide the complete required Hasher
contract and independent typed defaults/overrides, retaining arbitrary actual
state through the real dependency's HasherRef. Inherited hash_slice defaults
are explicit and write no length prefix. DefPathHash's concrete-H dictionary
helpers preserve Fingerprint specialization across the wrapper. DefPathHash
uses the real Fingerprint including the original
Unhasher specialization; stable hashing always retains both fingerprint limbs.

NewStableCrateId resolves a real Symbol's text from its actual session and uses
the complete ported stable Sip hasher. It sorts/deduplicates concrete metadata
strings, hashes each usize byte length, includes exe/lib and reads the forced
OS version at the source point after metadata hashing. Rust's lossy UTF-8 error
boundaries are preserved, including incomplete valid prefixes. Symbol hashing
by index and Symbol StableHash's length+bytes algorithm are distinct from this
source call's std string Hash sentinel. Go string inputs must be valid UTF-8;
Go-only invalid strings are rejected. No Rust allocator timing or ABI is claimed.

The primary Debug APIs use a mutable rustfmt.Formatter and fallible string/char
writer. Macro numeric Debug resets nested formatting options, while derived
fields retain the source compact/pretty and post-error state. The static Debug
hook uses immutable callback cells and sequentially consistent atomic swap/load
with cell identity. Callback cells represent static source function pointers;
owned destructible Go closure captures are not extra source fields.

ExpectLocal formats its dynamic panic arguments after panic processing starts.
A Debug panic during that phase aborts; fmt::Error instead leaves a best-effort
partial panic message. Ordinary allocating Debug string formatting retains its
separate formatting-error panic behavior. Fresh panic primaries use explicit semantic-site profiles: constructor macro
locations, Step/Idx defaults, expect_local tracked callers and ModId internal
callers retain actual source coordinates, or nil when no profile is supplied.
Explicit normal/inherited/processing contexts model the used source operations; this is not a full Go implementation
of the standard Rust panic-hook runtime or its ABI.

ContextOrd returns owning comparator descriptors. An actual arbitrary non-Copy
Fn environment is retained in one owned cell, with independent Fn/FnMut/FnOnce
operation witnesses, full environment/field destruction and current panic
context. Nil FnMut/FnOnce slots mean a specifically lawful compiler-generated
forwarding strategy, not a Rust trait default. A comparator's consuming FnOnce
call destroys the capture but its body still calls the captured H through Fn.
Callbacks are static operation dictionaries: destructible captures belong in
the actual owned state. Shared/exclusive source borrow rules remain caller
obligations. Source callback panic sites use Context.PanicAt with explicit metadata; raw Go callback-local
defers/recover remain host behavior, with only escaping panics guarded.
WithContext constructors/operations receive current caller runtime metadata;
no-context conveniences clear borrowed Runtime metadata while retaining panic
guards. An Owned.Context snapshot is not implicit current caller TLS. The two
adapter regressions check actual state transfer, callback and Drop contexts.

The approved partial data_structures stable_hash bridge provides actual RawDefId,
the real StableHasher alias, all required context callbacks, derived controls
operations, and borrowed actual-cell context dictionaries. It grants no full-file
credit to stable_hash.rs. Other completed dependency implementations remain
unchanged.

## Native reference

The reference compiles the **entire unchanged def_id.rs**, real procedural macro
sources, whole original Unord/Fingerprint/Unhasher/AtomicRef/fx/Idx source and real
locked codec/hash/indexmap dependencies. The surrounding Symbol text operation
and used SpanEncoder/decoder interfaces are functional providers; they are not a
claim to compile full Symbol/lib sources. Every selected used default/codec body
is retained byte-for-byte with source offsets and hashes. All registry archives
and expanded source members are verified before and after reference builds.

```sh
python3 tools/port-rust-def-id/reference-build/prepare.py --fetch --write-lock
python3 tools/port-rust-def-id/generate.py --offline
python3 tools/port-rust-def-id/generate.py --offline --check
```

FERRUGO_GO, FERRUGO_RUSTC and FERRUGO_CARGO accept absolute paths or PATH names;
explicit invalid/empty overrides fail. CARGO_HOME is respected. Go1.23 is the
minimum; the native formatter reference is Rust1.99.1, while the source pin's
version is1.101.0. This is source/API validation, not a whole-compiler ABI claim.
The current native record counts and exact source/helper bindings are in
manifest.json. Coherent package integration, final races and independent review
are required before changing the pending status.

The Step type-name regression uses the independently verified complete original
`rustc_span` and `rustc_index` crates with their canonical crate names. Nine
native cases retain `type_name::<I>()` and compact/pretty `IntervalSet<I>` Debug
for `rustc_span::def_id::DefIndex`, `rustc_span::def_id::CrateNum`, and
`rustc_span::AttrId`, over empty, singleton, and multiple-interval sets. This
client uses the whole-span reference's dated Rust 1.101 nightly profile; the
older def-id facade would expose its development crate name and cannot prove
these names. The only AttrId production delta is the authorized TypeName string.

```sh
python3 tools/port-rust-def-id/interval_type.py --offline --check
```
