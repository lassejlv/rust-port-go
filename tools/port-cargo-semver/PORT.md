# Complete Cargo semver extension source ports

Both files are pinned to `rust-lang/cargo@24de4721844c7a4d839072bab6fb66d41c9bc1f4`:

| Source → mirrored Go target in `port/cargo/src/util` | Git blob | SHA-256 |
| --- | --- | --- |
| `src/util/semver_ext.rs` → `semver_ext.go` | `39d0f5659d6e68115e4b5d07eea7a840b290297c` | `ec29b287daf33cf7ce867a7e0981df607e7e84fa4bc2984c7f42f8c64468d3d0` |
| `src/util/semver_eval_ext.rs` → `semver_eval_ext.go` | `4cb09e9fb53d3f12212a0abcd712f2e11a1d181e` | `49d93e6907b452ac732f559aeb4a96a7eca720c3a33240be7e23d47686bcd0c5` |

The complete files, including their public traits, concrete implementations,
enums, mutators, matching algorithms, derived behavior, and all eleven inline
tests are translated. They use the complete pinned Go `semver` dependency at
`port/dependencies/semver/v1_0_28/src`; production launches no reference compiler
or Cargo process. Source licensing is MIT OR Apache-2.0, retained in
`port/cargo`; the semver crate retains its own MIT/Apache notices.

## API and semantic mapping

`VersionExt` requires `IsPrerelease()` and `ToReq(semver.Op)`. `VersionAdapter`
provides the original implementation for `semver.Version`. The source defaults
map to `VersionToExactReq(VersionExt)` and `VersionToCaretReq(VersionExt)`.
These functions first dispatch optional `ToExactReq()` or `ToCaretReq()` methods
on the actual implementing object. Otherwise they call that object's `ToReq`
with Exact or Caret, respectively. Arbitrary implementing types, individual or
combined overrides, callback order, returned requirements, and callback panics
are supported; there is no concrete-version cast in these defaults.

`VersionReqExt` requires `MatchesPrerelease(semver.Version)` and is likewise
implementable by arbitrary types. `VersionReqAdapter` supplies the source
implementation for the concrete dependency type. `VersionReqMatchMode` exposes
`VersionReqDefault` and `VersionReqPrerelease`, value-copy semantics, Clone and
exact derived Debug names.

`OptVersionReq` is a tagged Go value with exported `Kind`, `Version`, and `Req`.
The active payload is defined by its kind:

| Rust variant | Go kind / constructor |
| --- | --- |
| `Any` | `OptVersionAny` / zero value |
| `Req(VersionReq)` | `OptVersionRequirement` / `FromVersionReq` |
| `Locked(Version, VersionReq)` | `OptVersionLocked` / `LockedVersionReq` |
| `Precise(Version, VersionReq)` | `OptVersionPrecise` / `PreciseVersionReq` |

`exact` and `lock_to_exact` map to `ExactVersionReq` and
`LockToExactVersionReq`. All methods are retained: `IsExact`, `LockTo`,
`PreciseTo`, `IsPrecise`, `PreciseVersion`, `IsLocked`, `LockedVersion`, and
`Matches`. `String` and fallible `WriteDisplay` implement Display; `Clone`,
`Equal`, `Hash`, and the streaming/text Debug APIs implement the corresponding
derives. `FromVersionReq` also maps
the original `From<VersionReq>` implementation.

Inactive payload fields are ignored, reflecting the Rust enum rather than
ordinary whole-struct equality. Clone owns independent requirement storage.
Constructors copy slice-bearing requirements to reproduce transferred Rust
ownership without retaining mutable caller aliases. Direct literals and the
exported payload fields require Rust-equivalent alias discipline from Go
callers. Version getters return borrowed inner pointers that must be treated
as read-only, with no mutation of the enum while the borrow is in use. Go does
not enforce Rust lifetimes or exclusivity; concurrent mutation is unsupported.

Hash emits the original enum discriminant before active payloads, in original
field order, and delegates to the dependency's full Rust Hasher adapter. Custom
primitive writes and their panics are retained. The required Go interface is
`semver.Hasher.Write([]byte)`; optional primitive methods supply Rust's
overridable defaults. Rust Hash does not call Hasher::finish, so the production
interface does not require it. The native stable oracle observes primitive
overrides and string byte/sentinel writes; `write_str` itself is unstable in
the pinned reference toolchain, and the dependency documents that extra Go
adapter hook separately.

