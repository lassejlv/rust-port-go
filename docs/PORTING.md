# Full upstream source port

Ferrugo's goal is a file-to-file Go port of the complete pinned Rust compiler and Cargo, able to compile production Rust projects. The existing bootstrap compiler, C11 subset backend and local Cargo subset do **not** fulfill this goal. They do not automatically count as ports of upstream files.

The source pins are recorded in [upstream-inventory.json](upstream-inventory.json). The file-level source-to-destination map and current evidence-backed states are recorded in [port-status.json](../port-status.json) and the JSONL inventories under [tools/port-inventory/data](../tools/port-inventory/data). Those inventories start unported; an existing destination or passing bootstrap example does not change a row's state.

## Complete top-level repository inventories

The inventories use `git ls-tree -r` at each exact commit, rather than the contents of a sparse checkout. Every tracked file and gitlink in those two Git trees is accounted for, including files that have not been downloaded as blobs.

| Upstream | Pinned commit | Tracked `.rs` files | Other tracked files | Gitlinks |
| --- | --- | ---: | ---: | ---: |
| rust-lang/rust | `2d20b42b883dcfc014f3c2c56081eeb6091c2179` | 39,551 | 24,684 | 13 |
| rust-lang/cargo | `24de4721844c7a4d839072bab6fb66d41c9bc1f4` | 1,379 | 1,699 | 0 |

A `.rs` file count includes compiler/library/tool implementations, test harnesses, examples, build scripts and Rust programs used as test inputs. It is not an implementation-completion count. File sizes and complexity vary too much for a file-count percentage to establish compiler completion.

`source_sha` is the pinned Git blob object ID for a file, or the commit object ID for a gitlink. It is not a raw-file SHA-256 digest. `port-status.json` also records SHA-256 digests of the generated inventory files so accidental edits, missing rows and stale summaries can be detected.

## Mapping and test preservation

Implementation files mirror their upstream crate/directory layout, replacing `.rs` with `.go`. For example:

```text
compiler/rustc_lexer/src/lib.rs
  -> port/rust/compiler/rustc_lexer/src/lib.go
src/util/edit_distance.rs
  -> port/cargo/src/util/edit_distance.go
```

Executable test-harness implementation files use Go's `_test.go` convention. For example, `compiler/rustc_lexer/src/tests.rs` maps to `port/rust/compiler/rustc_lexer/src/tests_test.go`.

Rust language-input subjects must remain Rust. A `tests/ui/...rs` program tests Rust syntax, type checking, diagnostics or runtime behavior; turning it into a Go program would stop testing that behavior. These rows have role `rust_fixture`, retain their `.rs` destination and require a Go runner/harness obligation. The inventory verifier checks byte preservation against the upstream Git blob when a retained fixture is marked ported or validated, and requires a nonempty Go runner file among its port evidence. Shared fixture bundles and runners can satisfy multiple rows when their evidence identifies every covered subject.

File roles are initially classified from deterministic path rules. Review the actual file before recording its completion; distinguish harness code from embedded/standalone Rust inputs, expected outputs, support crates and generated runner code. No file is dropped because its role is uncertain. Build manifests, Python/shell/native support, licenses, expected diagnostics and other assets have separate support inventories with their original path and suffix. Their required disposition—port, adapt, preserve or replace with a proven compatible build/input path—must be recorded in evidence.

## Source caches and tooling

The ignored source caches live at `.toolchain/upstream-rust` and `.toolchain/upstream-cargo`. Both use a local `main` branch at the exact pinned commit. Initial sparse checkouts materialize Rust's lexer, compiler data structures and typed indices, core Unicode and license files, and Cargo's source/crate directories. Working-tree files are read-only references; do not edit them or push to upstream remotes. Git metadata remains available to fetch further pinned reference files when a port needs them.

Use Python 3.8 or newer and Git. The tooling uses only the Python standard library:

```sh
# Network access is required only for preparing reference caches.
python3 tools/port-inventory/inventory.py prepare

# These commands use pinned local Git trees and do not download source blobs.
python3 tools/port-inventory/inventory.py generate
python3 tools/port-inventory/inventory.py verify
python3 tools/port-inventory/inventory.py summary

# Local isolated Git fixtures; no upstream/network access is needed.
python3 -m unittest discover -s tools/port-inventory -p 'test_*.py' -v
```

`generate` preserves recorded progress only when the source blob, source mode and target mapping are unchanged. Changes to a previously ported source or mapping fail instead of silently retaining completion. After inspecting such changes, `generate --reset-changed` explicitly clears affected progress and records the previous blob in notes. Removed ported files also require explicit review/reset. Generated row order and manifest digests are deterministic.

`verify` checks the exact pinned tree coverage, source object IDs/modes, destination mapping, duplicate destinations, state flags, evidence paths and recorded inventory hashes. It requires a nonempty destination for a ported Go implementation, exact bytes plus Go runner evidence for a retained Rust fixture, and validation evidence for validated rows. A target's mere existence never marks it complete. These are structural checks; semantic equivalence still requires the review and tests below.

A completed file is recorded explicitly, for example:

