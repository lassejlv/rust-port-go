# Cargo semver extension validation evidence

Validated on Linux/amd64 with Go 1.23.12 and Go 1.27.3. Reference compiler:
`rustc 1.99.1 (58cf8f9c3 2026-10-13)`. Full source/lock/dependency/support-file
pins and raw/compressed evidence hashes are in `manifest.json`; API mapping,
ownership adaptations, runtime profiles, and reproduction are in `PORT.md`.

Results:

- **All eleven original Rust inline tests passed unchanged:** two in
  semver_ext.rs and nine in semver_eval_ext.rs. Every corresponding Go test
  passed, preserving all original case tables, version lists, outcomes, and
  iteration order, including the 28-case prerelease regression table.
- **388,620 original-source requests passed:** 194,310 each from independent
  checked and wrapping whole-workspace Rust builds. Each profile contains
  193,632 matching cases, 230 mutator/hash/error scripts, and 448 custom-trait
  scripts. The replayer reports **1,564,792 observations** and **6,682 caught
  panics**, including custom-trait and hash-callback panics.
- The matching matrix compares normal dependency matching, the full Cargo
  prerelease extension, and both optional-requirement matching modes. It covers
  all operators, partial components, stable/prerelease/build identifiers,
  upper-bound increment overflow, short circuit ordering, opposing/composed
  bounds, and public Comparator states absent from parser output.
- Scripted operations compare complete initial and post-operation/panic states:
  Display, derived Debug, exact/locked/precise flags, borrowed getters, Clone
  equality, variant equality, and ordered native Hash callbacks. Tests include
  failed locks preserving state, precise updates outside the original
  requirement, exact/locked constructors, build-metadata distinctions, From,
  Any transitions, and partial traces when the hasher panics at successive
  callback counts.
- Custom Rust and Go implementers verify both required VersionExt methods,
  inherited defaults, each independent default override and combined overrides,
  dispatch order, returned requirements, and panic timing. Separate custom
  VersionReqExt implementers preserve that public extension point.
- Independent formatter review found and fixed the initial text-only adapter
  gap. **4,164 original Display/Debug formatting cases** now pass, including
  outer flag behavior and every Display write failure. Debug additionally passes
  **8,668 default-string writer records**, **8,668 overridden-character writer
  records**, and **8,668 mutable outer formatter-state records**. These include
  all write-failure positions, complete attempted chunks/accepted prefixes,
  actual empty writes, nested indentation, caller numeric options, and all
  371 error paths that expose the original compact formatter mutation.
- Focused Go regressions verify independent requirement ownership after
  constructors and Clone, read-only borrowed getter identity, metadata rules,
  mode Clone/Debug, checked versus wrapping bounds, and the source's unknown-op
  unreachable branch with earlier-comparator short circuiting. Invalid Rust
  enum discriminants are never constructed in the native oracle.
- Complete utility package and tool tests, plus vet, passed under **Go 1.23.12**.
- Race checks passed for the complete utility package and generation tool.
- Fuzzing passed **287,271 executions** in approximately ten seconds using two
  workers, covering matching, mutators, panic-state preservation, independent
  Clone ownership, and equivalent hash traces.
- Live **Go 1.23.12 PATH-selected `-check` passed** against evidence generated
  by Go 1.27.3. Original tests, both original-source build profiles, fixture raw
  bytes, compressed bytes, and retained generation provenance all matched.
- Generation-tool unit tests passed for deterministic requests, profile
  encoding, flag/environment/PATH precedence, explicit-empty rejection,
  registry archive/expanded-source tampering, absent selected dependencies, and
  preserving a different valid gzip level/header while rejecting truncated,
  CRC-corrupt, changed, or absent data in check mode.
- Nine invalid CLI/environment selections were independently invoked against
  the built tool. All failed with diagnostics and preserved hashes of the
  corpus and manifest: empty Rust/Cargo/source/home/root selections, a missing
  Rust executable, an incompatible executable, an explicitly empty Rust
  environment selection, and an unexpected positional argument.

Principal commands:

```sh
go test ./port/cargo/src/util ./tools/port-cargo-semver -count=1
go vet ./port/cargo/src/util ./tools/port-cargo-semver
go test -race ./port/cargo/src/util ./tools/port-cargo-semver -count=1
go test ./port/cargo/src/util -run '^$' \
  -fuzz '^FuzzCargoSemverExtensions$' -fuzztime=10s -parallel=2
FERRUGO_RUSTC=rustc FERRUGO_CARGO=cargo \
  go run ./tools/port-cargo-semver -check
```

This evidence validates both complete Cargo source files under the declared
dependency/runtime profiles. It does not establish completion of registry
access, dependency or feature resolution, Cargo, or production rustc
compilation. The bootstrap Cargo acceptance profile was not expanded.
