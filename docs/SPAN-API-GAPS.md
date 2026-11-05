# Remaining span ownership contracts

The complete target remains the 13 pinned span implementation/test files in
the current goal. Native comparisons and coherent Go checks validate a large
implementation, but they do not close every public value contract.

Four implementation files remain partial at Rust revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`:

| Original file | Lines | Concrete remaining contract |
| --- | ---: | --- |
| `compiler/rustc_span/src/lib.rs` |2930| `SourceFile::new` consumes a caller-owned `String`; public `src` stores `Arc<String>`. Owned filename/path payloads and public Vec fields also need real owning storage. |
| `compiler/rustc_span/src/hygiene.rs` |1766| `debug_hygiene_data` and `ExpnKind::descr` return owned `String`. Returned collection storage also needs a full capacity/ownership audit. |
| `compiler/rustc_span/src/source_map.rs` |1327| File-loader results, source-file inputs and snippets expose owned `String`; owned `PathBuf`/`OsString` and `Cow::Owned` need owning storage. |
| `compiler/rustc_span/src/symbol.rs` |3213| `Symbol::to_ident_string` returns owned, mutable `String`; used_keywords returns an owning Vec<Symbol>. |

For example, a lawful Rust caller can pass a `String` with spare capacity to
`SourceFile::new`, then inspect that capacity through the public `src` field.
The existing Go constructor accepts `string`; `SharedString` deliberately
stores `Shared[string]` and exposes immutable text and reference counts.
That interface cannot express the source input's mutable buffer, capacity or
unique-owner recovery. Matching bytes and Arc counts does not close this gap.

Likewise, `RustPath` is explicitly an immutable Path/OsStr view and `CowString`
retains text plus a borrowed/owned tag. These are useful scoped dependencies,
but an owned source value needs its actual buffer, length, capacity, move,
clone and mutation behavior. Source APIs that return owned vectors require
the same attention to the selected Rust growth and collection strategy.

The closure work is a shared owning string/byte-buffer implementation with
the exact selected SDK's allocation/growth behavior, borrowed mutable cells,
and consuming ownership operations. It must then be composed into the four
files' primary inputs, public fields and results, including path/Cow owners.
Immutable text entry points can remain documented conveniences. Native
comparisons must observe capacity, lawful mutation and buffer/owner identity,
in addition to the existing formatter, callback, panic and algorithm traces.

The source mapping and existing comparisons remain retained evidence. They
grant no full-file credit to these four implementation rows. Complete source
analysis, caching, edit distance, fatal-error operations and the five original
child test files are reviewed separately. Standard-library and container
helpers receive no whole dependency-file credit from either group.


The new scoped String/Path engines are validated prerequisites; the four
canonical implementations still need their complete owning primaries. Direct
Lib vector crossings include SourceFileLines::Lines (line 1942), the public
multibyte/normalization vectors (2006/2008), and FileLines.lines (2835).
Private carried byte/normalization vectors also need exact collection behavior.
DistinctSources and the error aggregates carry owners transitively through
FileName and SourceFile; they do not introduce a separate direct Vec field.

SourceMap composition must preserve the same owners and cleanup. Duplicate
source-file returns must drop unused consumed filename/source inputs. Imported
ExternalSrc locks need the ExternalSource destructor, and Result-to-Option or
boolean helpers must drop discarded owning error payloads. Real filename
construction must retain actual PathBuf fields so a later into_local_path moves
the selected field's buffer. Generic Shared/Arc contracts remain bounded by the
actual element type; matching strong counts alone does not prove weak or
unique-owner operations.
