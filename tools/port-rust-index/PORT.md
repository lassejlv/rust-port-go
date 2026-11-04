# Complete typed-index source port

Pinned source: `rust-lang/rust`
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`,
`compiler/rustc_index/src/idx.rs` (Git blob
`60c4dd90d8f666fc855b5a5e6e22f07209dfd1f8`).

The entire 145-line/3,793-byte file maps to:

- `port/rust/compiler/rustc_index/src/idx.go`: the `Idx` trait adapter, both
  integer implementations, `IntoSliceIdx`, and all seven standard conversions;
- `port/rust/compiler/rustc_index/src/idx_nightly.go`: all four
  `cfg(feature = "nightly")` core-range conversions, enabled by Go build tag
  `ferrugo_rustc_nightly`.

This is one completed upstream source file. The new `idx*_test.go` files are
validation support; the pinned source has no separate upstream test file.
Neither the general `IndexSlice`/`IndexVec` containers nor standard-library
range iteration or slicing receive source-port completion credit here.

## Traits and integer semantics

`Idx[Key any]` supplies required `FromIndex` and `ToIndex` callbacks and the
actual `New`, `Index`, `Plus`, and `IncrementBy` operations. Optional
`PlusOverride` and `IncrementByOverride` callbacks independently represent
concrete trait-method overrides. Default increment invokes the selected plus
implementation; its assignment occurs only after plus succeeds. An increment
override receives the original mutable key and can mutate before panicking.

`NewIdx` uses checked overflow. `NewIdxWithConfig` accepts
`IdxConfig{OverflowChecks: false}` for Rust's unchecked unsigned wrapping
behavior. Default plus calls `Index`, adds using the current host's `uint`
width (`usize`), and calls `New`; checked overflow panics before `New` runs.
`UsizeIdx` and `U32Idx` mirror the complete upstream implementations. The u32
constructor's range assertion remains active in either overflow profile.

`Key any` permits reference-like metadata represented by Go slices; there is no
extra Go `comparable` constraint. Adapter clients must supply keys with the
lawful Copy, Eq/PartialEq, Debug, and Hash behavior required by Rust's `Idx`
supertraits wherever their consumers use those traits. This file does not
implement or claim completion of those general Rust traits. Go's type system
does not enforce their laws, Rust lifetimes, or the must-use return annotation.

## Slice-index conversion

`IntoSliceIdx[Input, Output]` is the public conversion callback adapter, so
downstream custom implementations can choose their own implementing and
associated output types. The output must obey the downstream slice-index
contract; the adapter does not impose an unrelated Go representation.

Built-in conversion factories return `TypedSliceIndexResult`, whose tag retains
the exact output variant: single index, full range, standard Range/From/To/
Inclusive/ToInclusive, and the four distinct core variants. Inclusive standard
ranges retain `End`; core inclusive ranges retain `Last`. Full/open ranges
remain full/open. Endpoint conversions execute in source order and stop at the
same callback panic. Conversion performs no slice bounds validation, inclusive
end increment, range-order rejection, or range normalization.

`RangeInclusive` accepts the endpoints actually returned by Rust's `start()` and
`end()`, along with its exhaustion state. Its conversion constructs a fresh
inclusive result and drops exhaustion, as the pinned implementation does.
The original Rust standard library's iterator may change endpoints on
exhaustion: consuming `0..=0` leaves `1..=0`; consuming
`usize::MAX..=usize::MAX` leaves `0..=usize::MAX` with exhaustion set. The
conversion keeps those endpoints but clears exhaustion, making the latter
result nonempty. The oracle captures the real pre-conversion iterator endpoints
and supplies those exact inputs to the Go adapter; no range iterator is
approximated or ported here.

## Original-source oracle

`oracle.rs` includes the entire original `idx.rs` without edits, shims, or
dependency replacements. It builds four profiles: standard/nightly trait
implementations, each with overflow checks enabled/disabled. The pinned Rust
1.99.1 compiler supports all core range types directly; no `RUSTC_BOOTSTRAP`
setting or feature-attribute shim is needed.

The oracle uses actual separate Rust concrete index types with default methods,
overridden plus, overridden increment, and both overrides. Keys contain static
slice metadata with lawful index-based Eq/Hash; their Go counterparts contain a
non-comparable `[]byte` field. It also exercises the original usize/u32 impls
and an independently implemented `IntoSliceIdx` conversion.

`manifest.json` records the source/toolchain pin, source/oracle/request/response
hashes, profile sizes, and committed compressed-fixture hash. Ordinary tests
are offline and use only Go. The reference fixture was produced for the pinned
64-bit Rust host; host-width-independent unit tests remain available elsewhere.

Run with Go 1.23 or later:

```sh
go test ./port/rust/compiler/rustc_index/src
go test -tags ferrugo_rustc_nightly ./port/rust/compiler/rustc_index/src
go run -tags ferrugo_rustc_nightly ./tools/port-rust-index -check
```

Omit `-check` to regenerate. Generator flags `-root`, `-rustc`, and
`-rust-source` select the repository and pinned local toolchain/source cache.
