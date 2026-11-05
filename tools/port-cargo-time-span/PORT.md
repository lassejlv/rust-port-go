# Cargo time-span parser

This is the complete 73-line `src/util/time_span.rs` from Cargo
`24de4721844c7a4d839072bab6fb66d41c9bc1f4`, Git blob
`8ef61ed1aa9b32c62ade1a7462123443dd66f389`, SHA256
`185b6ee6c972b98a19ca7577482777c9e706b71869e6a6d3008d5a9a1c36f130`.
It maps to `port/cargo/src/util/time_span.go`; both original inline tests and
all 21 assertions remain in `time_span_test.go`. Parent inventory acceptance is
separate. No claim about the full compiler or Cargo resolver follows.

`MaybeParseTimeSpan` returns the exact Option result as `(Duration,bool)`;
`ParseTimeSpan` returns the same Duration or the real owned-message error.
Contextual primaries take explicit `TimeSpanConfig` and the current
`rustfmt.Context`. The two source functions do not use debug assertions;
ordinary multiply overflow independently follows checked or wrapping profile.
Inputs require valid UTF-8 as Rust `&str` does. Decimal digits are ASCII only;
there is at most one optional ASCII space; unit spelling is exact and
case-sensitive. Months retain the source average 2,629,746 seconds.
The existing queue Duration represents all u64 seconds and fractional nanos;
this parser returns nanos 0 and never converts through Go `time.Duration`.

The overflow site is an actual static-string panic at source 37:30. A caller
may supply `TimeSpanPanicLocationsAtPinnedRevision(filename)`; that exact file
spelling is borrowed, and missing metadata remains unlocated. Processing-panic
operations abort at the actual checked multiply before local cleanup. Ordinary
and ambient unwind keep the inherited current runtime metadata.

## Scoped dependencies

`port/support/rustanyhow` implements the used concrete
`anyhow 1.0.103 Error::msg(String)` branch, from the exact root-locked archive
SHA256 `2a4385e2e34eb35d6b3efe798b9eb88096925d87726c0798709bf56d9ed84af3`.
The source bodies are error.rs construction/downcast, wrapper.rs
MessageError<String>, and fmt.rs display/debug. Every archived regular file
and its original license is verified before reference compilation. This is
supporting used-contract code; no entire anyhow source file/crate credit.

The actual owned String cell, shared/mutable downcast reference, payload-type
distinction from `&'static str`, owner move, consuming String downcast and
sourceNone are preserved. Copies of a Go owner descriptor are aliases, not
Rust Clone; moving or closing invalidates every alias. String/Backtrace borrows
obey source lifetimes. The primary returned String has its real byte buffer, length/capacity and
scoped push/reserve/reserve_exact methods, with actual fmt::format initial
allocation and Vec<u8> growth. Text-only Go conversions remain conveniences.
No complete String API, allocator ABI or arbitrary anyhow contexts/payloads
are claimed by this scoped dependency. Both message and backtrace cells retain within-process
identity across a move; consuming downcast retains the String byte-buffer
identity. No addresses are compared between languages.

Display follows outer write! delegation and ignores outer width/precision
for the message. Alternate Debug delegates to actual Rust str Debug;
ordinary Debug displays the message and, only for a captured backtrace,
formats the exact capitalized/header/trimmed suffix. Fallible mutable formatter
primaries retain actual C/S chunks, early failures and caller Context. Neutral
Unicode17 string Debug is used explicitly, independent of Go Unicode tables.

Backtrace capture is an explicit complete source-runtime operation receiving
THIS caller Context. The named default profile selects disabled backtraces
and does not claim to infer Rust TLS/environment or produce a native stack.
Captured records must come from a truthful runtime/profile witness. The native
producer captures actual original error backtraces and its own formatter
output from that SAME error; Go consumes those exact captured records.
No Go-created Rust stack trace or complete std Backtrace implementation is
used or claimed. The actual SDK disabled Display is `disabled backtrace`.

## Reference and replay

The native client includes the complete original file unchanged. Its
`CargoResult<T> = anyhow::Result<T>` alias is the actual source declaration
from src/util/errors.rs:11. The exact dependency archive is materialized as a
verified path dependency, with a locked Cargo workspace and no replacement
source body. Each of four independent debug/overflow profiles compiles/tests
sequentially with SDK 1.99.1 (`58cf8f9c3`, Unicode17). Both original tests pass in
each profile. All subsequent builds use `--offline --locked`.

```sh
python3 tools/port-cargo-time-span/prepare.py --fetch
python3 tools/port-cargo-time-span/generate.py --output-dir /tmp/time-span-producer
FERRUGO_TIME_SPAN_PROOF=/tmp/time-span-producer go test ./port/cargo/src/util -run '^TestTimeSpan' -count=1
go test -race ./port/support/rustanyhow
python3 -m unittest discover -s tools/port-cargo-time-span -p 'test_*.py'
```

