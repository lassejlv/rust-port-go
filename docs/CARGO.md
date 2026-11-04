# Ferrugo Cargo bootstrap

`ferrugo-cargo` is implemented in Go and calls Ferrugo's compiler packages directly. It never invokes upstream Cargo or rustc. Binary packages can compile local library-only path dependencies, including transitive and renamed dependencies, into one native executable. Library packages can be checked. This remains a bounded bootstrap of Cargo.

```sh
ferrugo-cargo new hello
cd hello
ferrugo-cargo check
ferrugo-cargo build
ferrugo-cargo run
ferrugo-cargo run -- argument1 argument2
ferrugo-cargo build --release
ferrugo-cargo clean

# Create a library with a supported pub fn add template:
ferrugo-cargo new ../arithmetic --lib
ferrugo-cargo check --manifest-path ../arithmetic/Cargo.toml
```

Builds go to `target/debug/<package-name>` or `target/release/<package-name>` in the selected binary package. Both profiles currently use the same compiler settings; `--release` selects the output directory only. `run` forwards standard input, output, error, arguments and process exit status, and preserves the caller's working directory. The initial language runtime does not yet expose argument/input APIs.

`--manifest-path /path/to/Cargo.toml` selects a project for build, run, check or clean. Without it, the command searches the current directory and its parents. `check` parses and type-checks the entire dependency graph without producing artifacts or creating target directories. `build` and `run` reject library-only roots explicitly: Rust rlib/ABI-compatible and standalone native-library artifacts are not implemented. No placeholder library files are emitted.

`new` requires a nonexistent destination whose parent exists. `init [path]` requires an existing directory. Both create `Cargo.toml` and a single source root: `src/main.rs` by default or with `--bin`, and `src/lib.rs` with `--lib`. The two flags cannot be combined. `--name` and `--vcs none` are supported; version control setup is skipped by default. Existing source and manifest files are never overwritten. The library template contains a supported public addition function, without unsupported test/attribute machinery.

## Manifests and local dependencies

The manifest parser accepts `[package]` with required `name` and `version`, optional `edition` (`2015`, `2018`, `2021`, `2024`; default `2015`), and single-line string metadata `description`, `license`, `repository`, `homepage`, `documentation` and `readme`. One crate root must exist: either `src/main.rs` or `src/lib.rs`. Dual binary/library packages, additional `src/bin` targets, custom target tables and build scripts are rejected. Supported inline/external module declarations load supporting Rust files relative to the crate root. Edition declarations are recorded and validated; the documented bootstrap grammar remains the same for each edition.

`[dependencies]` accepts bare ASCII dependency names and single-line inline tables containing a nonempty `path` and optional `package` rename:

```toml
[dependencies]
math-lib = { path = "../math-lib" }
numbers = { path = "../numeric-core", package = "numeric-core" }
```

Paths resolve relative to the declaring manifest and must lead to library-only packages with their own `Cargo.toml` and `src/lib.rs`. The dependency key must match that package's name unless `package` supplies its name explicitly. Hyphens in dependency keys become underscores for Rust paths: the example exposes `math_lib` and `numbers`. Aliases must be valid supported Rust identifiers; aliases that collide after normalization are rejected.

The loader canonicalizes manifest parent directories, preserves a manifest-file symlink’s selected package directory, interns shared packages and rejects dependency cycles, missing manifests, package-name mismatches, binary dependencies, duplicate aliases for one direct package, and distinct paths declaring the same package name and version. Package identity follows the canonical manifest, even when different packages share a source-file symlink. A library used through multiple branches retains one shared type identity. Dependencies keep their own crate roots, visibility and extern aliases; source code must use the declared dependency aliases.

Only `path` and `package` dependency fields are supported. Registry/version/git dependencies, features, optional/default-feature settings, workspaces, profiles, dev/build dependencies, nested dependency tables and other manifest settings are rejected. The external dependency alias `std` is unsupported because replacing Rust's implicit standard prelude is not implemented. There is no dependency download, version solver, lockfile generation or incremental cache.

The bounded parser supports basic and literal single-line TOML strings, quoted commas/hash characters within strings, and comments outside strings. Raw source must be valid UTF-8 and obey TOML whitespace/control rules; escaped Unicode controls remain valid string content. Arrays, multiline values, quoted dependency keys and other TOML forms outside this subset are rejected.

Each successful binary build records an ownership sidecar. `clean` removes only the selected package's recorded Ferrugo binaries and markers, preserving unrelated files in `target`. It refuses symlinked artifact directories and non-regular artifact paths, and never recursively deletes target directories. Libraries are linked into the selected binary without creating target directories in dependency packages. Testing, packaging and publishing commands remain unsupported and fail explicitly.