The source's distinct matching rules remain intact. Req matching ignores build
metadata through normal semver requirements. Locked matching compares the full
version, including build metadata. Precise matching permits arbitrary build
metadata when the requested version has none, and requires exact metadata when
the requested version supplies it. `PreciseTo` can select a version outside the
original requirement and retains that original requirement for Display;
`LockTo` asserts a match before mutating. Any admits every version, including
prereleases; locking Any preserves STAR as its original requirement. Exactness
is determined by the source variant and original comparator shape, rather than
the presence of a selected precise version.

The complete formatter path preserves more than the resulting text.
`WriteDisplay(io.Writer, semver.FormatOptions)` writes Any once and delegates
other variants directly to the dependency requirement formatter. Outer width,
alignment, precision, sign, and fill do not add padding/truncation absent from
the source. Write boundaries, the first error, and the accepted output prefix
are preserved. The Go `fmt.Formatter` bridge prevents Go's default Stringer
padding/truncation; ordinary verbs use Display and `%#v` uses compact Rust Debug.

`WriteDebugFormatter(*semver.DebugFormatter)` is the primary Debug API. It
delegates to the validated standard tuple builder and concrete field formatters
with the actual mutable outer formatter. Compact fields share that state;
pretty fields use isolated copies and indentation adapters. `WriteDebug` is the
value-options convenience wrapper; `RustDebugWith` renders that streaming path
into a string. Options retain pretty/decimal/lower/upper hex, width, precision,
fill, alignment, sign, and zero-padding behavior. This includes real empty
writes, optional `WriteChar(rune) error` overrides through nested indentation,
and the source's outer fill/alignment mutation when compact numeric formatting
fails. Unit matching-mode variants support the same Debug APIs while ignoring
irrelevant options, as the source does. No one-chunk buffered substitute is
used for streaming Debug.

The format corpus adds 4,164 Display/Debug records. Three independent native
Debug probes add 8,668 records each for default string writes, overridden
character writes, and before/after outer formatter state. All writer failure
positions through one beyond the complete stream are included. The probes
cover every enum variant, empty/multiple/raw requirements, partial Option
fields, maximum numeric components, mode unit variants, and fourteen caller
format patterns. Their unchanged probe sources and newly generated results
are bound by the same pinned-source manifest.

## Complete RFC 3493 algorithm and overflow profile

The complete `matches_prerelease` algorithm is `MatchesPrerelease`. Every
private helper is preserved with the `semver` prefix: implementation dispatch,
exact/greater/less comparison, both partial-bound filling functions, and exact,
tilde and caret prerelease bounds. Raw public Comparator states are supported,
including patch-present/minor-absent combinations that the parser cannot
produce. Comparator traversal and short circuit order are retained.

The source explicitly records unresolved RFC 3493 upper-bound behavior. This
port preserves that current behavior; it does not repair or reinterpret the
RFC's semantics. Precedence and identifier comparisons come from the exact
dependency port, with no simplified prerelease comparison or string sorting.

The default entry points use `SemverCheckedOverflow`, reproducing the original
checked/debug `u64 + 1` operations. `MatchesPrereleaseWithOverflow`,
`OptVersionReq.MatchesWithOverflow`, and `LockToWithOverflow` also expose
`SemverWrappingOverflow`, matching the original Cargo files compiled with
overflow checks disabled. These profiles affect each upper-bound increment at
its original evaluation point, including early returns that avoid overflow.
The added selection API is a compatibility adapter, not another source file.

The dependency's Op enum is non-exhaustive. The source's wildcard match reaches
`unreachable!()` for unimplemented operators; the Go extension retains that
panic and its short circuit position. Out-of-range Go enum casts have no safe
current Rust value; the Go regression checks the source branch directly, and
the native oracle never constructs invalid Rust enum discriminants.

Allocation failures and Go memory/borrow enforcement are not cross-language
claims. Normal matching scans comparators in source order; temporary bounds
clone concrete comparators. Requirement cloning and hashing visit their
comparators in order. Prerelease comparison complexity follows the dependency.

## Unchanged original-source oracle

