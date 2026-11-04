This development reference workspace builds the unchanged pinned Rust index
storage modules, all of `rustc_index_macros`, and the actual `rustc_serialize`
and `rustc_hashes` implementations. It grants no Go-port inventory credit.

Run from the repository root:

```sh
python3 tools/port-rust-index-storage/reference-build/prepare.py --fetch --validate
```

Use `--offline` after the pinned Git objects and registry archives are cached.
`FERRUGO_CARGO`, `FERRUGO_RUSTC`, and `CARGO_HOME` select installed tools and
the Cargo cache. The defaults use `.toolchain/rust` and
`.toolchain/port-cargo-home`.

The source pin declares Rust 1.101.0. With the installed Rust 1.99.1 compiler,
validation uses `RUSTC_BOOTSTRAP=1`; the nightly profile additionally uses
`-Zcrate-attr=feature(never_type)` because that compiler still gates `!` trait
implementations. This enables a compiler feature without editing any source
body. These settings apply only to development reference builds.

The ignored workspace at `.toolchain/oracles/index-storage-reference` contains
verified original files, normal and nightly macro expansions, and a
`build-profile.json` recording source hashes, registry archive checksums,
compiler versions, exact flags, test commands, and link arguments. Both profiles
run the original five vector tests, including nested-option niche sizes and
forward/backward `Step` behavior. The nightly executable also round-trips an
`IndexVec` through the actual memory encoder and decoder.

Other reference harnesses can depend on this generated workspace as the
`storage_reference` Cargo package. It reexports `Idx`, `IntoSliceIdx`,
`IndexSlice`, `IndexVec`, and `newtype_index`; enabling its `nightly` feature
also reexports the original serialization traits and `opaque` module. A caller
using the index macro should alias it with
`use storage_reference as rustc_index;`.
