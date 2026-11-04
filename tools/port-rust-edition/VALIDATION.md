# Edition validation

The retained reference uses all five original editions, the actual original
derive macros, the actual memory codec, and the real standard/stable hashers.
The used-method audit confirms `write_isize`, stable hashing of a `u8`,
`emit_u8`, and `read_u8`. All other reference-only span/context interface calls
remain zero. The complete original derived expansion is hashed in
`port/rust/compiler/rustc_span/src/edition_oracle_manifest.json`.

The fixture contains 3,220 original-runtime records:

| Records | Count | Coverage |
| --- | ---: | --- |
| Variants | 5 | Every explicit predicate/name, derived typed hash/codec operations |
| Comparisons | 25 | Every ordered pair, equality, clone and clone-from |
| Formatting | 75 | Fifteen Display/Debug flag combinations per variant, including precision, custom fill, width and alignment |
| Writer failures | 5 | Display and Debug both propagate failure |
| Stateful hash comparisons | 645 | All variants with prefix lengths 0–128 and seeded prefix bytes |
| Decode cases | 393 | Every byte tag, all 251 invalid variants, missing/truncated footer, exhaustion and initial positions |
| String parsing | 2,070 | Every valid spelling, strict rejections, signed/whitespace/Unicode/control variants and seeded UTF-8 strings |
| Constants and unused-interface audit | 2 | Exact lists/default/latest, zero unused interface calls |

Hash validation executes the actual Go `Hash` and `StableHash` adapters into
typed recording state. Those emitted operands and caller prefix bytes become
requests to the Rust oracle. It computes both the original derived hash and
the real Rust hasher result after replaying the Go operations, then asserts
equality. This covers actual hasher state without implementing an unrelated
caller algorithm or inserting a production Rust fallback. The original typed
Hash trace, macro expansion and stable primitive implementation independently
bind the operation kinds. Ordinary Go fixture tests verify that current code
still emits the same operations; live checks rerun the actual hash algorithms.

Uncompressed fixture SHA-256:
`edd2d86c9da83d430139bb0cfbab46c801dc091718a528a48ba723173a93fbd4`.
The gzip fixture and manifest are retained beside the Go implementation, so
normal Go tests require no Rust installation. `-check` preserves the existing
gzip bytes when decoded output agrees, avoiding Go compressor-version drift.
A tool test explicitly checks an alternate valid gzip encoding and rejection
of changed original output without rewriting files.

Prepare dependencies once (explicit online operation):

```sh
python3 tools/port-rust-edition/reference-build/prepare.py --fetch --validate
```

After the pinned objects/archives are cached, regenerate or check entirely
offline from the repository root:

```sh
go run ./tools/port-rust-edition
go run ./tools/port-rust-edition -check
FERRUGO_RUSTC=rustc FERRUGO_CARGO=cargo go test -race ./port/rust/compiler/rustc_span/src ./tools/port-rust-edition -count=1
go vet ./port/rust/compiler/rustc_span/src ./tools/port-rust-edition
```

`-root`, `-rustc`, `-cargo`, `FERRUGO_RUSTC`, `FERRUGO_CARGO` and `CARGO_HOME`
support the recorded local and CI tool layouts. Bare executable names resolve
through `PATH`; invalid explicit overrides fail. Source/dependency preparation
is offline in the Go runner and disables Git lazy fetching. The manifest binds
all 37 original source files via provenance, 12 exact contract fragments,
31 locked registry archive identities, helper sources, original expansion,
compiler profile, requests and outputs. Machine-specific artifact paths are
used for linking without becoming retained manifest data.
The helper also verifies all 920 compiled registry source files against those
31 archives before compilation and after proof, rejects missing/extra/changed
files and symlinks, and retains normalized source-tree hashes. Five isolated
helper regression tests cover the extraction boundary.

The actual minimum Go 1.23.12 toolchain is used for tests/race/vet and live Rust
comparison; Go 1.27 checks the same retained gzip and manifest. The reference
profile is Linux x86-64, rustc 1.99.1 against pinned 1.101.0 source with the
development gate detailed in `PORT.md`. No whole-compiler ABI/runtime claim or
source credit for dependency facades follows from these tests.
