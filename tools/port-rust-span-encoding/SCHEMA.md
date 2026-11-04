# Span encoding native protocol, version 1

Each JSONL request creates an independent scoped session (or deliberately omits
it) and executes its operations in order, continuing after caught panics.
Expected responses come exclusively from the actual Rust source. Go generates
requests through TestDumpSpanRequests/FERRUGO_SPAN_DUMP_REQUESTS and replays
retained observations in TestOriginalSpanEncoding.

Request fields: `id` string, `debug_assertions` and `overflow_checks` independent
booleans, `session` boolean, `ops` array. Optional `source_map:{text,panic}` is a
recording implementation of the one caller-owned source-map operation consumed
by source Debug, never a fabricated source-map/location algorithm. Optional
`track_panic_at` is a zero-based tracking-call index, reset for each operation
(default -1). Optional `track_reenter:{lo,hi,ctxt,parent}` creates a real span
inside the tracking callback, proving that no interner lock is held there.

Every span-producing successful operation appends one slot; other operations
and failed constructors append none. Slots are zero-based. Raw u64 bits are
exactly16 lower-case hexadecimal digits to avoid transport precision loss.
Parent is either null or a checked DefIndex integer. All integer inputs except
raw bits are exact JSON integers in the original Rust input domain.

| Operation | Additional inputs | Result |
| --- | --- | --- |
| new, new_ordered | lo,hi,ctxt,parent | raw bits; appends slot |
| raw | bits | raw bits; appends slot |
| dummy | none | raw bits; appends slot |
| clone, ord_clone | span | raw bits after actual clone/default clone_from; appends slot |
| inline_try | lo,len,ctxt | null or raw bits; appends on Some |
| inline_from | span | null or [lo,len,ctxt] |
| data, data_untracked | span | [lo,hi,ctxt,parent] |
| ctxt | span | context u32 |
| parent | span | null or parent u32 |
| is_dummy, from_expansion | span | boolean |
| map_ctxt | span,to, optional panic boolean | raw bits; appends slot |
| with_parent | span,parent | raw bits; appends slot |
| ord_eq, ord_ne | span,other | boolean |
| ord_cmp | span,other | -1,0,1 |
| ord_partial_cmp | span,other | {some:-1,0,1} |
| hash | span, optional panic_at index (-1 default) | null; typed writes in events |
| debug | span,kind,pattern,chars,fail,reuse | see below |

`debug.kind` is span, ord or data. `pattern` selects the19 retained actual Rust
patterns used by the parser Debug proof (listed in oracle.rs); `chars` chooses
an independent write_char override or its default write_str fallback. `fail`
is a zero-based writer call or -1. `reuse` formats native usize26 through the
same mutable formatter after the first result, including after an error.
Debug result: `{ok,first_ok,reuse_ok,writes:[{kind:"S"|"C",text}],accepted,
state:{before:[fill_code,alignment],after:[...],final:[...]}}`.
Alignment is0 unspecified,1 left,2 right,3 center. Callback/writer order is
also in the operation's events, including calls evaluated after earlier errors.

Response: `{steps:[{result,panic,events,interner,slots}]}`. `result` is null on
panic. `panic` is null or a source failure class; runtime locations/messages are
not normalized into an algorithm. `events` is the operation's event delta;
`interner` is the full actual insertion-order [[lo,hi,parent]...] table after
success or catch, and `slots` is the full raw-bit slot vector. Snapshots are
reference-only inspection adapters inside the original module's scope. They do
not alter its source bytes or reimplement interning.

Panic classes: debug_assert,subtract,add,identity_limit,context_limit,
missing_session,index,track,map,drop,source_map,hash. An unexpected panic remains
an explicit unclassified failure, requiring review rather than silent acceptance.

`map_owned` takes span,to,call_panic,drop_panic,move_token and ambient (normal,
unwinding or processing_panic). Its actual user-defined Rust FnOnce owns a boxed
mutable cell with count0 and Some(Token). Call logs map_call, increments count,
optionally moves Token into an outer retained slot and logs move_token, then
panics or returns the checked target context. Mapper.Drop logs map_drop then
optionally panics; Rust's actual field drop glue emits token_drop when present.
The outer caller destroys a moved token after catching the actual map result,
still inside the selected ambient context, then propagates that result. Success
returns raw bits and appends one slot; a caught panic appends none.

Inherited unwinding runs the real map inside an outer destructor with an inner
catch_unwind. Processing-panic runs it inside the actual panic hook. These
contexts are observed with std::thread::panicking(), not fabricated booleans.
A second locally escaping cleanup panic, or a panic in the panic hook, is the
actual native abort class. No post-abort cleanup is synthesized.

Event strings (numbers decimal except raw bits/write payloads):

- track:<parent>, then optional track_reenter:<raw16hex> after real insertion.
- map:<old_context> before a simple callback's panic or target conversion.
- hash:u64:<raw16hex> before an optional caller-hasher panic.
- source_map:<raw16hex> before returning the caller's text or panicking.
- write:S:<UTF8_hex> or write:C:<UTF8_hex> before sink acceptance/failure.
- map_call:<context>:<count>:<panicking_bool>, move_token,
  map_drop:<count>:<token_present_bool>:<panicking_bool>,
  token_drop:<panicking_bool>. Booleans are literal true/false.

Requests marked abort:true run alone in a child process and may terminate by
actual native SIGABRT. Reference-only flushed PROBE_START/STEP/EVENT records
retain observations up to termination. The response then is
{steps:[completed_steps],abort:{op,class:"abort",events,
last_observed:{interner,slots}}}. last_observed explicitly contains the actual
snapshot immediately BEFORE the failing operation; it does not claim a
postmortem table inspection. Ordinary caught panics always have the complete
post-catch interner/slot snapshots in their steps. An isolated request that
returns normally keeps the ordinary steps response. Rust signal termination and
Go's explicit exit134 normalize only to this shared abort class.
