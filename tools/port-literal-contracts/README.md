# Literal-escaper source contracts

The shared original-source runner is
[`../port-lexer-debug/generate.py`](../port-lexer-debug/generate.py). It compiles
the checksum-verified complete `rustc-literal-escaper 0.0.8` crate, using its
exact pinned registry archive. No lexer dependency is introduced into the
production literal port. Sharing is limited to validation machinery.

`debug.tsv.gz` records every native writer operation, error and formatter reuse
for all derived Debug variants and their used NonZero primitives.
`owned.tsv.gz` records all seven consuming callback entry points, including all
eight `check_for_errors` modes, owned and borrowed FnMut receivers, call/Drop
panics, field cleanup, ambient unwinding and isolated double-panic aborts.
`processing.tsv.gz` records panic-hook aborts before callback cleanup.
Each manifest binds original source, exact dependencies, validation clients and
Go adapters. Baseline Go tests need only these committed files.

```sh
python3 tools/port-lexer-debug/generate.py --scope literal --check
python3 tools/port-lexer-debug/generate.py --scope owned-literal --check
python3 tools/port-lexer-debug/generate.py --scope processing-literal --check
```

See the shared `PORT.md` and `VALIDATION.md`. The pre-existing escaping,
exhaustive scalar, nine original tests and 27 benchmark workloads remain in
`tools/port-literal-escaper`; these new fixtures complement that evidence.
