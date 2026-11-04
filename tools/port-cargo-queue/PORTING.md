# Cargo queue direct port

The complete `src/util/queue.rs` from Cargo revision
`24de4721844c7a4d839072bab6fb66d41c9bc1f4` maps to
`port/cargo/src/util/queue.go`. The original source SHA-256 is
`f1ca01db50fd9b0534af5b6b1160b6e47de86b4770ef59191a54346ede72eb2a`.
The source is licensed under MIT OR Apache-2.0; retained license texts are
`port/cargo/LICENSE-MIT` and `port/cargo/LICENSE-APACHE`.

`Queue[T]`, `NewQueue`, `Push`, `PushBounded`, `Pop` and `TryPopAll` implement
every upstream declaration and method. A generic ring buffer replaces
`VecDeque`; `sync.Mutex` protects all state. A condition variable wakes bounded
producers. Removable waiter channels implement the timed popper condition
variable without creating helper goroutines. Each push wakes one registered
consumer; a successful pop below the bound wakes one producer, and a drain
wakes all producers. Consumers always recheck the item predicate after waking.
Unbounded pushes can exceed the bound. A zero bound prevents bounded pushes
from completing, including after a drain, matching the original source.

`queue_duration.go` adapts the entire Rust duration domain: uint64 whole seconds
plus fractional nanoseconds. `NewDuration` normalizes nanoseconds and panics on
seconds overflow. `DurationFromGo` is an explicit lossless adapter, rejecting
negative input. The `Seconds` and `Nanoseconds` accessors mean Rust `as_secs`
and `subsec_nanos`. Long timeouts use successive waits of at most one day, so
Go's signed nanosecond timer range does not truncate or overflow a Rust timeout.
An already-queued item is returned even with a zero timeout.

The implementation uses only Go's standard library. Production code does not
call Rust, Cargo, external processes or the development oracle. Queue values
must not be copied after first use, matching the usual Go mutex rule. No
fairness guarantee is added: native thread/goroutine scheduling may differ.
As upstream assumes, blocked bounded producers cannot be canceled.

## Validation

Run from the repository root:

```sh
GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-cargo-queue
```

`oracle.rs` includes the unchanged pinned Rust source. The runner verifies its
source hash, compiles it with the installed Rust toolchain, and compares exact
outputs against the Go API for 100,000 deterministic operations across bounds
0, 1, 2, 17 and `usize::MAX`. Operations include unbounded pushes above capacity,
bounded pushes with available capacity, zero-timeout pops and FIFO drains.
Additional scenarios check blocked producers, pop wakeups with the maximum
Rust duration, zero bounds and duration normalization. Direct original
`Queue::pop` operations cover queued items with a zero timeout; inspection of
the original method identifies its use of standard
`Condvar::wait_timeout_while` predicate rechecking. Separately, the Go unit test
waits for a registered `Queue.Pop` consumer, holds the queue mutex past timer
expiry, inserts an item, then releases the mutex and requires that item to be
returned. This controlled reacquisition race is a Go-only regression test. No
forced Rust mutex-reacquisition interleaving is claimed: portable standard Rust
cannot observe registration inside the unchanged queue method, and a timing
guess would produce false failures on delayed CI workers. All 100,000 trace
operations call the unchanged original Queue methods; the Rust harness has no
test-only primitive probes or queue-internal instrumentation.

The same runner executes Go queue tests with `-race -count=10`, then `go vet`.
Those tests cover concurrent consumers/producers, FIFO against a separately
maintained model, bounded wake-one and drain wake-all behavior, nil items,
reference release, repeated empty notifications without extending the timeout,
full-range durations, timeout waiter cleanup and goroutine retention. The
uncancelable zero-bound case runs in an isolated subprocess. Actual commands,
toolchain/host versions, output hash, case count and results are retained in
`validation.json`. Wall-clock scheduling is deliberately not compared byte for
byte; assertions use generous upper bounds and synchronize the relevant
interleavings. This validates this utility, not the full Cargo program.

For CI, `go run ./tools/port-cargo-queue -check` performs the original-source
comparison and verifies the retained output hash without changing evidence or
repeating the separately configured Go race/vet jobs. The optional
`FERRUGO_QUEUE_ORACLE=1 go test ./port/cargo/src/util -run TestQueuePinnedUpstreamOracle`
delegates to that same command; ordinary tests require no Rust installation.

`FERRUGO_GO` and `FERRUGO_RUSTC` select executables. Without overrides, the
runner uses the local `.toolchain` binaries if present, then normal `PATH`
lookup. `FERRUGO_CARGO_SOURCE` selects the Cargo checkout directory (default
`.toolchain/upstream-cargo`); its queue source must match the full pinned hash.
Explicit invalid/empty overrides fail without falling back to another tool or
source. Run commands from the repository root. The reference harness uses
only Rust's standard library and does not need a Cargo executable.