The development generator reads both complete files and the original root
Cargo.lock from their pinned Git objects, verifies object IDs and SHA-256, and
materializes their unchanged bytes in a temporary workspace. `util/mod.rs`
provides only the original module layout. The source's inline tests compile
unchanged: two from semver_ext and nine from semver_eval_ext, all eleven run on
every generation/check. The Go test mirrors retain all original requirements,
version arrays, expected outcomes, and loop order, including the first file's
28-case prerelease table.

The exact dependency is `semver 1.0.28`, archive SHA-256
`8a7852d02fc848982e0c167ef163aaff9cd91dc640ba85e263cb1ce46fae51cd`, matching
the original Cargo.lock. The isolated oracle uses its standard/default feature
set; these files require no serde behavior. Cargo metadata identifies the
actual selected registry package. The tool verifies the original archive and
every archived member against the expanded sources Cargo will compile.

The pinned runtime is exactly
`rustc 1.99.1 (58cf8f9c3 2026-10-13)` on a 64-bit host. Checked and wrapping
profiles use separate whole-workspace debug builds with explicit
`CARGO_PROFILE_DEV_OVERFLOW_CHECKS`; they do not apply flags only to the driver.
Source arithmetic panic text is normalized to `overflow`. Lock assertions,
custom trait panics, and hasher panics compare their payload text; source paths
and runtime panic locations are not claimed.

The retained corpus has **388,620 requests**, 194,310 per profile. Each profile
contains 193,632 matching cases, 230 mutator/hash/error scripts, and 448 custom
trait scripts. Matching inputs cover combinatorial requirements, all operator
families, partial components, prerelease identifiers, metadata, max-u64 bounds,
composed and opposing bounds, and unusual public Comparator fields. Each case
compares normal semver matching, Cargo prerelease matching, and both Opt Req
matching modes, including caught overflow.

Mutator scripts observe full enum state initially and after every operation or
panic: Display, derived Debug, all flags, borrowed version getters, Clone
equality, and native Hash callback traces. They test lock assertions, precise
updates, exact/locked constructors, From, variant equality, resetting to Any,
and partial hash traces when the hasher panics at successive callbacks. Custom
trait scripts separately verify required methods, defaults, each independent
override, call order, returned values, and panics. Constructor parsing failures
retain the exact dependency error text and create no source value.

`manifest.json` binds source and original lockfile pins, dependency checksum,
temporary-workspace support files, deterministic generator, both request and
original-response streams, and raw/compressed fixture hashes. Baseline Go replay
requires no Rust, Cargo, Git, source checkout, registry cache, or network.

## Reproduction and portability

After the pinned Cargo Git cache is ready, fetch once, then all reference builds
run with `--locked --offline`:

```sh
export CARGO_HOME="$PWD/.toolchain/port-cargo-home"
RUSTC="$PWD/.toolchain/rust/bin/rustc" .toolchain/rust/bin/cargo fetch --locked \
  --manifest-path tools/port-cargo-semver/Cargo.toml
go run ./tools/port-cargo-semver
go test ./port/cargo/src/util -run 'Semver|OptVersionReq'
FERRUGO_RUSTC=rustc FERRUGO_CARGO=cargo go run ./tools/port-cargo-semver -check
```

Flags `-root`, `-cargo-source`, `-cargo-home`, `-cargo`, and `-rustc` select paths
and tools. Flags override `FERRUGO_CARGO_SOURCE`, `CARGO_HOME`, `FERRUGO_CARGO`,
and `FERRUGO_RUSTC`; bare executable names resolve through PATH, and relative
paths resolve against the repository root. Defaults use the local pinned
toolchain and caches. Explicit empty, missing, or incompatible selections fail
rather than falling back. Lazy Git fetching is disabled, and reference source
checkouts are never edited.

Go 1.23 is supported. `-check` re-executes the original sources and tests without
changing retained evidence. Existing gzip bytes are reused only after bounded
decompression, CRC validation, and exact raw equality; compiler/compressor
version differences therefore do not rewrite identical original results.
Original generation-host/Go-version metadata is retained during check mode.

These are complete source-file foundations for Cargo version resolution. They
do not establish completion of registry access, dependency resolution, feature
resolution, Cargo, or production rustc compilation. No bootstrap acceptance
profile was expanded by these ports.
