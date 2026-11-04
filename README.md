# Ferrugo

**A Rust compiler and Cargo reimplementation in Go.**

Ferrugo's long-term goal is to compile Rust projects with a compiler and package manager implemented in Go. This repository starts that effort with an independent, deliberately bounded bootstrap compiler. It is **not a complete port of rustc or Cargo** and cannot compile the upstream compiler or arbitrary crates.

## First goal

Compile and run a small, typed Rust binary without invoking `rustc` or upstream `cargo`, then build that same binary from a `Cargo.toml` using Ferrugo's package command. Keep supported behavior tested, reject unsupported features, and grow compatibility in reviewable checkpoints on `main`.

Implementation is in progress. See [the roadmap](docs/ROADMAP.md), [interfaces](docs/INTERFACES.md), and [progress log](docs/PROGRESS.md).

## Architecture

```text
Rust source → Go lexer/parser → Go type checker → C11 → system C compiler → executable
Cargo.toml → Go manifest/project commands ────────────┘
```

The C backend is a bootstrap strategy. It does not provide Rust's ownership model, ABI, standard library, or full language semantics. Unsupported constructs must produce an error; delegating compilation to rustc is not an implementation strategy.

## Upstream references

- [rust-lang/rust](https://github.com/rust-lang/rust): compiler, libraries, bootstrap, and conformance tests.
- [rust-lang/cargo](https://github.com/rust-lang/cargo): package manager, dependency resolution, registry protocols, and project commands.

Ferrugo is an independent project and is not affiliated with the Rust project. Upstream source and test material retain their original licenses and attribution. New implementation code is MIT licensed; see [LICENSE](LICENSE).
