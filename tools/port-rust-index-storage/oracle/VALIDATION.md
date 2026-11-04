# Index storage validation

The final corpus contains **150,068 original-source requests and 189,668
steps**, covering eight independently compiled profiles: normal/nightly ×
checked/wrapping arithmetic × enabled/disabled debug assertions. The reference
is the complete unchanged pinned storage modules, full `idx.rs`, complete index
macro crate, and real nightly serialization/hash dependencies. Its compiler is
the recorded Rust 1.99.1 development compatibility profile, not a claim that
the entire Rust 1.101.0 compiler has been bootstrapped.

| Request family | Records |
| --- | ---: |
| Vector, slice, custom conversion, iterator, and allocation scripts | 70,928 |
| Option maps and nested trait dispatch | 29,120 |
| Full-width zero-sized storage | 2,016 |
| Inversion, assertions, and partial mutation | 1,244 |
| Inversion diagnostic aborts | 80 |
| Formatter flags, writer methods/errors, state, and reuse | 43,296 |
| Owned closure/range/element double-panic aborts | 24 |
| Nightly codec specializations and Extend overrides | 3,360 |

There are 40,232 caught step panics and 104 isolated unrecoverable abort
records. Callback traces compare source thread panic state as well as ordering.
The 89 operation labels include source methods and selected caller scenarios;
they are not an extra upstream API or source-file completion count.

Concrete discrepancies found during independent review and closed before this
snapshot include borrowed element/drop addresses, shared zero-sized view
cells, iterator allocation reuse, default clone replacement and tail reserve
ordering, owned closure/range destruction, source layout capacities, independent
element `ne`, nested Option's default `ne`, destructor double panics, diagnostic
formatter aborts, and the complete fallible formatter/writer surface. The final
corpus has no known unmatched supported observation.

The formatter cases compare each write string/character, all publicly visible
options forwarded to callbacks, pretty indentation and multiline splitting,
normal/hex numeric Debug, sink failure, callback-local cleanup, and options
after return. Focused cases format again through the **same** Formatter after
a caught writer error. They preserve Rust's zero-padding fill/alignment state
on errors and pretty-child formatter isolation. Diagnostic formatting also
distinguishes immediate abort during panic-message processing from ordinary
unwinding, where local cleanup runs first.

Iterator cases call Nth/NthBack/Count/Last on every exposed family. Internal
catch-and-resume scripts verify constructor/drop panic progress, independent
Enumerate counters, bulk Vec skip drops versus item-wise Drain defaults,
consuming cleanup, and owned Last return-place behavior. Zero-sized raw iterator
shortcuts are exercised through `usize::MAX`, including a maximum skip count,
without traversing an enormous mapped callback sequence.

Validation completed:

- Actual **Go 1.23.12** complete storage test package, default and
  `ferrugo_rustc_nightly`, including the original five translated tests and all
  differential records applicable to each feature build.
- Actual Go 1.23.12 generator tool tests and default/tagged `go vet`.
- Go 1.27.3 tagged **race** run for storage plus generator tests.
- Original Rust normal/nightly reference builds: all five untouched upstream
  vector tests, storage proof, actual memory codec round trip, and macro
  expansions passed.
- Actual Go 1.23.12 **bare PATH** selections (`FERRUGO_GO=go`,
  `FERRUGO_RUSTC=rustc`, `FERRUGO_CARGO=cargo`) rebuilt all eight reference
  profiles with `-check`; decoded output matched, and the committed Go 1.27.3
  gzip and manifest bytes were retained unchanged.
- Ten invalid/empty tool, empty flag, and positional-argument CLI cases failed
  without changing either corpus or manifest.
- Five isolated registry-guard tests cover accepted exact bytes, changed
  expanded source, changed/missing archive, and dependency pin mismatch.

The ordinary baseline verifies committed artifact and input hashes and requires
no Rust compiler, Cargo, Git cache, registry cache, or network. The live opt-in
rebuilds the originals and never uses Go results as reference output. The
reference build and oracle wrappers earn no additional Go source credit.
