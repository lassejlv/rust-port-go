# Shared complete rustc_span native reference

This development tool compiles the entire unchanged `rustc_span` crate and all
its sibling modules at Rust commit
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`. Ten complete compiler crates and
108 exact registry dependencies provide real macros, codecs, stable hashing,
session globals, hygiene, interning, source maps and locks. There are no fake
runtime branches, replacement algorithm bodies or source completion credits
for this build helper.

The compiler source version is 1.101.0, dated 2026-11-04. The earlier installed
Rust 1.99.1 cannot build the complete graph because `allocator_ext` is unknown.
The official dated nightly below builds every body unchanged. Previous 1.99
tools and evidence retain their original compiler and standard-library profile.

```sh
python3 tools/port-rust-span-reference/install.py --fetch
python3 tools/port-rust-span-reference/prepare.py --fetch-source --fetch-deps
```

The installation is separate at `.toolchain/rust-nightly-2026-11-04`, preserving
`.toolchain/rust`. `toolchain.json` pins all official archive SHA-256 values,
Rust `1.101.0-nightly (708c94ef0 2026-11-03)`, Cargo
`1.101.0-nightly (31d3246e0 2026-10-27)`, Unicode 18 and `CFG_RELEASE=1.101.0`.
The initial installer target is Linux x86-64; an explicitly selected matching
native compiler/Cargo on another 64-bit host can use the preparation tool.
No injected feature attribute or bootstrap cfg is needed. `RUSTC_BOOTSTRAP=1`
acknowledges the original compiler macro build script and permits source crate
features; source bytes remain unchanged.

After cold preparation, omit fetch flags for strictly offline operation:

```sh
python3 tools/port-rust-span-reference/prepare.py
python3 tools/port-rust-span-reference/prepare.py --debug-assertions false --overflow-checks true
```

The source checkout defaults to `.toolchain/upstream-rust`, registry cache to
`.toolchain/port-cargo-home` and output to
`.toolchain/oracles/rustc-span-reference`. `FERRUGO_RUST_SOURCE`, `CARGO_HOME`,
`FERRUGO_RUSTC`, `FERRUGO_RUSTDOC` and `FERRUGO_CARGO` override those defaults. CLI `--source`,
`--cargo-home`, `--rustc`, `--rustdoc`, `--cargo` and `--output` override environment values.
Explicit empty/invalid values fail. Bare binaries resolve through PATH; relative
paths resolve at the repository root. Executable symlink basenames are preserved
for Rustup dispatch; source/cache identities are canonicalized separately.
Matching rustdoc defaults to the selected rustc's sibling. Python 3.11 or newer
on a POSIX host is required.

`sources.json` binds every original file's Git blob, byte count and SHA-256.
Fetches request only exact missing blobs and do not mutate the upstream index
or branch. Original compiler manifests are retained; only the reduced enclosing
reference workspace manifest is generated. The selected lock is checked against
the original complete lock. Every selected registry archive and every expanded
source file is verified before and after compilation. Existing unowned output
directories, overlaps with the source cache and edited copied source bodies are
rejected.

Each build executes all 43 original rustc_span unit tests and an external client
that runs the genuine session, interner and root hygiene. Independent assertion
and overflow flags select `target/d#o#` and `build-profile-d#o#.json`. The canonical
`build-profile.json` always describes the checked/debug profile, so another
profile cannot change a sibling driver's default silently.

The profile contract is schema 1:

- `rustc`, `cargo`, version identities and configuration describe the reference.
- `artifacts.<compiler_crate>.filenames` lists real output files; `library` is the
  code archive or proc macro library. `rustc_macros` exposes its actual dylib.
- `package_artifacts` retains registry artifact candidates without hiding
  duplicate crate names/versions.
- `link_arguments` is the verified complete argument list for an external native
  driver, including edition, independent overflow/assertion flags, dependency
  directories and compiler crate imports.
- `source-provenance.json` binds source, registry checks, tool code and lock;
  its digest is recorded by every build profile.

Cargo/rustc 1.101 split full metadata out of rlib. Drivers must use the complete
`link_arguments`, which includes both `--extern name=...rmeta` and
`--extern name=...rlib`. Passing the `library` field alone is insufficient.
Driver-only Fn traits or other test features can use explicit native attributes;
they do not alter the original modules. Helpers receive no Rust/Go source-file
completion credit merely by compiling the original reference.
