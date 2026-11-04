# Exact scalar width reference

This development-only tool checks the scoped adapter described in
`port/dependencies/unicode-width/v0_2_2/PORTING.md`. It compiles the exact
SHA-verified unicode-width 0.2.2 archive unchanged and calls the builtin
`UnicodeWidthChar::width` for the complete Unicode scalar domain.

```sh
cargo fetch --locked --manifest-path tools/port-unicode-width-scalar/Cargo.toml
python3 tools/port-unicode-width-scalar/generate.py --check
go test ./port/dependencies/unicode-width/v0_2_2/src
```

Without `--check`, the generator writes the archive inventory, original licenses
and support metadata, compact exact ranges and compressed original response.
Check mode preserves all retained bytes; native build inputs live only under
`.toolchain/oracles/unicode-width-scalar`. Production hashing/compiler code does
not invoke this tool or Rust. Only the scalar function is adapted; the rest of
the dependency remains unported and receives no source-file completion credit.
The explicit Cargo fetch prepares a cold registry; the scalar checker itself
reads verified cached bytes and runs offline.
