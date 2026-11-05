# AST utility candidate validation

The candidate covers the complete pinned `util/case.rs` (6 lines),
`util/comments.rs` (133 lines), `util/unicode.rs` (35 lines), and the canonical
64-line comments test file. The source audit verifies exact Git blobs,
SHA-256 values, contiguous whole-file regions, all five original test
declarations, all ten assertions and their literal order. The parent owns
inventory credit and independent acceptance.

The complete original rustc_ast reference uses 12 unchanged compiler crates
and 109 checksum-verified registry selections. Every preparation checks both
the official archives and the actual expanded registry files compiled by
Cargo. All six original AST unit tests pass in each debug/overflow profile;
the extra tokenstream test is executed as reference evidence and receives no
port credit here. The exact compiler is official nightly 2026-11-04,
`708c94ef093ecc42fb8851e17d8383d999727abc`, x86_64 Linux, Unicode 18.0.0,
CFG_RELEASE=1.101.0. No original bodies, manifests or feature gates are patched.

Each of d0o0, d0o1, d1o0 and d1o1 produced 26,042 records: 25,852 beautification
and conservative classifier inputs, 80 actual Comment Clone/CloneFrom cases,
and 110 enum/layout/format/hash/codec records. Beautification compares text,
Symbol identity and changes in the genuine interner's allocation index. The
Comment cases compare vector capacity, each actual String capacity, distinct
buffer identities and copied public fields; Go replays use owning StringCell
values and mutate the original cells to check independence.

The detector is checked against the unchanged source for all 1,112,064 valid
scalars in five valid UTF-8 string positions in every profile: 22,241,280
classification calls. The 2,048 surrogate slots have a separate invalid marker
and are never constructed as Rust chars. The ordered nine-element constant
and the shared immutable borrow/copy boundary are checked separately.

Each profile also retains 264 real macro-derived decoder cases: all 256 tags
normally, plus tags 0, 1, 2 and 255 during inherited unwinding and during panic
processing. The two invalid processing cases really terminate with SIGABRT
per profile, eight total. Invalid tags consume one byte and raise an owned
String at the observed `original/compiler/rustc_ast/src/token.rs:40:55`.
Panic filenames and caller coordinates are observed verbatim. The only client
path remapping is an explicit producer compiler argument, retained in the
bound generator. No Go source location is inferred by default.

Minimum Go 1.23.12 gates ran in `/tmp/ferrugo-ast-util35.l4z3jvho`, an isolated
copy of the coherent Root35 snapshot plus exactly this AST tree and Cargo's
frozen ten-file owning String overlay. GOMAXPROCS=2, -p 1 and the shared cache
were used. No unrelated live Span or syntax-engine edits were included.

| Gate | Result |
| --- | --- |
| Default AST subtree and original child tests | PASS, utility replay 1.177 s; original tests 0.005 s |
| Race, complete subtree | PASS, utility replay 6.452 s; original tests 1.015 s |
| Both ferrugo_rustc_nightly and rustc_stable_hash_nightly tags | PASS, utility replay 1.151 s; original tests 0.005 s |
| Subtree go vet | PASS |
| Final actual four-profile generator --check | PASS; all raw and gzip fixture bytes preserved |
| Python producer integrity and source audit | PASS, five integrity tests plus exact source/test audit |

The owning String dependency separately matched 1,344 actual SDK 1.99/1.101
scripts and 45,696 operations across all four flags, with its own minimum-Go
race/vet evidence. Its frozen source/proof bindings are included in the AST
producer manifest. This dependency supplies scoped String methods only; it
does not receive whole standard-library String or Vec file credit.

Case's one actual identity lives in the approved leaf and is forwarded by the
canonical util module to avoid the future token/comments import cycle. This
adds no source credit. Parent CommentKind covers only token.rs lines 40–44 and
their complete derived operations. The inspected version.rs and node_id.rs
remain outside this scope. Native architecture evidence is 64-bit Linux;
32-bit layout metadata is explicitly logical and has no native runtime claim.

Reproduce the native evidence with `CARGO_BUILD_JOBS=1 python3
tools/port-rust-ast-util/generate.py --check`. For another selected producer
environment use a fresh `--output-dir`, then set
`FERRUGO_AST_UTIL_PROFILE_DIR` to that exact bundle for Go replay. The selector
fails closed, including empty/missing directories, source drift, compressed
corruption and missing manifests; parent abort subprocesses retain the same
canonical selection and captured manifest checksum. Default generation refuses
different historical raw observations. `--prepared` cannot bypass complete
preparation. No production function invokes Rust.
