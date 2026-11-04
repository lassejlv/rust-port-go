# rustc base-N source port

The two source files are pinned at Rust commit
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`. Their MIT OR Apache-2.0
licenses and notices are retained under `port/rust/`.

| Original source | Go destination | Git blob |
| --- | --- | --- |
| `compiler/rustc_data_structures/src/base_n.rs` | `port/rust/compiler/rustc_data_structures/src/base_n.go` and `base_n_owned.go` | `0c3d7613d4f1ff69a31c4458bd571074bbd5c618` |
| `compiler/rustc_data_structures/src/base_n/tests.rs` | `port/rust/compiler/rustc_data_structures/src/base_n/tests_test.go` | `148d8dde02af1b12433fa87509a742d1e59c3421` |

The mirrored package is `datastructures`. This mapping covers these two files;
it grants no completion credit for the surrounding crate or shared Go helpers.

## Source API

| Rust item | Primary Go item |
| --- | --- |
| Three base constants and exact `BASE_64` alphabet | `BaseNMaxBase`, `BaseNAlphanumericOnly`, `BaseNCaseInsensitive`, private `baseNDigits` |
| `u128` | `BaseNU128{Hi, Lo uint64}` |
| `ToBaseN: Into<u128>` | `BaseNTrait[T]`, consuming `*rustvec.Owned[T]` |
| `Into<u128>::into(self)` | `BaseNTrait.Into128(owned, context)` |
| Static `ToBaseN::encoded_len(base)` | `BaseNTrait.EncodedLen(base, context)`; no `Self` argument |
| `to_base(self, base)` | `BaseNToBaseOwned[WithContext]`; independent `ToBase` override |
| `to_base_fixed_len(self, base)` | `BaseNToBaseFixedLenOwned[WithContext]`; independent `ToBaseFixedLen` override |
| Built-in `u32`, `u64`, `u128` implementations | `BaseNU32OwnedTrait`, `BaseNU64OwnedTrait`, `BaseNU128OwnedTrait` |
| Private fixed buffer and start offset | `BaseNString`, `[128]byte` and native `uint` |
| `Deref<Target = str>` and `AsRef<str>` | Pointer-receiver `Deref[WithContext]`, `AsRefBorrowed[WithContext]` |
| `Display::fmt` | `WriteDisplayFormatter(*rustfmt.Formatter)` |

`Into128` and `EncodedLen` are required implementation metadata. Optional
ordinary/fixed method slots override independently. The fixed default dispatches
the ordinary method first, including its override, and only then calls the
static length function. Conversion and all remaining `Self` destruction finish
before encoding or static length calculation. The built-in trait factories
retain each source integer width's maximum and divisor casts.

The owned argument is a stable cell. Callbacks borrow that actual cell and can
mutate it, move/take it, destroy it explicitly, or leave it for source parameter
cleanup. `ValueOps.Drop` supplies the implementing type's complete destructor
and field glue. The dictionary is borrowed static operation metadata, with no
additional consumed closure. The input handle must not be reused after transfer;
borrowed pointers must obey the corresponding Rust lifetimes.

The contextual primary APIs accept `BaseNConfig{OverflowChecks: ...}` and an
explicit `rustfmt.Context`. Use the same arithmetic selection for the operation
and its built-in trait factory. `DefaultBaseNConfig` enables checked arithmetic;
debug assertions are independent and there are no `debug_assert!` sites here.
The source's final `assert_eq!(n, 0)` follows from the only successful loop exit.

Translated source panic sites call `Context.Panic`, including inside supplied
destructors. Normal, inherited-unwind, caught nested panic and panic-processing
states remain distinct. Raw Go panic/defer callbacks retain local Go cleanup
timing; boundary guards cannot undo callback-local Go defers. `Owned.Context()`
is the guard of an entered operation, not a current-thread lookup. A value moved
into longer-lived storage needs the later caller's explicit context when it is
moved or dropped there. Runtime metadata is a borrowed scope witness.

## Borrowing, formatting and conveniences

`Deref` and `AsRefBorrowed` expose the actual object's buffer, including the
one-past-end pointer of a zero-length suffix. Keep that object in place and do
not overwrite it while a borrow is used. Its backing storage remains live with
the Go borrow; no Rust ABI/layout equivalence is claimed. Obtain values through
an encoding operation: Rust exposes no zero-value constructor.

`WriteDisplayFormatter` is exactly one `write_str(self)` call, even for an empty
suffix. It preserves writer errors, borrowed string identity and the caller's
mutable formatter state. Width, precision, alignment, fill, alternate, sign,
zero-padding and Debug-number flags are passed through; this Display ignores
them as the original does.

`String`, `AsRef`, `fmt.Stringer`, the former `BaseNValue` extension interface,
named integer wrappers and width-specific free functions remain conveniences
for trivial-drop, normal checked contexts. They return independent strings or
copied values and do not replace the owned or borrowed primary contracts.
In particular the legacy interface's value receiver is not the primary static
associated-function adapter.

## Arithmetic and failures

The exact alphabet is `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@$`.
All three integer widths support bases 2 through 64. Zero emits one digit;
fixed encoding exposes leading initialized zeroes to the type's maximum width.
Base 2 through 36 fixed-width strings preserve numeric lexical order. The
original upper/lowercase alphabet ordering is retained for higher bases.

There is no added base-range check. Zero divisors, output exhaustion and invalid
digit indices fail at the selected operation. Oversized bases can succeed for
small values. A `u32` length divisor is narrowed exactly as in Rust. Length zero
returns an empty suffix; short custom lengths truncate. Lengths 129 or
`usize::MAX` panic during checked subtraction. With overflow checks disabled,
construction succeeds with the wrapped start; subsequent Deref/AsRef/Display
panics. Those phases are independently checked by the native oracle.

Base-one minimal encoding succeeds for zero and exhausts the buffer for positive
values. The primitive length loop at base one (including a u32 divisor cast to
one) never reduces its maximum. Eventual counter overflow is profile dependent;
the finite reference stream makes no runtime claim about those unbounded loops.
Panic wording is a runtime detail; operation order, success/failure and cleanup
are validated. Two-limb `math/bits.Div64` preserves every u128 bit.

## Reference tools

All reference clients compile the complete unchanged pinned source and its two
original tests. Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), x86_64 Linux, is the
explicit comparison runtime. `RUSTC_BOOTSTRAP=1` enables only the source's ASCII
API feature gates; original source bodies are never patched. This integer-only
file has no Unicode dependency or compiler-version-sensitive table binding.

```sh
go run ./tools/port-rust-base-n -check
python3 tools/port-rust-base-n/contracts/generate.py --check
python3 tools/port-rust-base-n/contracts/owned_generate.py --check
go test ./port/rust/compiler/rustc_data_structures/src/base_n
go test ./port/rust/compiler/rustc_data_structures/src -run '^TestBaseN'
FERRUGO_BASE_N_CONTRACTS=1 go test ./port/rust/compiler/rustc_data_structures/src -run '^TestBaseNContractsLiveRegeneration$'
```

The Python clients resolve `FERRUGO_RUSTC` on PATH, defaulting to the local
`.toolchain/rust/bin/rustc`, and accept `FERRUGO_RUST_SOURCE`. Source Git reads
disable lazy fetching. Their four profiles independently select debug assertions
and overflow checks. `--check` performs no tracked writes. Generation preserves
existing valid compressed bytes when the raw stream is unchanged, so gzip
implementation differences do not invalidate fixture hashes. The new ownership
tool also enforces the frozen numeric and formatter fixture hashes.

`manifest.json`, `contracts/format-manifest.json` and
`contracts/owned-manifest.json` record source/fixture/tool identities and profile
evidence. Ordinary Go tests need no Rust, Git or network. Production code uses
Go's standard library and the shared ownership/format adapters; it never invokes
the Rust tools or reads the reference fixtures. See `VALIDATION.md` for gates
and explicit architecture limits.
