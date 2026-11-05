These are scoped genuine container codec dependencies for pending unord.rs,
not completion claims for unord.rs, rustc_serialize or standard-library files.
Production is Go only. The development observers link the whole unchanged
pinned Rust compiler source graph and execute the actual generic container,
opaque and Unord derive implementations; no expected output comes from Go.

The 1,674 decoding records cover generic HashMap/HashSet/Vec and derived
UnordMap/UnordSet/UnordBag. They record decoder consumption, owned constructor,
Default/BuildHasher/Hasher/Hash/Eq callbacks, duplicate replacement, partial
initialization, full destructor order and panicking state. Read, hash, equality,
Default and destructor failures preserve the observed partial state. Source
layouts are explicit: keys/values are 16/8, map pairs 32/8. The additional owned
context corpus described in ../bag/README.md validates actual nested field and
compiler cleanup boundaries, including initialized key failure before map
cleanup. Current context is forwarded through the borrowed decoder operation;
no container dictionary stores an earlier runtime scope.

Generic vector decoding supplies the real trusted Map<Range<usize>,decode>
iterator to a statically selected actual Vec FromIterator strategy. This avoids
a rustc_serialize -> rustc_index import cycle. It is not an arbitrary opaque
callback: that strategy owns the actual iterator/cells and must preserve its
specialization, allocation/capacity, complete cleanup and return-place behavior.
The production Bag strategy composes existing IndexRaw/IndexVec storage. Full
logical ZST lengths are not projected to Go int-sized slices.

The vector specialization observer adds 292 records: generic element emission
and every selected emit failure, derive framing, true opaque bulk-byte dispatch,
owned byte decoding, independent copied output, exact capacities, and exhausted
or usizeMAX-declared buffers. MemEncoder's returned Vec capacity distinguishes
a bulk raw write from per-byte emission even when serialized bytes agree.
The contiguous source slice is borrowed directly, with no temporary input
copy. Decoding copies its borrowed input once into the owned vector storage.
Byte helpers apply only to Rust's primitive u8; newtypes supply their own traits.
Selected non-generic slice/Vec specializations remain explicit operation slots.

The companion wrapper corpus retains 528 actual source processes, including
32 abort prefixes, for direct and distinct wrapped MemEncoder/FileEncoder
values. A wrapper uses generic per-byte encoding even when Go embedding
promotes an opaque marker. The raw factory therefore checks exact concrete
types; source aliases retain their identity. For 31 bytes, the native memory
encoder records zero EmitU8 calls and capacity 32; the wrapper records 31 calls
and capacity 40.
Ordinary, inherited and processing contexts include callback-local Drop and
fresh/resumed failures. Adapters representing the same original concrete Self
use explicit OpaqueByteVectorEncodeOps instead of inheriting discovery. Vec
and UnordBag share this actual slice specialization; the earlier 292-record
fixture remains byte-for-byte unchanged.

The exhausted primitive's actual static-string payload kind, original relative
source location, hook flags and decoder position are retained. Runtime metadata
is installed through the decoder's current-operation binding; constructors do
not capture it. Raw callback Go panic/defer timing is a convenience boundary;
complete translated panic sites use Context.PanicAt and the exact selected
source profile. Existing source fixture bytes remain unchanged.

Run from the repository root with the shared full Rust graph prepared:

    python3 tools/port-rust-unord/codec/generate.py --check
    python3 tools/port-rust-unord/codec/generate_vector.py --check
    python3 tools/port-rust-unord/codec/generate_wrapper.py --check

FERRUGO_SPAN_REFERENCE_PROFILE and FERRUGO_GO select the verified reference
profile and validated Go compiler. Check mode rewrites neither fixture bytes
nor machine evidence. Existing valid gzip representations are preserved.

The wrapper companion uses the shared complete-reference loader on every run:
original Git blobs, all registry archives and extracted sources, compiler
selection, all 43 upstream tests, and external linkage are revalidated. Cached
profile JSON supplies build flags only; its compiler and artifact commands are
never executed. Its three loader/selector inputs are bound in the evidence.
