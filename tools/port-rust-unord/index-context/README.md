This is a narrow operation-time runtime bridge over the existing complete
`IndexVec` / `IndexRaw` storage. It supports the still-pending UnordBag work and
adds no standard-library or Unord source-file completion credit.

The independent native observer links the complete unchanged pinned Rust compiler
crate graph, then calls the actual `UnordBag` methods and SDK `Vec` / `IntoIter`
operations. Native expected responses never come from Go. The 228 retained
processes cover observer A at construction followed by B or no observer,
normal/inherited-unwind/panic-hook calls, Clone and clone replacement,
initialized-prefix cleanup, explicit element and iterator destruction, Extend,
FromIterator, and resumed rather than freshly raised payloads. Eighty-eight
processes actually terminate with SIGABRT. Only their observed stdout prefixes
are retained. Go exit 134 represents that abort class, not its signal ABI.

The source zero-sized storage cases reach the full `usize::MAX` logical length
without a host-sized allocation. They distinguish generic, trusted-length, and
Vec::IntoIter extension failures, an O(1) clone of a trivial zero-sized vector,
and valid trusted iterators whose true length exceeds `usize::MAX`. RawVec
capacity overflow, trusted Extend overflow, and trusted FromIterator overflow
have distinct original locations despite identical payload text. Immutable
caller-provided site dictionaries preserve those locations verbatim.

`CloneWithContext`, `CloneFromWithContext`, `PushWithContext`,
`ExtendWithContext`, the context-aware FromIterator constructors, and vector /
owned-iterator `CloseWithRuntimeContext` borrow the current `rustfmt.Context`.
`ValueOps.CloneRefWithContext` and `DropWithRuntimeContext`, plus contextual
iterator operation slots, forward it without capturing a construction-time
runtime. Escaping vectors and iterators store only their element dictionaries.
Empty-context legacy methods remain available; old `IndexDropContext` methods
retain their prior explicit cleanup convenience. The new runtime primary
permits a first panic caught inside an outer destructor; only actual escaping
compiler cleanup edges call `AbortCleanup`. Recovered payloads use
`ResumeUnwind`, and intrinsic capacity checks raise at the actual failure phase
before component cleanup. A separate identity regression verifies truly nil
runtime calls after B-scoped cloning and iterator moves.

These are typed operation dictionaries, not arbitrary owned Go closures.
Callbacks borrow actual source cells and current metadata for the call only;
Rust shared/exclusive borrowing, invalidation on mutation/move/drop, complete
iterator/environment/value destruction, and unsafe TrustedLen laws remain
caller obligations. A constructor slot must perform the complete selected
IntoIterator operation, including its input cleanup. Hidden destructible Go
captures and stored runtime scopes are forbidden. Translated source panic sites
must use the supplied context; raw Go callback panics retain Go-local deferred
cleanup timing before their escape guard. Allocation OOM behavior and a Rust ABI
are not introduced by this bridge.

Run `python3 tools/port-rust-unord/index-context/generate.py --check` with the
shared complete Rust reference profile prepared. `FERRUGO_SPAN_REFERENCE_PROFILE`
selects that profile; `FERRUGO_GO` selects and validates Go. Check mode changes
neither fixtures nor machine evidence. Four independent integrity regressions
cover source corruption, symlink substitution, gzip-byte preservation, and
missing coverage. Existing source fixtures remain unchanged.
