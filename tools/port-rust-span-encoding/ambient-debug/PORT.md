# Immediate-abort Debug regression proof

This separate corpus closes a review finding in the existing Span port. A
primary formatter carries explicit ambient Rust panic state. Internal getter or
constructor failures must raise at their failure site: recovering later at the
Debug boundary permits local cleanup before a panic-in-hook abort, which Rust
skips. It adds no source credit and preserves the large frozen span corpus,
request builder, generator and manifest.

The unchanged original source is prepared by the parent source-verified builder
in a separate ignored workspace. The small development wrapper delegates to
actual Span/SpanData/OrdSpan Debug and observes real normal calls, outer Drop
with inner catch_unwind, and a real panic hook. The 1,344 pure-input cases cover
all four assertion/overflow profiles, nil-session and bad-index failures,
SpanData interning construction, inline/parent/overflow spans, pretty/compact,
independent character/default string sinks and first-write returned errors.

Records preserve writes, accepted prefixes, tracking order/panicking state,
caller scope destruction, returned fmt errors, caught panic classes, formatter
state and post-catch interner rows. Processing failures really SIGABRT and skip
scope destruction. A separate reference_abort_lock field records the actual
original lock at SIGABRT; it is native-only evidence, not a fabricated Go result.
The Go replayer compares the response and separately tests its panic-raise site
with a structural witness that inspects the Go mutex before cleanup.

The Linux64 SIGABRT audit publishes an atomic pointer to the real session. Its
handler performs no allocation, formatting, TLS access or stdio locking. It
calls the original Lock::try_lock nonblocking primitive path and raw libc write
only. Bad-index panic-in-hook cases observe the still-held interner lock;
missing-session/inline arithmetic/constructor failures observe a free lock.
The handler returns and actual SIGABRT termination proceeds. This proof is
explicitly limited to the recorded Linux64 reference profile; unsupported
platforms fail instead of substituting an observation.

Source files, exact fragments and registry expansion verification are inherited
unchanged from the parent; all staged inputs and helper bytes are frozen and
checked. The parent manifest is also bound as an immutable input. Expected
responses and lock observations come only from Rust. Neither the signal probe
nor the source/session facades provide a production source-map implementation,
full surrounding crate port or production Rust fallback.

```sh
python3 tools/port-rust-span-encoding/ambient-debug/generate.py -check
python3 -m unittest discover -s tools/port-rust-span-encoding/ambient-debug -p 'test_*.py'
go test -race ./port/rust/compiler/rustc_span/src
```

Generation omits -check and preserves existing gzip bytes when decoded content
matches. Check mode mutates neither fixture nor manifest. Executable/cache
selection follows the parent tool, including bare PATH names, explicitly invalid
override rejection, locked/offline Cargo and disabled Git lazy fetch.
