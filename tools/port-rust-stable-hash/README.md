# Original rustc-stable-hash oracle

This development checker exercises the unchanged official `rustc-stable-hash`
0.1.2 crate. Production Go hashing has no Rust fallback. The source inventory
and licenses are in `port/dependencies/rustc-stable-hash/v0_1_2`; independent source/API review and final native/minimum-Go gates passed. The crate itself has no dependencies; locked serde_json and
its closure are development transport only.

With Go 1.23+ and the recorded Rust 1.99.1 formatter reference:

```sh
cargo fetch --locked --manifest-path tools/port-rust-stable-hash/oracle/Cargo.toml
go run ./tools/port-rust-stable-hash -check
```

Without `-check` the maintainer generator records new native evidence. Check mode
compares retained artifact bytes/content without rewriting them; ignored native
builds and the exact private arithmetic source copy are regenerated as execution
inputs. Existing gzip bytes remain unchanged whenever their complete decoded
content matches native output, so newer/minimum Go compressors share artifacts.
Gzip CRC/end-of-stream and exact native data are checked.

Flags `-cargo`, `-rustc`, `-go` accept PATH names or explicit paths. The matching
FERRUGO_CARGO/RUSTC/GO environment variables select the same tools; local installed
toolchains are preferred only without a selection, then PATH. Empty/invalid
explicit overrides fail. `-cargo-home`/CARGO_HOME select the cache; otherwise an
existing project cache is preferred, then the host `.cargo`. Fetch and check
must use the same cache. The checker runs Cargo offline with locked versions.
Every selected registry archive is SHA-256 verified and every expanded source
member is checked before/after execution, rejecting symlinks/injected files.

```sh
PATH="$PWD/.toolchain/rust/bin:$PATH" \
CARGO_HOME="$PWD/.toolchain/port-cargo-home" \
  go run ./tools/port-rust-stable-hash -check -cargo cargo -rustc rustc -go go
```

The native profile matrix is default/nightly × debug-assertions on/off ×
overflow-checks on/off. RUSTC_BOOTSTRAP=1 belongs only to the reference programs'
std prefix-free trait hooks and the original crate's requested nightly feature;
no reference source body or production algorithm is replaced.

The 3,742 streaming requests check every buffer/word/spill boundary, raw/stable
primitive writes, both 128-bit paths, source-valid release short arrays above
eight bytes, string/length prefixes, clone/diverge/restore, keyed/random streams,
borrowed finish, typed 128-bit result, and Debug/state after each operation.
Debug assertion profiles have separate retained expected streams; irrelevant
nightly/overflow switches must preserve those outputs.

Eight generic trait streams cover all primitive defaults/overrides, default vs
custom const-LEN short write (including arrays beyond eight bytes), std string/
length hooks, borrowed custom state, CloneFrom's replacement path, option reset
in delegated Debug, consuming finish/conversion, and non-comparable associated
outputs. Nightly/default traces differ where the original delegation differs.

10,594 Debug writer records compare accepted prefixes, each attempted string or
character call, error propagation, width/precision/fill/hex/pretty options,
newline fill, empty writes, and post-error mutable options. The generic delegate
uses reset options; the raw hasher and result preserve caller options, including
MaybeUninit's padded/precision string behavior.

37 owned-operation records cover arbitrary non-Copy H and nontrivial Hash/W
outputs, actual borrowed state mutation, whole-value CloneFrom, replacement after
old Drop panic, prepared-return leaks, inherited unwind, new cleanup panic, and
processing-panic abort. Native SIGABRT and Go exit 134 compare as the abort class;
exact event prefixes remain compared. The unchanged private int_overflow module
adds 882 inputs per arithmetic profile, distinguishing both independent flags.
No general stdHasher/Vec/fmt source-file credit is implied by these used-contract
adapters or test subjects.

The Go fixture packages and preserved original tests run under the same eight
profiles. The two original files contain 17 declarations (15 active natively),
38 assertion sites, the exact 64×16 vector table, all signed/wide literals, and
12 macro instantiations. Three original doctests are preserved. Their untouched
Rust runs, input Hash-dispatch witness, literal/AST order audit, race/vet, and
386/ARM compile-only evidence live in `original-tests`.

```sh
go test -race ./port/dependencies/rustc-stable-hash/v0_1_2/...
go test -race -tags rustc_stable_hash_nightly ./port/dependencies/rustc-stable-hash/v0_1_2/...
go vet -tags rustc_stable_hash_nightly ./port/dependencies/rustc-stable-hash/v0_1_2/... ./tools/port-rust-stable-hash/...
python3 tools/port-rust-stable-hash/original-tests/audit.py --check
```
