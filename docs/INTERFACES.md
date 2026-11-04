# Bootstrap interfaces

Module: github.com/lassejlv/rust-port-go. All work stays on main. Agents edit their assigned files; the coordinator commits and synchronizes checkpoints.

`internal/syntax.Parse(source string) (*Program, error)` parses the AST in `ast.go`. Errors identify line and column. Function return types default to `()`. Supported type spellings: i32, i64, bool, &str and (). Expression literals keep numeric spelling and decoded string text. Unary operands use Right; conditional expressions use Left as condition and Body/Else for branch blocks. No-semicolon expression statements store Type="()" as their expected type. No implicit externals or ignored unsupported syntax.

`internal/compiler.Check(source string) error` parses and type checks a complete binary crate. `internal/compiler.EmitC(source string) (string, error)` checks and lowers to portable C11. `internal/compiler.Build(sourcePath, outputPath string) error` reads Rust source, emits C to a temporary file and invokes system cc. Compilation never invokes rustc or cargo. Generated executables use libc. Build emits an executable at outputPath.

Cargo package: `internal/cargo.Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int`. CLI entrypoint `cmd/ferrugo-cargo`. Support new/init/build/run/check/clean/version/help for a single binary package with no dependencies. Calls compiler.Build/Check directly. Parse Cargo.toml with an explicitly bounded parser and reject dependencies, build scripts, workspaces and unsupported targets rather than ignoring them. No shell interpolation. CLI errors exit nonzero; child exit status preserved. README is coordinator-owned; cargo agent can write docs/CARGO.md and package tests.

Compiler CLI entrypoint `cmd/ferrugo`, owned by coordinator. Codegen supports lexical scopes, let/mut, assignments, integer arithmetic, booleans, functions/recursion, if/else, while, return/break/continue, println!/print!/assert! (bounded formats), value-producing if/block expressions, unit expressions and function tails. Unsupported Rust constructs must fail clearly. An initial implementation is a bootstrap subset, never full Rust compatibility.
