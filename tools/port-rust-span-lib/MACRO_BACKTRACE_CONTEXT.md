This focused proof calls `Span::macro_backtrace` and its actual returned
iterator in the complete unchanged pinned `rustc_span` crate. It adds no
source-file completion credit. The shared reference selector verifies the
original graph, official compiler, dependencies, 43 upstream tests and external
linkage before every generator run.

The source boundary is `lib.rs:1068-1072`: `outer_expn_data()` clones owned
expansion data, both `lo_hi()` calls can invoke the public `SPAN_TRACK` function,
and the iterator updates its two Copy spans after those calls return. The
native client owns two independent actual `Arc<[Symbol]>` fields. Their strong
counts distinguish the table owner, caller owner, temporary clone, returned
value destruction and final session destruction.

The retained 240 cases cover all four independent debug/overflow profiles,
ordinary operation under construction runtime A, a later runtime B, absent
runtime metadata, inherited unwind caught inside the destructor, inherited
unwind escaping the destructor, and calls during panic processing. Callback
failure uses either a fresh static-string panic or `resume_unwind`, whose
already-raised payload must not invoke another hook. Normal completion,
first/second tracked-read failures, recursive skipping, subsequent iteration
after a caught failure, explicit returned-value Drop, repeated None and final
session release are observed. The Go primary receives each operation's current
`Context`; the iterator is constructed before B or the absent context is chosen.

There are 72 actual native SIGABRT cases. The generator requires SIGABRT for
the selected cases and checks exact native diagnostic text. The fixture retains
native stderr separately. Go compares the complete ordered stdout event prefix
and termination class. It does not infer post-abort state or compare Go's host
exit mechanism with Rust's process signal ABI. Panic filenames are retained
verbatim. The client is compiled with an explicit root-path remap. The named
profile records the execution-root-relative rule (generator ROOT plus `/` to
empty), while rustc receives that checkout's actual prefix. The observer
performs no filename normalization. An isolated regression checks relocation,
including a checkout path containing spaces, and the manifest binds the rule
through the generator's SHA-256.

`lib_macro_backtrace_context_oracle_test.go` verifies every selected production
and tool input hash, fixture hash, raw hash, record count and termination count
before/while replaying child processes. The actual iterator, Shared owners and
current-context tracking descriptor perform the operation. Client helpers
supply the source caller's local destruction and ambient destructor witnesses;
they do not reconstruct the production iterator's owned-data cleanup.

Minimum Go 1.23.12 replay passed with 240 records / 72 abort prefixes (0.858s
test execution). The commands are:

```sh
python3 tools/port-rust-span-lib/generate_macro_backtrace_context.py --check
GOMAXPROCS=2 GOCACHE=/tmp/ferrugo-go123-cache .toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go test -p 1 ./port/rust/compiler/rustc_span/src -run '^TestOriginalMacroBacktraceContext$' -count=1
```

This proof concerns `next` and concrete caller ownership/runtime behavior. It
does not claim every standard `Iterator` provided method, arbitrary raw Go
panic/defer abort timing, thread-local Go storage, or the complete standard
panic runtime.
