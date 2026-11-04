This development helper executes the unchanged pinned `edition.rs`, including
all original derives. It builds the complete actual `rustc_macros`,
`rustc_serialize`, and `rustc_hashes` sources and uses the real
`rustc-stable-hash` implementation.

```sh
python3 tools/port-rust-edition/reference-build/prepare.py --fetch --validate
```

After the original Git objects and exact registry archives are cached, use
`--offline --validate`. Offline mode also sets `GIT_NO_LAZY_FETCH=1` for Git
reads. `FERRUGO_CARGO`, `FERRUGO_RUSTC`, and `CARGO_HOME` select installed tools;
defaults use the repository's `.toolchain` installations.

The source pin declares Rust 1.101.0. The tested development compiler is Rust
1.99.1 with `RUSTC_BOOTSTRAP=1` and
`-Zcrate-attr=feature(never_type)`. The extra flag enables a feature gate still
present in that compiler; original implementation bodies remain unchanged.

The `rustc_span` and `rustc_data_structures` packages in this workspace are
bounded reference facades. They retain the exact original relevant codec and
stable-hash trait definitions, dummy stable-hash context, and primitive `u8`
stable-hash implementation. Unused span, symbol, definition-ID and fingerprint
interfaces have opaque types and fail-fast sentinels. They never supply dummy
values. The proof must exercise all five editions and all 251 invalid enum tags
while reaching none of those interfaces. These facades grant no source-port
credit and do not represent complete builds of those two crates.

`AuditEncoder` and `AuditDecoder` record typed primitive operations and forward
them to the actual memory codec. The stable-hash proof uses the actual SipHasher
with different caller prefixes. The retained expansion and audit show that the
edition needs only `emit_u8`, `read_u8`, standard `write_isize`, and primitive
stable hashing of a `u8`.

The ignored workspace `.toolchain/oracles/edition-reference` retains full
original files, hashes and source ranges for selected contracts, the complete
derive expansion, proof logs, exact compiler flags and registry checksums.
Before compilation, every extracted registry file is compared with its verified
archive, including normalized manifests; symlinks and unexpected files are
rejected. The profile retains the verified file counts and tree hashes.
`build-profile.json` includes `library` and `link_arguments` for an independent
Rust oracle. The `edition_reference` crate reexports the unchanged edition,
actual codecs and hash types, recording adapters, and unused-interface counter.