Retain the producer's manifest.json, time-span.jsonl.gz and raw time-span.jsonl
verbatim. Captured stack addresses/filenames are inherently producer-specific;
fresh native runs use `--output-dir`, not normalization or replacement of the
historical golden. Ordinary Go replay can use retained historical records.
Explicitly empty/missing/nonregular selections fail without fallback; pin,
archive, source/replay bindings, CRC, raw/gzip hashes and counts are checked.
Ambient subprocess requests carry the already-validated immutable row directly;
children do not independently select or reread a different fixture.

Native caller witnesses exercise ordinary calls, caught inherited unwind and
actual panic-hook processing. Source result/None/error, full u64 duration,
String type/downcast/cell/buffer relations and formatter failures are compared.
Source termination is genuine SIGABRT; Go exit134 represents the unrecoverable
class, without signal/ABI or post-abort state claims. All callbacks are explicit
runtime operations; raw Go callback-local defer timing is not a Rust runtime.
No Go test/compile acceptance was claimed while the parent build hold applied.


## Owning String allocation boundary

`port/support/ruststr/owned_string*.go` is NEW scoped compatibility code; old
slice helpers are unchanged. It preserves the concrete String cell, byte
buffer, len/capacity, sufficient-capacity no-allocation, amortized/exact
reserve, UTF8 push_str/push and consuming buffer transfer. Error moves keep the
same boxed String cell; consuming downcast moves the buffer/capacity into a
new String header. Source borrows must end before mutation/move/destruction.
Go descriptor copies do not create source ownership. Growth may choose a new
Go allocation; the source allocator's optional in-place realloc choice is not
an ABI promise. Native comparisons require stable pointer identity only when
source capacity guarantees no allocation, and across ownership transfer.

The official SDK 1.99.1 rust-src archive is pinned by SHA256
`e485a88c45db546b73571df7bebfcb4454ba4fb552c492e25fc5b00eeb5bf22d`.
`sdk199.json` names the actual core fmt template-capacity scan, alloc fmt,
String, Vec and RawVec source files and hashes. These bytes are reverified
before native builds. This is explicit support, not std source-file credit.
The source template has one trailing runtime argument, so its literal prefix
is doubled exactly by Arguments::estimated_capacity; all actual string writes
then use the translated String Writer with source growth. Native full-file
errors observe their original capacity, reserve0/reserve13, ASCII/non-ASCII
pushes, final capacity and same-buffer consuming downcast. Safe usizeMAX
reserve failures preserve original state and use the actual SDK raw_vec panic
site 28:5, including inherited and processing contexts without huge allocation.

## Validation and first publication

The minimum Go 1.23.12 comparison ran in an isolated coherent checkpoint35 base
with only the new TimeSpan/String/Anyhow files overlaid. All 16,676 actual-source
parser/formatter/type/downcast/len-capacity/ownership/context observations,
including 8 actual native abort records, matched in 0.461s. Source Duration spans
all 64-bit seconds. The same selected producer passed race in 2.631s; affected
package vet passed. Eleven fast Python integrity tests passed, including cache ownership, selected compiler overrides and extracted-source tampering.

The shared String primitive independently matches 1,344 genuine std scripts /
45,696 public operations under BOTH SDK 1.99 and 1.101, all four assertion/
overflow profiles. Exact Clone/CloneFrom, mutation, capacities, reserve failures
and within-process buffer identity relations match. Minimum-Go replay 0.414s,
String race 3.456s, Anyhow race 1.011s, and vet passed. The standalone tool is:

```sh
python3 tools/port-cargo-time-span/generate_string.py --check
go test -race ./port/support/ruststr -run '^TestOwnedString'
```

`string-manifest.json` binds the owning String helper and test bytes. The parser
producer manifest binds its own full used implementation/replay closure.
`VALIDATION.json` records the bounded review evidence. Exact final source-byte
lists and authoritative ledger credit belong to the parent integrator.

The first validated canonical corpus was copied byte-for-byte from the actual
fresh producer that passed Go/race/vet. Earlier untracked pre-acceptance
prototypes (before the owning String closure) were preserved outside the
repository in `/tmp/ferrugo-time-span-pre-owning-prototype`. Future native runs
must use fresh output directories when captured stacks differ; they cannot
replace this retained historical observation corpus implicitly.

The preparer rejects symlinks at its owned cache root, ownership marker and
descendants; a trusted enclosing `.toolchain` alias remains usable in isolated
checkouts. Explicit paired `--rustc-199` / `--rustc-1101` selections are checked
before native work. Tool-only integrity changes require a genuine producer
refresh; they do not alter the frozen Go ownership API.

The final cache-ownership/compiler-selection hardening was followed by genuine
regeneration under both SDKs and all four profiles. Both retained fixture files
remained byte-for-byte identical to the inputs that passed the minimum-Go
replay, race and vet gates. Only source-binding manifest entries changed.

Foreign pre-existing ownership markers are rejected before any child directory
is created or any file is written. The regression replaces mkdir with a hard
failure and verifies the complete foreign tree remains untouched. This final
review correction was followed by genuine native regeneration; both fixture
files remained identical to the already replay/race/vet-validated data.
