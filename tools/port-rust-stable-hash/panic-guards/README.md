This caller rebuilds the unchanged official `rustc-stable-hash 0.1.2` dependency
through the integrity-checked development reference builder. Run
`python3 tools/port-rust-stable-hash/panic-guards/generate.py --check`.
`FERRUGO_RUSTC`, `FERRUGO_CARGO`, `FERRUGO_GO` and `CARGO_HOME` retain the builder's
portable executable/cache selection contract, including executable symlinks.

The native caller has non-Copy hasher state, a state destructor and actual local
cleanup guards. It raises from borrowed write/clone and consumed finish operations
under normal, already unwinding and processing-panic contexts. Finish additionally
raises from its required state cleanup. Twelve cases compare event order, actual
panic payload dynamic type, source location, unwind/backtrace flags, local/state
Drop contexts and exit status. The selected compiler's real cleanup-abort hook is
recorded rather than assigned a guessed source path. Its metadata is passed as
explicit borrowed profile information to the Go source callbacks and runtime.

`ownership.go` forwards already raised payloads with `ResumeUnwind`. Its proven
compiler-generated cleanup boundary uses `AbortCleanup` after a second escaping
panic. This support protocol does not claim an entire Rust panic-runtime port.
