# Complete union-find file port

Source pin: `rust-lang/rust` commit
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`.

| Original source | Go port | Scope |
| --- | --- | --- |
| `compiler/rustc_data_structures/src/union_find.rs` | `port/rust/compiler/rustc_data_structures/src/union_find.go` | All types, `new`, `find`, `unify`, `snapshot`, rank cases, path-compression steps, derived debug representation, and debug assertions |
| `compiler/rustc_data_structures/src/union_find/tests.rs` | `port/rust/compiler/rustc_data_structures/src/union_find/tests_test.go` | Both original test functions, with additional typed-index and failure-contract tests |

The pinned files contain 96 lines/3,497 bytes and 32 lines/644 bytes,
respectively. `manifest.json` records their Git blob IDs and SHA-256 hashes.
The Go implementation uses only the standard library.

`UnionFind[Key]` holds the same fixed parent/rank table. `Find` updates each
visited key to its original grandparent and advances to its original parent.
`Unify` performs its two finds in source order, retains the first root on equal
ranks, increments that rank, and attaches the second root. `Snapshot` calls
`Find` for every key in increasing index order and returns independent storage.
This is a complete port of this compiler utility, not a complete Rust compiler.

## Typed indices and container dependency

The upstream file depends on `rustc_index::Idx` and `IndexVec`. Their complete
`idx.rs`, `vec.rs`, `slice.rs`, and crate-root source were inspected before
porting. The Go file uses scoped adapters for the operations this utility needs:

- `UnionFindIndex[Key]` supplies `FromIndex` (`Idx::new`), `ToIndex`
  (`Idx::index`), and independent optional `Equal`/`NotEqual` callbacks for
  `PartialEq::eq` and `PartialEq::ne`. Nil `Equal` uses Go's
  `==` for comparable keys. Non-comparable keys require `Equal`; nil `NotEqual`
  calls `!Equal`, matching Rust's default `PartialEq::ne`. Keys and callbacks must obey the upstream
  typed-index and equality laws. `Key any` permits lawful slice-backed metadata
  keys corresponding to Rust keys containing static slice references. Callbacks
  may trace their calls or panic.
- `UnionFindUintIndex` and `UnionFindUint32Index` delegate `New` and `Index` to
  the completed `rustc_index/src/idx.go` component's `UsizeIdx` and `U32Idx`.
  Thus the two compiler utilities share the proven upstream `usize` and checked
  `u32` conversions. `uint` models the current Go host's `usize`. Union-find
  does not call their plus/increment methods; custom public index callbacks and
  comparison overrides remain unchanged.
- `NewUnionFind` validates `FromIndex(numKeys)` before allocation, then calls it
  for `0..numKeys`. This includes an empty forest's `FromIndex(0)` call.
- `Snapshot` validates `FromIndex(len)` before any per-key conversion, then
  constructs its keys in increasing order, matching `IndexSlice::indices`.
- Table access uses `ToIndex` followed by slice bounds checking. Lookup and
  callback failures panic; panic message spelling follows the host language.
- `UnionFindSnapshot.Raw` corresponds to the returned owned `IndexVec::raw`.
  `At` and `Set` retain typed indexing. Changing it never changes the forest.

`Idx` is independently ported and validated as its own source file; this
integration adds no completion credit. These scoped storage/snapshot adapters
do not claim to port the general `IndexVec` or `IndexSlice` files.

## Debug and release configuration

`NewUnionFind` enables source debug assertions by default. Use
`NewUnionFindWithConfig(n, index, UnionFindConfig{DebugAssertions: false})` for
the source release profile. The debug profile preserves the extra typed-index
lookups and equality call in both assertions, including callback panic timing.
Rank overflow cannot occur in a constructible lawful forest: a rank-r root
requires at least 2^r keys, while the rank field has 32 bits.

`Debug(formatKey)` preserves the derived representation and delegates each key's
Rust-style debug formatting to the supplied formatter. Go pointer layout,
borrow checking, and Rust's automatic `Send`/`Sync` trait system are not
reproduced. As with Rust's exclusive `&mut self`, callers must exclude concurrent
mutation. A Go zero value lacks its required index adapter; use a constructor.

## Original-source oracle dependencies

`oracle.rs` compiles the complete original `union_find.rs` and both original
tests without editing their bytes. It also compiles the complete original
`rustc_index/src/idx.rs`. A development-only Rust `IndexVec` shim supplies:

- `from_fn_n`, including the upfront `I::new(n)` validation and per-key order;
- `indices`, including upfront `I::new(len)` and increasing index construction;
- typed `Index`/`IndexMut`, each calling `index()` once before `Vec` access;
- `FromIterator`, collecting into an independent `Vec`;
- debug formatting through the underlying vector, matching `IndexSlice`.

These operations mirror the inspected pinned container implementations. The
oracle does not replace the union-find algorithm with a reference algorithm.
The shim, index adapters, and oracle do not count as other completed source ports.

Generate fixtures with `go run ./tools/port-rust-union-find`; verify them with
the same command and `-check`. Flags `-root`, `-rustc`, and `-rust-source` select
the workspace and pinned local tools/cache. Ordinary Go tests use the committed
compressed fixture and require neither Rust nor network access.
