# Compiler fixed-width hash types

The complete `compiler/rustc_hashes/src/lib.rs` at Rust revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` maps to
`port/rust/compiler/rustc_hashes/src/lib.go`.
Its Git blob is `c8f3057dac5fc26da8c243f95d8a978ee8cc00b7` and raw SHA-256 is
`ecb7777ffca8e89ad7056cc864771983a3eb4384049d739fbd5b562ed2d53850`.
The original MIT/Apache licenses remain under `port/rust/`.

Both hash value types include every constructor/conversion, zero/default value,
wrapping addition, cloning, equality, ordering and typed `Hash` operation.
`Hash64.XorAssign` implements the source's only bitwise assignment trait.
`Hash128.Hash` deliberately sends only its low 64 bits to `WriteU64`, preserving
the original behavior. Hash callbacks retain caller state, overrides and panics.

`Uint128{Lo, Hi}` replaces Rust's native unsigned 128-bit value with two copied
64-bit limbs. Arithmetic uses carry propagation, comparison orders high limbs
first, and stable-hash conversions preserve the original low/high limb order.
Go assignment/native equality implement Copy/Eq; explicit Clone/CloneFrom and
Compare expose the other derived operations. Zero-value constructors replace
immutable struct constants without exposing a mutable package variable.
These adapters do not promise a Rust ABI or native u128 alignment.

The original Debug and LowerHex implementations directly forward to standard
integer formatting. The primary Go methods forward to a caller-owned
`Formatter`'s corresponding unsigned primitive operation, returning its error.
`integer_format.go` supplies the used unsigned integer formatting contract:
decimal and Debug hexadecimal modes, alternate prefixes, sign, width,
alignment, scalar fill, sign-aware zero padding, ignored integer precision,
fallible output and independent character-write overrides. It receives no
completion credit for any standard-library implementation file.

`NumericFormatter` is mutable, like Rust's formatter. Successful padding restores
the original options. A writer failure during zero padding retains the temporary
zero fill and right alignment, exactly as the original does. Optional
`WriteChar(rune) error` models `fmt::Write::write_char`; otherwise scalar writes
use UTF-8 `write_str`. An `io.Writer` short write without an error is a Go-only
case translated to `io.ErrShortWrite`. Valid Rust formatter widths in the tested
standard-library profile fit 16 bits; extra Go widths have no Rust-domain claim.
Go `%v` and `%x` are convenience Debug/LowerHex adapters; the full primary API
supports custom fill/center alignment and upper-hex Debug modes.

The reference compiles the entire byte-verified original file unchanged against
the exact locked `rustc-stable-hash 0.1.2` archive. The dependency's expanded
files are checked against that archive before compilation. Its two-limb output
type is adapted as `StableHasherHash`; its hashing algorithm is not replaced or
claimed ported here. Despite the source module's documentation about encoding,
this file contains no Encoder/Decoder implementation, so none is invented.

The pinned compiler source is version 1.101.0. Reference execution uses the
recorded Rust 1.99.1 standard library on Linux x86-64 without development gates.
The Go implementation never invokes Rust or Cargo. Full compiler integration,
other hash/serialization modules and production compatibility remain unfinished.
