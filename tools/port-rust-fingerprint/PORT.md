The production port covers all of pinned
`compiler/rustc_data_structures/src/fingerprint.rs` (242 lines) and its complete
14-line test file at Rust revision `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.

| Pinned source | Git blob | Go target |
| --- | --- | --- |
| `fingerprint.rs` | `1855d1ff921fcd43c3daecd97c017562209f95df` | `port/rust/compiler/rustc_data_structures/src/fingerprint.go` and `fingerprint_format.go` |
| `fingerprint/tests.rs` | `e04af19abf2bd9a410f8b167680cd832d70dac1b` | `fingerprint/tests_test.go`, original permutation test with all six input constants and permutations |

The package is `datastructures`. The port retains both limbs, lexicographic
ordering, explicit wrapping arithmetic, the carry in commutative combination,
un-padded hexadecimal concatenation, fixed little-endian codecs, generic and
specialized hashing, stable trait operations, and both wrapper conversions.
`PackedFingerprint` stores 16 private native-endian bytes with alignment1;
it never exposes an unaligned integer reference. The native reference validates
16-byte size, fingerprint alignment8, and packed alignment1 on amd64. These
checks do not certify the ABI of the whole pinned compiler.

`NewFingerprint` accepts the public `FingerprintComponent.AsU64` contract;
`FingerprintU64` represents the primitive implementation, while `Hash64` already
implements the method. `FingerprintFromU64` is the raw-value convenience.
Components with owned destruction use `NewFingerprintOwned` or its primary
contextual form `NewFingerprintOwnedWithContext`. Each descriptor contains its
moved value, a borrowed conversion callback, and a borrowed destructor callback.
Descriptors transfer exclusive ownership and must not be reused after the
constructor consumes them. Callback pointers and encoder byte slices are
temporary borrows; copy data when retaining it. The callbacks receive the
actual local cell. Context-aware callbacks take
precedence over their simple forms. Conversion occurs first-to-second and
argument destruction second-to-first, including when conversion panics.

Panic state is an explicit `rustfmt.Context`, not global Go state. Ambient
unwinding remains distinct from locally active argument cleanup. A caller can
catch a fresh panic inside an outer destructor after local cleanup; a panic
escaping that outer required cleanup aborts. A second panic escaping local
cleanup also aborts. Translated panic sites call `Context.Panic`, which aborts
immediately during panic-hook/message processing. The host representation is
exit134 rather than a promise about SIGABRT or core dumps. Native catch/unwind
and real panic-hook probes cover all read/drop masks: 12 catchable records and
36 isolated abort scenarios across ordinary, inherited, and processing contexts.

Plain Go panic/defer callbacks retain callback-local Go timing, including when
placed in a context-aware callback field. Translated source panic sites must
call Context.Panic for source timing. When a raw Go panic escapes a callback
in processing mode, a component-local fallback guard aborts before owned
argument cleanup or remaining argument cleanup after Close. It cannot undo
callback-local Go defers or intercept a panic caught inside that callback.
Eight isolated fallback cases compare the retained native processing traces
when no local defers exist, and explicitly retain the extra Go-local defer
events in the host fallback cases.

The `Hash` implementation recognizes exactly `*Unhasher` and emits one wrapping
sum. Other hashers receive two typed `WriteU64` calls in limb order. A distinct
wrapper around `Unhasher` follows that generic path, including its debug
assertion. `StableHash` delegates two typed writes to caller-owned state and
leaves context unused; it supplies no replacement hash algorithm. The trivial
stable-order macro's associated boolean and unit marker are represented by
`FingerprintCanUseUnstableSort` and `StableOrdTripleChecked`.

`fingerprint_unhasher.go` ports only the exact `Unhasher`/Default/Hasher block
from `unhash.rs` blob `0617ef83aad9c791aa11f7b22540c388024afdd6`. Its explicit
profile constructor selects source debug assertions. `UnhashMap`, `UnhashSet`,
and `UnindexMap` remain unported; this helper grants no whole-file credit for
`unhash.rs`. The shared `stable_hash.go` is unchanged.

Debug formatting delegates numeric fields to the hash package's formatter and
tuple/indent operations to the neutral `port/support/rustfmt` builders. Compact
fields retain the mutable formatter pointer; pretty children copy it. The
reference checks every selected writer failure, independently overridden
`write_char`, NUL and Unicode fill, numeric radix/flags, nested indentation,
and the formatter state after failure. Display uses its own inner hex format
and ignores outer padding flags, as in the source.

The development reference compiles untouched complete fingerprint/test/unhash
modules, complete original `rustc_serialize` and `rustc_hashes` crates, and exact
relevant stable-hash contracts/macro fragments. It uses real memory codecs and
the locked real stable hasher. No production path invokes Rust. The pinned
source declares Rust1.101; the available reference compiler is Rust1.99.1.
`RUSTC_BOOTSTRAP=1`, specialization, and injection of the missing `never_type`
feature gate are development-only; no source body is patched. See
`reference-build/README.md` for dependency integrity and scope.

Run the retained fixture and original source test with Go1.23 or later:

```sh
go test -race ./port/rust/compiler/rustc_data_structures/src ./port/rust/compiler/rustc_data_structures/src/fingerprint
go vet ./port/rust/compiler/rustc_data_structures/src ./port/rust/compiler/rustc_data_structures/src/fingerprint ./tools/port-rust-fingerprint/witness
python3 tools/port-rust-fingerprint/generate.py --offline --check
FERRUGO_RUN_FINGERPRINT_ORACLE=1 go test -run '^TestFingerprintOriginalOracleRegeneration$' ./port/rust/compiler/rustc_data_structures/src
```

`FERRUGO_GO`, `FERRUGO_CARGO`, `FERRUGO_RUSTC`, and `CARGO_HOME` select host
tools/cache; executable names on PATH are supported. Project-local toolchains
remain the defaults. `--fetch` explicitly obtains locked registry dependencies;
offline source-object misses fail without Git lazy fetching. The generator
retains existing valid compressed bytes when the raw oracle is unchanged, so
Go/Python compression versions cannot cause spurious check differences.

The manifest records exact source and dependency hashes, profiles, requests,
raw/compressed hashes, source fragments, and abort traces. Stateful hash probes
replay operations from the actual Go production adapter into real Rust
`DefaultHasher` and `StableSipHasher` with existing prefix state. The concrete
hash algorithm remains caller-owned in Go. The native reference and its cache
provide no completion credit for the dependency crates or standard-library
formatting files. Parent Rust MIT/Apache licenses cover the original source.