```sh
python3 tools/port-inventory/inventory.py set-status rust \
  compiler/rustc_lexer/src/lib.rs ported \
  --port-evidence port/rust/compiler/rustc_lexer/PORTING.md

python3 tools/port-inventory/inventory.py set-status rust \
  compiler/rustc_lexer/src/lib.rs validated \
  --validation-evidence port/rust/compiler/rustc_lexer/validation.json
```

The second command retains the already recorded port evidence. Evidence files must exist inside this repository. Do not execute these commands for a partial implementation, a stub, or a bootstrap file that merely has a similar purpose.

## What a completed file means

A `ported` implementation file must satisfy all of the following:

1. Its provenance identifies the exact upstream path, revision and object ID, and preserves the required licenses/attribution.
2. All behavior in that file is implemented in Go: types, constants, helpers, branches, invariants, failure cases, relevant configuration/target conditions and interactions with other ported components. Comments, placeholder functions, token enums without their algorithms and forwarding to upstream rustc/Cargo are insufficient.
3. The destination is buildable in its intended Go package. Any deliberate Go-specific decomposition or shared helper is listed in its port report, with the complete original file still accounted for.
4. Source-specific tests cover both successful and failing behavior. The report lists untranslated or unresolved dependencies; those cannot be hidden behind a completed component/production claim.

A `validated` file additionally needs retained evidence of matching the pinned upstream behavior. Record the actual commands, reference revision, test inputs/case set, result artifacts, host/target, relevant dependency/Unicode/data versions and unresolved skips. Port and validation evidence must remain reviewable in the repository. Passing unrelated tests or a few handwritten examples is not upstream validation.

For test harnesses, preserve the upstream cases, directives and expectations while porting the executable harness to Go. For retained Rust subjects, preserve original source bytes and run them through the appropriate ported compiler/Cargo harness; keep expected diagnostics and runtime outcomes. For licenses/assets, preserve the required contents and record how they are incorporated. A file record describes its own evidence, not completion of the containing compiler phase.

## Gitlinks and third-party obligations

The 13 Rust gitlinks are listed separately in `rust-submodules.jsonl`, at their exact recorded commits. Their internal files are **not** included in the top-level Git tree inventory. They remain explicitly unexpanded and cannot be marked complete by placing an empty directory at the mapped destination. They need child inventories or an explicit, reviewed external-dependency policy before a complete-project claim.

This includes LLVM/GCC-related backend sources, backtrace, documentation trees and embedded tools. Rust's `src/tools/cargo` gitlink is `3c747ea3ead23951c1b43a5edfcbdb449135ddeb`; it differs from this project's separately pinned Cargo target. Preserve both facts and resolve their integration deliberately.

Crates.io dependencies, generated tables and native libraries are additional obligations beyond these two repositories. Their exact versions/data and implementation or compatibility strategy must be pinned and tested. For example, a lexer port must reproduce the upstream Unicode identifier/emoji versions and classification behavior, rather than substituting whatever Unicode tables happen to ship with the installed Go toolchain.

## Production completion gate

The overall port is unfinished until the implementation is integrated into working Go compiler and Cargo binaries and the following evidence is available:

- Every in-scope upstream implementation, harness and support input has a complete reviewed disposition. Gitlink/third-party gaps and generated-data provenance are resolved, rather than omitted from a file count.
- The Go compiler implements the pinned Rust language/editions and compiler contracts: parsing/expansion, modules/name resolution, type/trait inference, generics, ownership/borrowing/lifetimes, const evaluation, patterns, closures, async, unsafe/FFI, metadata, monomorphization, ABI/layout, code generation, linking, panic/unwind behavior, supported targets and diagnostics/tooling interfaces.
- Applicable pinned upstream UI, compile-fail/check/run-pass, codegen, MIR/optimization, run-make, incremental, debuginfo and target tests execute through the Go implementation. Record every unsupported or skipped case and compare the expected output/diagnostics at the declared compatibility boundary.
- Compatible `core`, `alloc`, `std`, compiler builtins and target runtimes build and pass their relevant tests using the Go compiler. Verify native linking, Rust ABI, FFI, atomics, thread-local storage and panic strategy on each supported production target. A C11 subset executable is not this evidence.
- Production-style Rust projects and a pinned representative crate corpus build/test successfully with procedural macros, build scripts, dependencies and release profiles. Building the original upstream Rust compiler/Cargo sources is a useful high-coverage compatibility check; the Go implementation itself is built with Go.
- The Go Cargo port passes the applicable upstream Cargo suite, including workspaces/targets, configuration, dependency/version/feature resolution, registries/git/path sources, lockfiles/checksums/cache/offline behavior, build scheduling, profiles/cross compilation, build scripts, tests, packaging/publishing protocols and credentials. Exercise external-facing behavior against controlled fixtures/test services.
- Reproducible builds and end-to-end reference comparisons show that production work is performed by the Go ports. Upstream rustc/Cargo may be validation oracles, but cannot secretly compile/build in place of the implementation being claimed complete.

Until that gate is met, reports should name the files/components actually completed and the tests actually run. The inventories are an audit trail for the full port, not a substitute for it.
