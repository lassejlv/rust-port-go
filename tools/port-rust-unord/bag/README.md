This is a genuine UnordBag storage/trait foundation for the still-pending full
`rustc_data_structures/src/unord.rs` port. It adds no Unord, standard-library,
or hashbrown source-file completion credit. Generic UnordItems algebra and
sorting remain separate unfinished work; no placeholder methods represent them.

Production uses the existing IndexRaw/IndexVec implementation with identity
usize indices, including logical ZST lengths/capacities and the new current-call
runtime bridge. It does not introduce a second vector engine or call Rust.
Element layout and trait dictionaries describe the actual Rust type. Clone,
Drop, Eq and ne are distinct operations; TrivialClone is the actual unsafe Rust
marker, not merely Copy. Drop dictionaries include complete field destruction.
Callbacks borrow actual source cells and current context, obey Rust's borrowing
and invalidation rules, and have no hidden destructible Go captures. The source
Default bound is retained as a caller trait obligation, while its empty Vec
construction never calls T::default.

The native observer links the entire unchanged pinned compiler crate graph.
Its 1,260 records cover whole-value Clone/CloneFrom, independent Eq/ne dispatch,
owned and borrowed count/get_only, owned allocation transfer, default creation,
commutative StableHash and partial caller-hasher state, and fallible streaming
Debug. Debug records include independent write_str/write_char operations, every
failure position, error identity and mutable formatter state. The returned
opaque item handles expose neither Next nor IntoIterator. They also do not
expose Clone: the Rust items()/into_items() return bounds omit Clone even when
the hidden concrete iterator implements it. Generic UnordItems::new Clone
support is still part of the pending general iterator work.

The independent context observer executes 1,872 actual Rust child processes;
602 terminate with SIGABRT. Actual Value/Token cells own their fields, mutate
Option<Token> when moving a token, and destroy moved tokens in the caller.
Normal, inherited-unwind and panic-hook calls cover fresh hooks, resumed
payloads, complete automatic field cleanup, second/first/remaining get_only
destruction, partial vector/map/set decode, and tuple-key cleanup before an
already-populated map. It distinguishes a new locally caught panic in an outer
destructor from a second panic escaping a real compiler cleanup edge. Only the
observed prefix is retained for an abort; no postmortem state is fabricated.
Go exit 134 represents the abort class, not SIGABRT ABI identity.

All contexts are borrowed at operation entry; neither Bag storage nor escaping
iterator dictionaries retain a construction-time runtime. Source panic sites
use PanicAt, recovered payloads use ResumeUnwind, and real cleanup guards use
AbortCleanup. The caller supplies immutable source-location profiles verbatim;
missing locations remain nil. Raw Go callback panics retain Go-local defer
timing; complete translated callback sites use the supplied contextual API.
The ordinary current-call and no-observer storage laws are also covered by the
separate Index bridge proof and nil-runtime identity regression.

The context corpus includes actual UnordBag<u8>/MemDecoder exhaustion. Its
static string, original/compiler/rustc_serialize/src/opaque.rs:319:9 location,
hook flags, decoder position 1, and immediate processing-panic abort are
compared. The separately owned primitive implementation supplies that genuine
used dependency; its surrounding serialization source remains partial.

Run these from the repository root with the shared full Rust reference prepared:

    python3 tools/port-rust-unord/bag/generate.py --check
    python3 tools/port-rust-unord/bag/generate_context.py --check

FERRUGO_SPAN_REFERENCE_PROFILE selects that verified graph; FERRUGO_GO selects
and validates Go. Check mode changes neither fixtures nor evidence. Valid
existing gzip bytes are preserved. The minimum supported Go version is 1.23.
