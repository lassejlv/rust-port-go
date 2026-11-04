This oracle independently executes the unchanged pinned
compiler/rustc_data_structures/src/binary_search_util/mod.rs. Its Git blob is
1c6e227cec40870c0288f60270f609aeb4ebdc8e and its SHA-256 is
6ed8ff0a143e0e4badf23ccd92e483fb14597584c0665cee582882597f086fc9 at Rust
revision 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The actual source has 27
physical lines. The generator checks both identities and copies the exact file
into an ignored workspace before compiling the wrapper.

The Rust wrapper includes that original file. It supplies an actual Fn move
closure owning an environment with Drop and Cell state; no search loop is
copied into the wrapper. Extracted keys are non-Copy, own a Drop implementation,
and independently implement lt, ne and le. Operators mutate only order-neutral
metadata, preserving the ordering of the immutable key values.

The native trace proves:

- Every extracted temporary drops after its comparison, including panic cleanup.
- Fn state is borrowed in one capture cell and drops even on empty input/miss.
- Operators borrow the original query cell and the extracted temporary cell;
  subsequent Drop observes the same temporary mutations and address.
- Extractors receive original data cells. Per-data Cell changes and a returned
  subslice mutation prove shared input/result identity.
- The borrowed query remains caller-owned and live after success or catchable
  failure; queryDrops is sampled before its caller-owned destructor runs.
- Extractor, each operator, temporary Drop and Fn Drop can panic independently.
  Failure ordinals come from observed native baseline calls.
- Actual outer destructors with and without an inner catch and an actual Rust
  panic hook distinguish local cleanup from inherited/processing panic state.
  Double panics and escaping outer-cleanup panics run in isolated processes.

Safe Rust vec![(); usize::MAX] constructs the full-domain ZST input without a
large allocation. The original search returns a borrowed full-length slice.
The largest successful 64-bit case performs 131 extraction/operator/temporary
Drop cycles, retaining the original std partition_point probe order. Go uses
the validated inhabited unit-type IndexRaw representation for the same logical
length. An ordinary zero-sized array embedded beside a nonzero byte also
checks slice-backed pointer identity through both owned entry points. This
regression exposed a fresh-allocation projection in the slice convenience;
the production owner now projects nonempty ZST results into the original slice.
Empty results are classified as empty rather than falsely claimed to
borrow input. Nonempty ZST results check length and representative alias.

Requests specify id, sorted values/query, ordinary or ZST storage, decimal
usize length, ambient context and six independent failure ordinals. Responses
contain exact events, catchable outcome/borrowed view, and partial Fn/query/data
state. Aborted processes retain every event flushed before SIGABRT. The JSONL
fixture contains native expectations only; it does not run or simulate Go to
produce those expectations.

Go replays storage and slice owned APIs separately and also exercises their
normal-context conveniences. Full-domain ZST uses storage directly without
AsGoSlice projection. Compatible cases separately compare the legacy scalar
probe/operator trace. Existing comparator/operator fixtures and manifests stay
unchanged, and their exact hashes are included in the owned manifest.

Translated Go panic sites use Context.Panic. Raw Go callbacks retain their own
callback-local defer/recover timing and are a separate convenience boundary;
the native corpus makes no claim of transparently intercepting those callbacks.

Run with the project tools or FERRUGO_RUSTC/FERRUGO_RUST_SOURCE overrides:

    python3 tools/port-rust-binary-search/owned/generate.py
    python3 tools/port-rust-binary-search/owned/generate.py --check
    go test -race ./port/rust/compiler/rustc_data_structures/src/binary_search_util
    go vet ./port/rust/compiler/rustc_data_structures/src/binary_search_util
    FERRUGO_RUN_BINARY_OWNED_ORACLE=1 go test -run '^TestBinarySearchOwnedOracleRegeneration$' ./port/rust/compiler/rustc_data_structures/src/binary_search_util

The development reference compiler profile is recorded verbatim. Git lazy
fetching is disabled and no Cargo dependency or network fetch is needed.
Executable names on PATH and explicit paths are supported. Regeneration reuses
existing valid gzip bytes when decompressed native output is unchanged, making
checks independent of compressor implementation differences.

Bounds are 4000 cases, 128 non-ZST elements, 2048 native/Go events per case,
2 MiB per native child output, 10 seconds per isolated child, and 64 MiB for the
decompressed Go fixture. Development abort proofs disable core dumps. The
retained 64-bit ZST traces run only on a matching Go word width; other targets
have no runtime validation claim. Parent Rust MIT/Apache licenses cover the
included original source; this reference helper earns no extra source-file or
whole-project completion credit.
