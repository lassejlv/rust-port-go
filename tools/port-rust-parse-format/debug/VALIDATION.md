# Derived Debug validation

The reopened format-parser Debug contract is restored by the source-backed
methods in `debug_format.go`. No parser algorithm, source test bytes, existing
parser corpus or literal-escaper implementation changed. This repairs existing
source-file coverage and grants no new source credit.

Completed on Linux x86-64:

- Actual Go **1.23.12** full parser package race tests passed (45.579s),
  including the existing original-state corpus, original translated tests,
  scalar escape tests and new derived Debug replayer. Package vet passed.
- Exact original Rust **1.99.1** built the unchanged pinned parser/lexer and
  reproduced **156,460** Debug records covering all nine derived types, 40
  values and 19 option patterns. Both independent C/S dispatch and default
  character fallback matched every writer-failure position and formatter reuse.
- The live `-check` command passed with **Go 1.27.3**, using bare PATH names for
  Go/rustc/Cargo and relative source/cache overrides. It left retained gzip and
  manifest bytes unchanged.
- Six isolated integrity tests passed: exact archive/expanded sources plus
  allowed Cargo metadata, changed normalized manifest, missing source, injected
  source, symlink directory and changed archive. All 16 reference registry
  packages (577 original expanded files) were verified before and after build.
  Their versions/checksums also match the pinned original Rust Cargo.lock.
- A Go 1.23 **386** test artifact cross-compiled. Native ELF32 execution is not
  available in this environment; no 32-bit runtime comparison is claimed. The
  host-width native corpus is explicitly skipped when usize is not 64-bit.

The native corpus raw SHA-256 is
`c353d189d5004da84fd30fb9ae9e12a5f4fd39a31caf6a7d3e0704a4369b41f0`.
The preserved gzip SHA-256 is
`75f4f8d52e3e7a31db63c8632b736c779511051a87ae7c5386d59f140ca4bded`.
`manifest.json` binds both to exact original Git blobs/SHA-256, locked registry
archives, compiled-source counts and retained wrapper/generator/test sources.
The existing parser/lexer original-tests and algorithm proof remain in the
parent directory's `PORT.md`, `VALIDATION.md` and original manifest.

The neutral formatter's independent Unicode 17 scalar/streaming proof belongs
to the shared support package; these records additionally prove the exact calls
made through the original parser's nine derives. This does not claim a complete
Rust standard-library formatter port, parser compiler integration, or production
Rust compiler compatibility.
