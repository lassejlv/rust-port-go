# Ferrugo

**A Rust compiler and Cargo reimplementation in Go.**

Ferrugo's long-term goal is to compile Rust projects with a compiler and package manager implemented in Go. This repository starts that effort with an independent, deliberately bounded bootstrap compiler. It is **not a complete port of rustc or Cargo** and cannot compile the upstream compiler or arbitrary crates.

## First goal

Compile and run a small, typed Rust binary without invoking `rustc` or upstream `cargo`, then build that same binary from a `Cargo.toml` using Ferrugo's package command. Keep supported behavior tested, reject unsupported features, and grow compatibility in reviewable checkpoints on `main`.

The executable bootstrap works. The full port remains in progress. See [supported behavior and limits](docs/COMPATIBILITY.md), [Cargo commands](docs/CARGO.md), [the roadmap](docs/ROADMAP.md), and [progress log](docs/PROGRESS.md), and [upstream scope](docs/UPSTREAM.md).

Current goal: [host integer semantics](docs/GOALS.md), following the implemented module, Copy-record and local Cargo dependency milestones.

## Build and try it

Install Go 1.23 or newer and a C11 compiler (`cc`, GCC, or Clang). The Go implementation uses only the standard library.

```sh
make build
./bin/ferrugo-cargo run --manifest-path examples/hello/Cargo.toml
./bin/ferrugo-cargo run --manifest-path examples/local-packages/app/Cargo.toml
./bin/ferrugo --check examples/hello/src/main.rs
./bin/ferrugo examples/hello/src/main.rs -o /tmp/hello-ferrugo
/tmp/hello-ferrugo
./bin/ferrugo --emit=c examples/hello/src/main.rs -o /tmp/hello-ferrugo.c
make check
# Optional: compare the committed fixtures with an installed rustc
FERRUGO_RUSTC=rustc FERRUGO_CARGO=cargo go test -count=1 ./internal/compiler ./internal/conformance ./internal/cargoconformance
```

To start a package:

```sh
./bin/ferrugo-cargo new /tmp/my-ferrugo-app
./bin/ferrugo-cargo run --manifest-path /tmp/my-ferrugo-app/Cargo.toml
```

Set `CC` to a C compiler executable if needed. `make GO=/path/to/go build` selects a Go toolchain. `--check` and `--emit=c` do not invoke the C compiler. Both Cargo output profiles currently use checked arithmetic and the same backend settings.

## Architecture

```text
Rust files → Go lexer/parser + module loader → Go resolver/type checker → C11 → C compiler → executable
Cargo.toml → Go manifest/dependency-graph commands ────────────┘
```

The C backend is a bootstrap strategy. It does not provide Rust's ownership model, ABI, standard library, or full language semantics. Unsupported constructs must produce an error; delegating compilation to rustc is not an implementation strategy.

## Upstream references

- [rust-lang/rust](https://github.com/rust-lang/rust): compiler, libraries, bootstrap, and conformance tests.
- [rust-lang/cargo](https://github.com/rust-lang/cargo): package manager, dependency resolution, registry protocols, and project commands.

Ferrugo is an independent project and is not affiliated with the Rust project. Upstream source and test material retain their original licenses and attribution. New implementation code is MIT licensed; see [LICENSE](LICENSE).
