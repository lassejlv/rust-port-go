# AST utility source block — complete leaves and a held ownership contract

The selected source block is pinned `util/case.rs` (6 lines),
`util/comments.rs` (133 lines), `util/unicode.rs` (35 lines), and the original
64-line comments child test file (5 tests, 10 assertions). Inventory blobs and
SHA-256 values are retained in source-manifest.json. Case, Unicode and the
original child tests are complete-file candidates for coherent acceptance.
The comments implementation remains partial: its public `Vec<String>` field
needs an owning vector interface for source reserve, push, pop, mutation,
iteration and moves. A Go slice with matching clone capacity does not close
that public contract. No completion credit is proposed for `comments.rs`.

The one canonical Case identity and its complete Copy/Clone/Eq/Debug behavior
live in util/case_types, with the exact canonical util/case.go forwarding alias.
Parent token code can import that leaf without importing comments, which needs
parent CommentKind. This decomposition creates no additional source credit.
The scoped parent token_comment_kind.go dependency retains the actual token.rs
40–44 enum and all derived operations: Clone/Copy/PartialEq/Eq, std Hash,
SpanEncoder/SpanDecoder codecs, Debug and StableHash. This is not a full token
port. Actual builtin/custom macro behavior is exercised in the complete original
crate, including typed Hash dispatch and all 256 decoder tags.

BeautifyDocString takes the genuine explicit SessionGlobals, Symbol and
CommentKind. The contextual primary forwards the actual current metadata to
both Symbol borrow and conditional interning. Symbols are Copy; the source has
no owned generic callback here. Comment's current public Lines field contains
the actual ruststr.StringCell values, retaining mutable String headers and
buffer capacities. Clone allocates a buffer with capacity equal to length and
independently clones each String with capacity equal to its byte length.
The derived default clone_from prepares a complete clone, then destroys and
replaces the old fields. It does not use String/Vec clone_from's reuse strategy.
String and vector descriptor assignment is an explicit owner move under the
documented borrow discipline. No Go append growth is presented as Rust Vec
growth. CommentText is the separately named copying []string convenience.
The source String Vec element metadata is 24 bytes, alignment 8, in the native
64-bit profile; the logical 32-bit metadata is 12 bytes, alignment 4, with no
native 32-bit runtime claim.
Cargo's new owning String helper is an explicit acceptance dependency. Its
matching 1.101 primitive proof and this candidate's actual String-field replay
both pass; the parent still owns independent acceptance.
CommentStyle exposes only its actual Clone/Copy/PartialEq/Debug operations.

Whitespace is ASCII plus the existing exact pinned core Unicode18 White_Space
table. Private lines handling follows Rust str::lines (LF, CRLF, trailing
terminator, lone CR), and the horizontal prefix retains the source scalar/byte
distinction. It does not use the host Go Unicode approximation. Flow-control
detection retains the original first-E2-byte search and three-byte skip over
valid UTF-8. The constant exposes a documented immutable shared borrow plus
an independently allocated Go copy convenience. Raw string arguments retain
Rust &str's valid-UTF8 precondition.

The inspected attr/version.rs (53 lines) and node_id.rs (52 lines) are outside
this acceptance scope. Version requires all macro-derived codecs/hash/order,
process environment and OnceLock behavior; NodeId requires the actual newtype,
Idx/Step/niche/format/codecs plus real NodeMap/NodeSet/Entry dependencies. Neither
receives partial type-only completion credit from this work.

The intended development reference adds the complete unchanged rustc_ast and
rustc_ast_ir crates to the actual original compiler dependency graph. Original
Rust functions, tests, token types and macros must execute without body edits
or type stubs. The first native d1o1 graph built successfully with one Cargo
job: all 12 unchanged crates, 109 verified registry selections, all six original
AST tests and a real externally linked session/Symbol/beautify smoke passed.
All four debug/overflow profiles now pass. Each produced 25,932 request
observations plus 110 trait records, 264 tag cases including 2 real SIGABRT, and
all 1,112,064 valid scalars in five string positions. Minimum Go 1.23 default,
race, both integration tags, vet and the original child tests now pass in an
isolated coherent snapshot. Exact counts, commands and bounds are retained in
VALIDATION.md.
Source and Python integrity work can run independently:

```sh
python3 tools/port-rust-ast-util/audit.py
python3 -m unittest discover -s tools/port-rust-ast-util -p 'test_*.py'
```

The separate complete reference lives at
`.toolchain/oracles/rustc-ast-util-reference/build-profile-d1o1.json`. Other
profiles use their own named files. `reference.load(debug, overflow)` always
runs the complete preparer before returning paired rmeta/rlib linkage; cached
JSON is never a replacement for source, registry archive and expanded-source
verification. This graph includes the real rustc_ast_ir for other scoped
clients. It does not modify the accepted Span preparer or Span cache.

The generator's default profile matrix is debug assertions on/off crossed with
overflow checks on/off. It uses the exact official dated nightly. The native
client has an explicitly recorded client-only `--remap-path-prefix` argument;
all observed panic filenames are retained verbatim. The actual token derive
currently reports `original/compiler/rustc_ast/src/token.rs:40:55`. No default
Go panic location is inferred. A fresh `--output-dir` bundle and the explicit
`FERRUGO_AST_UTIL_PROFILE_DIR` replay selector support alternate valid producer
environments. Default regeneration preserves historical compressed bytes when
raw observations agree and refuses different historical raw observations.
`--prepared` is a compatibility spelling only, with no verification bypass.

Passing native gates cover the original tests, exhaustive flow-control scalar
membership, adversarial/seeded beautification including exact Symbol identity,
Comment vector/string cloning, all three enums' formatting and equality, CommentKind
typed Hash/default dispatch, real codecs/all 256 tags and stable hashing. Full
production implementation and native/coherent Go evidence are required before
the parent can accept these files.
