# Pinned core Unicode source port

This is the full port of `library/core/src/unicode/unicode_data.rs` from Rust revision `2d20b42b883dcfc014f3c2c56081eeb6091c2179`, Git blob `84414a72df88c4718d040cb93777d7a4b265e8d4`, to `unicode_data.go`. The source has 1,786 lines and selects Unicode 18.0.0.

The Go file retains all 33 compressed static tables and translates their bitset lookup, short-offset-run headers, skip search, conversion range/LUT lookup and casefold algorithms. It exposes 13 named `*Lookup` predicates, four case conversions (`ToLower`, `ToUpper`, `ToTitle`, `ToCasefold`) returning all three rune slots, and `UnicodeVersion`. It uses no Go Unicode tables. Predicate lookups are the source's non-ASCII tables; core callers must handle ASCII separately. Invalid runes return false for predicates and panic for conversions, since Rust's `char` excludes them.

`tools/port-core-unicode` deterministically translates the source tables and combines them with the reviewed Go algorithm template. It verifies the pinned source blob and records source, table, template, output and reference hashes in `unicode_data_manifest.json`. The same tool compiles the untouched source with a standalone Rust shim. The shim reexports standard range/ordering types and implements the optimization-only `intrinsics::assume` as an assertion; it does not replace the Unicode algorithms or data.

Every one of the 1,112,064 Unicode scalar values matches the untouched Rust implementation for all 13 predicates and all four exact three-slot conversion arrays. The retained reference is `testdata/core_unicode_oracle.bin.gz`. Tests also check table/header/LUT invariants, invalid runes, ASCII contracts, source provenance, checksums and licensing. The manifest identifies the exact oracle encoding and hashes. Regeneration, package/generator vet and whitespace checks pass.

```sh
go test ./port/rust/library/core/src/unicode
FERRUGO_RUSTC=rustc go run ./tools/port-core-unicode -check
FERRUGO_REGENERATE_CORE_UNICODE_ORACLE=1 FERRUGO_RUSTC=rustc \
  go test ./port/rust/library/core/src/unicode
```

The live commands require the pinned Git reference cache prepared by `tools/port-inventory/inventory.py prepare`. `FERRUGO_GO` and `FERRUGO_RUSTC` select tool executables; ordinary tests require only Go and committed references. Production execution never calls the oracle.

The pinned REUSE annotation overrides this file to Copyright 1991–2024 Unicode, Inc., under Unicode-3.0. The exact license is retained as `LICENSE-UNICODE`, with its checksum and annotation in the manifest. This standard-library Unicode 18 profile remains separate from rustc_lexer's pinned Unicode 17 dependencies and Cargo's Rust 1.99.1 standard-library profile.

This file's completion does not complete core, the standard library or production compiler compatibility.
