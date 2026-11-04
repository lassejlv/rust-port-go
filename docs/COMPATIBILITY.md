# Bootstrap compatibility

Ferrugo 0.1.0 implements the subset below. It is not yet compatible with complete Rust or Cargo. The implementation is new Go code organized around compiler phases, not a mechanical translation of all upstream sources.

## Accepted language

- A binary crate with nested inline/external `mod` items and `fn` items; a required unit-returning, zero-argument `main` entrypoint, declared at the root or imported there.
- Public/private functions and modules, relative and `self`/`super`/`crate` qualified calls, simple `use` aliases for functions/modules, and source-file diagnostics. External modules use `name.rs` or `name/mod.rs`; ambiguous/missing files and cycles reject.
- Valid UTF-8 source, ASCII identifiers, line comments, nested block comments, source line/column errors.
- Types `i32`, `i64`, `bool`, `&str`; `()` return types and unit expressions.
- Parameters with explicit types; explicitly typed function returns, default unit returns.
- Decimal/binary/octal/hexadecimal integers, underscores, optional `i32`/`i64` suffixes, signed minima; unsuffixed integers default to `i32` with limited contextual inference.
- UTF-8 string literals and common escapes, including embedded NUL. CRLF source text in strings normalizes to LF; escaped CRLF line continuations are supported. Strings carry lengths at runtime.
- `let`, optional type annotations, `mut`, lexical shadowing and nested scopes.
- `=`, `+=`, `-=`, `*=`, `/=`, `%=` assignments to mutable locals.
- Integer `+ - * / %`, unary `-`; integer comparisons; boolean `! && || == !=`; string `== !=`.
- Parentheses, typed function calls, forward calls and recursion.
- `if`/`else if`/`else`, `while`, `break`, `continue`, `return` and nested block statements.
- Value-producing `if`/`else` and block expressions, function tails, and unit expressions. Branch results must agree; no-else `if` expressions require unit. Expression statements without a semicolon require unit.
- `print!`/`println!` with literal formats, sequential `{}` placeholders and `{{`/`}}` escaped braces; `println!()`; `assert!(boolean)`.

The C11 backend generates native executables using libc. Operands and arguments evaluate left to right; `&&`/`||` short-circuit; signed integer overflow, division/remainder by zero and failed assertions exit with status 101. These runtime checks apply in both output profiles. Generated programs do not need Go or Rust installed.

## Known limits

There is no ownership/borrow checker yet: the accepted subset has copyable scalar values and static string literals only. There are no references beyond literal-backed `&str`, heap allocation, user-defined types, traits, generics, closures, patterns, arrays/slices, enums, structs, statics, const evaluation, async, unsafe, FFI, inline assembly, procedural or declarative macros, attributes, conditional compilation, standard library APIs, argument/input APIs, incremental compilation or Rust ABI support.

Public reexports, restricted visibility, grouped/glob/absolute imports, local module/import items, and shared function/module/import names in one namespace remain unsupported. Module nesting is bounded to 256 levels. Source-string compiler APIs support inline modules only; file-based APIs load external declarations.

Unit-valued bindings/parameters, integer bitwise operations, non-ASCII identifiers, raw/byte strings and general format specifications are unsupported. Return/break/continue require semicolons. `&str` returns require exactly one `&str` parameter under Rust lifetime elision; explicit lifetime syntax is unsupported. Inference is local and intentionally more restrictive than Rust inference across later statements. Compile-time constant-overflow linting is not implemented; arithmetic overflow is checked at runtime. Edition declarations do not select complete edition-specific grammars.

Diagnostics have source locations but do not reproduce rustc error codes, wording or recovery. Successful C emission does not establish cross-target or ABI compatibility. Local validation is on Linux using GCC; CI also exercises macOS. The backend uses the host C compiler and has no cross-compilation support yet.

See [Cargo command boundaries](CARGO.md) and the [remaining roadmap](ROADMAP.md). Unsupported syntax, types, commands, manifests and targets fail explicitly; a test passing for this subset does not imply upstream conformance.
