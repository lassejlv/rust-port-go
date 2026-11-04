# Shared streaming debug builders

This is a supporting adapter for the Rust `std::fmt` contracts used by selected
source ports. It grants no completion credit to a standard-library source file.
The native reference profile is rustc 1.99.1 on Linux x86-64; tests also retain
the native observations so ordinary Go testing requires no Rust installation.

`Value` is `func(io.Writer) error`. `Field` contains a name and value callback;
`Entry` contains key/value callbacks. The batch functions are `WriteStruct`,
`WriteTuple`, `WriteList`, and `WriteMap`, each taking its writer, names where
applicable, a `pretty` boolean and the corresponding slice. All write errors
and callback panics propagate without replacement or recovery.

The incremental forms are `BeginStruct(...).Field(name, value).Finish()`,
`BeginTuple(...).Field(value).Finish()`, `BeginList(...).Entry(value).Finish()`
and `BeginMap(...).Entry(key, value).Finish()`. They remember the first returned
formatting error and skip subsequent rendering. A caller still evaluates method
arguments after an earlier error. This distinction matters for expressions such
as tracked span getters: constructing the builder writes its name first, then
each getter runs immediately before its `Field` call, even when that call has
become inert. Precomputing all getters or evaluating them inside skipped render
callbacks changes source behavior. Batch APIs are for already-evaluated values.

`WriteString` retains empty writes and returns `io.ErrShortWrite` for invalid
short successful Go writes. `WriteChar` preserves the independent optional
`WriteChar(rune) error` sink override; otherwise its default is one UTF-8 string
write. `NewIndentWriter` supplies the pretty PadAdapter: separate four-space
string writes at every nesting level, empty child string suppression, Unicode
character forwarding and newline tracking. Its state changes only after a
required indentation write succeeds; a callback that catches an indentation
error and retries must receive indentation again.

Numeric formatting flags and mutable formatter state do not belong to these
builders. A source wrapper passes its original formatter pointer to compact
callbacks and copies only its options into each pretty child with the supplied
indentation writer. The semver adapter demonstrates this without introducing
dependency-specific options or generic aliases into the shared package.

`testdata/native.rs` independently invokes the installed Rust standard-library
builders. Its 1,284 records cover named/anonymous tuples, structs, lists, maps,
empty and nonempty values, nested maps, both writer dispatch profiles, every
failure position, interleaved argument/render side effects, and callbacks that
catch an error then retry string or character writes. Go replays both incremental
and batch forms; batch comparisons omit only the already-completed argument
evaluation events. The extracted semver wrapper must additionally keep its
10,500 core Debug records, 33,982 core value/trait records and the Cargo extension
corpora unchanged.

To reproduce the scoped reference without changing retained evidence:

```sh
FERRUGO_RUSTC=rustc python3 port/support/rustfmt/testdata/generate.py --check
go test -race ./port/support/rustfmt
```

The generator preserves an existing valid gzip representation when its decoded
output agrees. Full source/output hashes and the exact reference compiler are
recorded in `testdata/manifest.json`. This builder subset does not claim the
complete `std::fmt` API or ABI.
