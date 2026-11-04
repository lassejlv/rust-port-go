# Bootstrap compatibility

Ferrugo 0.1.0 implements the bootstrap subset below. It is not yet compatible with complete Rust or Cargo. The bootstrap is new Go code organized around compiler phases. Direct upstream file ports are tracked separately under `port/`; their existence does not extend the bootstrap's accepted language until integration and compatibility checks pass.

## Accepted language

- A binary crate with nested inline/external `mod` items and `fn` items; a required unit-returning, zero-argument `main` entrypoint, declared at the root or imported there.
- Public/private functions and modules, relative and `self`/`super`/`crate` qualified calls, simple `use` aliases for functions/modules, and source-file diagnostics. External modules use `name.rs` or `name/mod.rs`; ambiguous/missing files and cycles reject.
- Valid UTF-8 source, Rust Pattern_White_Space, ASCII identifiers, line comments, nested block comments, source line/column errors. Source boundaries come from the complete source-ported low-level lexer; the frontend adapter rejects token kinds outside this subset. Doc comments are skipped; documentation/attribute expansion is not implemented.
- Signed `i8`, `i16`, `i32`, `i64`, `isize` and unsigned `u8`, `u16`, `u32`, `u64`, `usize`; `bool`, `&str`, `()` return types and unit expressions. Pointer integers use the host pointer width.
- Parameters with explicit types; explicitly typed function returns, default unit returns.
- Brace-style named records deriving exactly `Copy` and `Clone`, with supported integer/boolean or nested Copy-record fields; named type paths and simple type imports. Constructors support named/shorthand fields and preserve written evaluation order.
- Record values in locals, function parameters/results and branches, field reads, and field assignments rooted in mutable locals; type and field privacy is checked.
- Owned fixed-size Copy arrays: `[T; N]`, lists/repeats, nested arrays/records, function values, `usize` indexing, mutable indexed places and array `.len()`. Lengths are unsuffixed/usize literals up to 65536; elements are supported integers, bool, Copy records or nested arrays. Repeat operands evaluate once even when N is zero.
- Decimal/binary/octal/hexadecimal integers, underscores, optional supported integer suffixes, signed minima; unsuffixed integers default to `i32` with limited contextual inference.
- UTF-8 string literals and common escapes, including embedded NUL. CRLF source text in strings normalizes to LF; escaped CRLF line continuations are supported. Strings carry lengths at runtime.
- `let`, optional type annotations, `mut`, lexical shadowing and nested scopes.
- `=`, `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `|=`, `^=`, `<<=`, `>>=` assignments to mutable locals or supported record places.
- Integer `+ - * / %`, signed unary `-`, bitwise `! & | ^`, shifts `<< >>`, and comparisons. Boolean `! && || == != & | ^`; string `== !=`. Shift counts may use any supported integer type.
- Parentheses, typed function calls, forward calls and recursion.
- `as` conversions between supported integer types and from bool to a supported integer, with Rust truncation/sign-extension and no cast overflow panic. Literal expectations follow Rust cast expression boundaries.
- `if`/`else if`/`else`, `while`, `break`, `continue`, `return` and nested block statements.
- `for` over bounded integer ranges (`start..end`, `start..=end`) and owned Copy arrays. Bounds/array expressions evaluate once; array iteration uses a snapshot. Binding patterns are a name, `mut name`, or `_`; inclusive maximum endpoints do not overflow.
- Value-producing `if`/`else` and block expressions, function tails, and unit expressions. Branch results must agree; no-else `if` expressions require unit. Expression statements without a semicolon require unit.
- `print!`/`println!` with literal formats, sequential `{}` placeholders and `{{`/`}}` escaped braces; `println!()`; `assert!(boolean)`.

Replacing the implicit standard prelude with an external dependency named `std` is unsupported.

Local Cargo path dependencies are supported as separate library crate roots with isolated `crate`/`super` paths, extern bindings and nominal record identities. Library-only packages can be checked; supported binary graphs link together through the bootstrap backend. `ferrugo --check --crate-type=lib` checks a standalone library source tree.

The C11 backend generates native executables using libc. Operands and arguments evaluate left to right; `&&`/`||` short-circuit; integer overflow, invalid shift counts, division/remainder by zero and failed assertions exit with status 101. These runtime checks apply in both output profiles. Signed right shifts sign-extend, and left shifts discard shifted-out bits; boolean bitwise operations evaluate both operands. Generated code rejects a C target pointer width that differs from the Go compiler host. Generated programs do not need Go or Rust installed.

## Known limits

There is no ownership/borrow checker yet: the accepted subset has copyable scalar/record values and static string literals only. There are no references beyond literal-backed `&str`, heap allocation, general user-defined types, traits, generics, closures, patterns, slices, general iterators, enums, move-only structs, statics, const evaluation, async, unsafe, FFI, inline assembly, procedural or declarative macros, attributes, conditional compilation, standard library APIs, argument/input APIs, incremental compilation or Rust ABI support.

Array borrows/slices, reference/string/unit elements, general constant length expressions and lengths above 65536 remain unsupported. Open/stored ranges, general iterator traits, borrowed iteration, destructuring loop patterns and loop labels remain unsupported. Dynamic bounds checks panic with status 101; constant unconditional-panic linting is incomplete.

Move-only/plain structs, tuple/unit struct declarations, generic records, methods, struct update syntax, unit or reference fields, and trait-based record formatting/comparison remain unsupported. Records require the supported `Copy`/`Clone` derive pair; this does not implement general trait selection. Logical zero-sized arrays/records use an inaccessible C padding byte and zero-sized record fields are omitted. Rust layout/ABI introspection remains unsupported.

Public reexports, restricted visibility, grouped/glob/absolute imports, local module/import items, shared function/module/import/record names in one namespace, and shadowing primitive types with record declarations or record import aliases remain unsupported. Module nesting is bounded to 256 levels. Source-string compiler APIs support inline modules only; file-based APIs load external declarations.

Unit-valued bindings/parameters, `i128`/`u128`, floating-point and character types, non-ASCII identifiers, raw/byte strings and general format specifications are unsupported. Return/break/continue require semicolons. `&str` returns require exactly one `&str` parameter under Rust lifetime elision; explicit lifetime syntax is unsupported. Inference is local and intentionally more restrictive than Rust inference across later statements. General coercion/float/char/reference casts remain unsupported. Compile-time constant-overflow linting is not implemented; arithmetic overflow is checked at runtime. Edition declarations do not select complete edition-specific grammars.

Diagnostics have source locations but do not reproduce rustc error codes, wording or recovery. Successful C emission does not establish cross-target or ABI compatibility. Local validation is on Linux using GCC; CI also exercises macOS. The backend uses the host C compiler and has no cross-compilation support yet.

See [Cargo command boundaries](CARGO.md) and the [remaining roadmap](ROADMAP.md). Unsupported syntax, types, commands, manifests and targets fail explicitly; a test passing for this subset does not imply upstream conformance.
