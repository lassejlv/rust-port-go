# Ferrugo Cargo bootstrap

`ferrugo-cargo` is implemented in Go and calls Ferrugo's Go compiler packages directly. It never invokes upstream Cargo or rustc. This is a local single-binary bootstrap, not a replacement for Cargo.

```sh
ferrugo-cargo new hello
cd hello
ferrugo-cargo check
ferrugo-cargo build
ferrugo-cargo run
ferrugo-cargo run -- argument1 argument2
ferrugo-cargo build --release
ferrugo-cargo clean
```

Builds go to `target/debug/<package-name>` or `target/release/<package-name>`. Both profiles currently use the same compiler settings; `--release` selects the output directory only. `run` forwards standard input, output, error, arguments, and the program's exit status. It preserves the caller's working directory. The initial language runtime does not yet expose an argument or input API.

`--manifest-path /path/to/Cargo.toml` selects a project for build, run, check, or clean. Without it, the command searches the current directory and its parents. `check` performs parsing and type checking without creating a target directory. `new` requires a nonexistent destination whose parent exists. `init [path]` requires an existing directory. Both create `Cargo.toml` and `src/main.rs`, accept `--name`, `--bin`, and `--vcs none`, and never overwrite existing source or manifest files. Version control setup is skipped by default.

The manifest parser accepts a single `[package]` table with required `name` and `version`, optional `edition` (`2015`, `2018`, `2021`, `2024`; default `2015`), and single-line string metadata `description`, `license`, `repository`, `homepage`, `documentation`, and `readme`. It supports basic and literal single-line TOML strings and comments outside strings. Source must be valid UTF-8 with TOML whitespace and control-character rules; escaped Unicode controls remain valid string content. Other TOML forms, unknown keys and tables, dependencies, workspaces, profiles, custom targets, libraries, and build scripts are rejected. Presence of `build.rs`, `src/lib.rs`, or `src/bin` is rejected too. Only `src/main.rs` is compiled. Rust edition declarations are recorded and validated; syntax remains the documented bootstrap subset for all editions.

Each successful binary build records a small ownership sidecar. `clean` removes only Ferrugo's recorded binaries and their markers, preserving unrelated files in `target`. It refuses symlinked artifact directories and non-regular artifact paths. It does not recursively delete target directories. Dependency fetching, registries, lockfiles, feature resolution, incremental caching, workspaces, testing, packaging and publishing remain unimplemented; unsupported commands fail explicitly.
