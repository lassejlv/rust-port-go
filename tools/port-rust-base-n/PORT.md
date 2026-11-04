# rustc base-N source port

This is the complete source port of `compiler/rustc_data_structures/src/base_n.rs` and `compiler/rustc_data_structures/src/base_n/tests.rs` from Rust commit `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.

| Pinned source | Go destination | Git source blob | Source SHA-256 |
| --- | --- | --- | --- |
| `compiler/rustc_data_structures/src/base_n.rs` | `port/rust/compiler/rustc_data_structures/src/base_n.go` | `0c3d7613d4f1ff69a31c4458bd571074bbd5c618` | `090d62952cb4515c9de66489d6ce0e0f3cd5c3becd7e8e2c2bf39d119d4f9890` |
| `compiler/rustc_data_structures/src/base_n/tests.rs` | `port/rust/compiler/rustc_data_structures/src/base_n/tests_test.go` | `148d8dde02af1b12433fa87509a742d1e59c3421` | `e4a58e24148ca8120d30d974fc4084ab08e0aa9ea2fd1d4b1101eb21c9155cde` |

The original files are MIT OR Apache-2.0; the notices and license texts are preserved under `port/rust/`. This port covers these two whole files, not the rest of `rustc_data_structures` or a working production compiler.

## Complete API mapping

The mirrored `src` folder uses package `datastructures`. Names have a `BaseN` prefix because additional upstream Rust modules may occupy this same Go package later.

| Rust item | Go item | Preserved behavior |
| --- | --- | --- |
| `MAX_BASE`, `ALPHANUMERIC_ONLY`, `CASE_INSENSITIVE` | `BaseNMaxBase`, `BaseNAlphanumericOnly`, `BaseNCaseInsensitive` | Constants 64, 62 and 36 |
| `BASE_64` | private `baseNDigits` constant | Exact ASCII alphabet `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@$` |
| `u128` input | `BaseNU128{Hi, Lo uint64}` | All 128 bits, including values beyond `uint64`, remain intact |
| Public `ToBaseN: Into<u128>` trait | `BaseNValue`, `BaseNToBase`, `BaseNToBaseFixedLen` | External custom types supply conversion and associated encoded length; both default methods can be overridden independently; fixed default dispatches an ordinary-encoding override before querying length |
| Trait implementations for built-in unsigned widths | `BaseNU32`, `BaseNU64`, `BaseNU128` | Named adapters satisfy the public extension interface; width-specific free functions remain available for native Go integer inputs |
| `BaseNString` | `BaseNString` | Private start offset and private 128-byte backing buffer, copied by value |
| `Deref<Target = str>`, `AsRef<str>`, `Display` | `String()`, `AsRef()`, `fmt.Stringer` | The initialized ASCII suffix, with no terminator; Go strings copy the suffix into immutable storage |
| `ToBaseN::to_base` for `u32`, `u64`, `u128` | `BaseNToBase32`, `BaseNToBase64`, `BaseNToBase128` | Minimal representation; zero emits one digit; repeated exact division and remainder; fixed buffer; original digit lookup and failure behavior |
| `ToBaseN::to_base_fixed_len` for all three types | `BaseNToBaseFixedLen32`, `BaseNToBaseFixedLen64`, `BaseNToBaseFixedLen128` | Encode first, then expose leading zero digits up to the declared input width's maximum encoded length |
| `ToBaseN::encoded_len` for all three types | `BaseNEncodedLen32`, `BaseNEncodedLen64`, `BaseNEncodedLen128` | Divide the corresponding type's maximum by the base until zero; use the corresponding narrow divisor cast |
| Test `limits` | `TestLimits` | All three maximum-value base-36 round trips |
| Test `test_to_base` | `TestToBase` | Every base 2 through 36, all seven explicit boundary values and all 1,000 `i * 983` values, checking normal and fixed-length encodings |

Go cannot add methods to the built-in `uint32` or `uint64` types, so named adapters implement the extension interface and explicit width-specific functions also accept the native types. A custom `BaseNValue` implements `BaseNInto128() BaseNU128` and `BaseNEncodedLen(uint) uint`; the latter describes its type's width, corresponding to Rust's associated function. Optional `BaseNToBase(uint) BaseNString` and `BaseNToBaseFixedLen(uint) BaseNString` methods override the free functions' defaults. A width of 5 can therefore encode 7 as `00007`. Short widths truncate the initialized suffix exactly as upstream, zero width yields an empty string and widths over 128 panic during the fixed-length call itself. The default converts into u128 once; a normal override bypasses conversion, and a fixed override bypasses both defaults and length calculation.

`BaseNU128` is a numeric representation, not an emulated floating-point value or a truncated `uint64`. `math/bits.Div64` implements the two-limb division, after dividing the high limb to ensure its remainder satisfies `Div64`'s precondition. No arbitrary-precision library is required by production code. The original final `assert_eq!(n, 0)` is guaranteed by the loop's only successful exit condition.

The string's Go zero value is not an encoded number. As with the private-field Rust structure, callers obtain initialized strings through the encoding functions. Go uses explicit `String`/`AsRef` calls where Rust permits implicit dereferencing; native language formatting flags are outside this API mapping.

## Supported bases, ordering and invalid inputs

Every integer width supports bases 2 through 64. Minimal strings omit leading zero digits. Fixed-length strings have the same length for every value of one declared integer width. In bases 2 through 36, these fixed-width strings preserve numeric ordering under ASCII lexical comparison. In higher bases, the original alphabet places uppercase letters after lowercase letters numerically; this does not preserve ASCII lexical order. The port retains that alphabet exactly.

The original source has no initial base-range assertion. The Go port retains operation-level failures rather than introducing a broader rejection:

- Division by zero panics, including a base that becomes zero when `encoded_len` casts it to `u32`.
- Digit indices 64 and above panic when reached. An oversized base can still successfully encode a value whose digits remain within the alphabet.
- Base one encodes zero as `0`; a positive value exhausts the 128-byte buffer and panics. Its length loop does not reduce the maximum value, and a fixed-length zero encoding reaches that loop. These are unsupported inputs and are excluded from the terminating oracle stream.
- Fixed-length encoding performs ordinary encoding before calculating the width, as the original default trait method does. Native 64-bit `usize` casts beyond the supported range are covered by the failure-contract oracle. On 32-bit Go hosts those unrepresentable base arguments are explicitly skipped; all supported-base cases still run.

Panic text is a language-runtime detail: the port preserves panic versus success and evaluation order, not Rust's panic diagnostic wording. Both ordinary and checked-build semantics agree throughout the supported domain. For the unsupported base-one length loop, eventual native counter overflow differs between Rust build profiles and is not a usable API result.

## Original-source validation

`main.go` reads both original files from their exact pinned Git objects, checks their blob IDs and raw SHA-256 hashes, and copies their bytes unchanged to a temporary directory. `oracle.rs` is a separate protocol wrapper. It enables the `ascii_char` and `ascii_char_variants` features needed by the pinned source; `RUSTC_BOOTSTRAP=1` applies only to this validation compiler process. Neither original file is patched.

The tool compiles and executes the entire original two-test module. A separate original-source executable returns encodings, lengths and caught failures for a deterministic input stream. That stream covers all bases 2 through 64 and all three integer widths, every power-of-two boundary with neighboring values, maximum values, zero, limb boundaries and deterministic random inputs. The compressed committed fixture contains 74,781 supported-base cases and 543 invalid-base return/panic cases. Another 1,225 external-newtype cases exercise the public trait's custom lengths and independent default-method overrides, comparing exact conversion/encoding/length/override call traces even when a call panics. Custom widths include 0, 1, 5, 128, 129 and `usize::MAX`; their length functions remain well-defined even for bases zero and one. Both compressed and raw fixture data, source files, wrapper and request stream have recorded checksums in `manifest.json`.

Ordinary Go tests compare the Go implementation to these independent Rust answers without requiring Rust, Git or network access. They also run the entire translated upstream test module, check exact digit spelling and string/display conversion, verify fixed-length ordering and fuzz independent `math/big` round trips. A live opt-in test recompiles the original source and compares its complete stream to the committed fixture; explicitly requesting an unavailable or different Rust compiler fails.

```sh
go test ./port/rust/compiler/rustc_data_structures/src/base_n
FERRUGO_RUSTC=/path/to/rustc go test ./port/rust/compiler/rustc_data_structures/src/base_n
go run ./tools/port-rust-base-n -check
```

Generation defaults to `.toolchain/upstream-rust` and `.toolchain/rust/bin/rustc` relative to the repository root. `-root`, `-rust-source` and `-rustc` override them; `FERRUGO_RUST_SOURCE` and `FERRUGO_RUSTC` supply optional defaults. Compiler names resolve on `PATH`, while relative compiler paths resolve against the repository root. `-check` leaves generated files unchanged and checks the decompressed original stream as well as manifest integrity. Omitting it regenerates the fixture and manifest. The tool never fetches missing objects implicitly or changes an upstream checkout.

The validation compiler is pinned to Rust 1.99.1 (`58cf8f9c3`, 2026-10-13), on a 64-bit host, with overflow checks enabled. This integer-only module has no Unicode profile or external dependency. Production execution uses only Go's standard library and never runs Rust, reads the oracle fixture, executes a subprocess or uses the network.
