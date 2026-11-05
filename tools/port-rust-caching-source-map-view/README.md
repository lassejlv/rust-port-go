# Caching source-map view

The Go implementation maps the complete
`compiler/rustc_span/src/caching_source_map_view.rs` at Rust commit
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` to
`port/rust/compiler/rustc_span/src/caching_source_map_view.go`. Integration and
parent inventory acceptance remain pending. The surrounding source-map/span
implementation receives no completion credit from this bounded proof.

The native drivers call the actual public reexport from the complete unchanged
`rustc_span` crate. All four independent debug-assertion/overflow configurations
use official nightly 2026-11-04 (Rust1.101, Unicode18). The shared reference
verifies its compiler source graph, exact registry dependencies and original
unit tests. No algorithm or private metadata representation is substituted.

| Evidence | Original-source coverage |
| --- | ---: |
| Normal files, repeated cache hits/misses and map growth |18,672 requests /141,656 steps |
| Imported/lazy metadata, empty/unsorted lines, full-u32 lengths |2,304 requests /368,640 steps |
| Normal, inherited caught/escaping, actual panic-hook contexts |224 requests /56 isolated aborts |

Every result retains line/byte columns, file identity through start position,
all relevant Arc owner counts before/during/after calls, and lazy line-state
freeze/conversion changes. The normal corpus covers BOM/CRLF normalization,
UTF-8 continuation bytes, final newlines, EOF, empty files, reversed spans and
cross-file rejection. Point results own a cloned file; span results borrow
the view's actual file without cloning it.

Imported metadata uses actual SourceFile constructors and codecs. Invalid lazy
diffs are public decoder byte inputs: the helper replaces the encoded line
section and calls the original SourceFile decoder. It never constructs private
Diffs fields. Width0/3 and overflowing differences preserve failure phase,
checked/wrapping behavior, partial cache updates, freeze state and owner counts.

The context driver supplies complete external operation/local-cleanup witnesses.
Rust's actual outer destructor and panic hook exercise internal library failures.
Go uses the explicitly supplied current `rustfmt.Context`; callback-local raw
Go defer/recover timing is not a Rust runtime implementation. Aborted cases
retain only observed event prefixes. Exit134 models the unrecoverable class,
without a SIGABRT/core-file ABI claim. Core dumps are disabled and subprocesses
are bounded.

`CloseWithContext` consumes the cached owner. Copying a Go view descriptor is
not an ownership clone. The map, borrowed span file and returned point owners
must follow their source borrow/drop lifetimes. The Go descriptor and GC-backed
storage do not claim the original Rust ABI or custom allocator behavior.

Prepare the shared reference first, then reproduce:

```sh
python3 tools/port-rust-caching-source-map-view/generate.py --check
python3 tools/port-rust-caching-source-map-view/generate.py --metadata --check
python3 tools/port-rust-caching-source-map-view/generate_context.py --check
go test -race ./port/rust/compiler/rustc_span/src -run '^TestOriginalCachingSourceMap'
```

`--prepared` remains an accepted compatibility flag. Every run revalidates the
complete original graph and receives fresh verified compiler/linkage arguments;
cached profile JSON selects the arithmetic flags only.
New generators preserve existing compressed bytes when their raw records match.
The fixtures bind native drivers, the complete local production import closure
and its Go tests, the helpers and compiler/source profiles. Replays verify all
selected input hashes, fixture SHA-256 and retained record counts.

Actual minimum-Go1.23 native replay, race detection (13.456s for the complete
bounded corpus) and span package vet pass. Wider span/source-map work remains
under integration; the production Rust compiler goal is unfinished.
