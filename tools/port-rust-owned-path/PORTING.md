# Owning paths used by rustc_span

This helper implements the concrete owning `PathBuf`, `OsString`, `Cow<Path>`
and `Cow<str>` operations required to close the path storage boundary of
`compiler/rustc_span/src/lib.rs` and `source_map.rs` at Rust revision
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`. The four parent Span files remain
partial until their primary interfaces and public fields use owning storage.
This report grants no complete standard-library file, allocator ABI or parent
Span file credit.

The selected standard-library dependency is the official SDK1.101 nightly
component for 2026-11-04, commit
`708c94ef093ecc42fb8851e17d8383d999727abc`. `prepare.py` verifies the official
channel manifest, source archive and each selected source member, then requires
the paired compiler's exact `-vV` identity. `SOURCES.json` retains the archive,
manifest, compiler and ten scoped original module identities. Required license
bytes are retained and verified in `port/support/ruststr/{COPYRIGHT,LICENSE-MIT,
LICENSE-APACHE}`.

The production implementation is decomposed into these new files:

| Go file | Scoped source operations |
| --- | --- |
| `source_map_owned_path.go` | `OsString` actual mutable byte storage, `with_capacity`, `from String`, `from_wide`, clone/clone_from, reserve/exact/shrink, clear, push, ASCII mutation, encoded-buffer transfer and `into_string` Result |
| `source_map_owned_path_buf.go` | `PathBuf` owner/cell, zero-copy OsString/String conversion, clone/clone_from, clear/reserve, push/pop, file-name and extension mutation; `Path::to_path_buf` and `Path::join` owned results |
| `source_map_owned_path_cow.go` | All borrowed/owned clone, clone_from, to_mut and into_owned branches for `Cow<Path>`/`Cow<str>`; `OsStr::to_string_lossy` with actual owned String |
| `port/support/ruststr/owned_string_buffer_bridge.go` | Consuming transfer of the real String/Vec byte descriptor, valid-UTF8 reconstruction and shared empty-u8 identity |

Path files reside in `port/rust/compiler/rustc_span/src`. The new String bridge
does not change the pre-existing frozen owning String implementation. The
immutable `RustPath`, component parser, Windows prefixes and formatter/hash
protocols remain borrowed-view dependencies. Legacy immutable `CowString` and
`RustPath` conveniences are retained; they do not become owning containers.

Each owner contains a mutable cell whose slice stores actual bytes, length and
capacity. Handle copies alias the same owner; they are not clones. Move/drop
consumes that owner for every handle alias. Cloning allocates independently with
capacity equal to source length. A borrowed `RustPath` uses the real buffer
without copying. Rust's lifetime rule applies: a borrowed view or mutable cell
must not be retained across a conflicting mutation, move or drop. No runtime
metadata, panic hook, caller location or formatting context is captured in an
owned value.

On Windows, `Wtf8Buf` derives `Clone` and inherits the default `clone_from`,
which replaces the destination with a length-sized clone. Unix's OsString
clone_from reuses capacity through Vec's specialization. `Cow<Path>`'s
owned-to-owned clone_from calls `Path::clone_into` instead; its OS slice
specialization reuses destination capacity on both hosts. The native corpus
distinguishes these operations, including empty sources and spare capacity.

The String bridge consumes an owning slice descriptor. Its recipient must
relinquish all previous mutable aliases. `into_string` validates before taking
that descriptor; success transfers it to a String, and failure returns an
OsString preserving the original buffer, length and capacity. `BufferIdentity`
is a within-process witness, not a Rust ABI address. Unallocated empty u8
buffers share the existing String helper's dangling identity.

Mutation follows source order. Path push may clear or truncate first, and
appends a separator and right-hand path separately. Windows verbatim push
creates a new empty OsString and pushes each normalized component separately,
then replaces the old allocation. Extension replacement validates first,
truncates to the actual borrowed stem end, then reserves **exactly** extension
length plus one, pushes the dot and appends extension bytes. Adding an empty
extension leaves trailing separators unchanged. A path join clones the left
path's length before calling the actual push sequence. There is no final-text
capacity approximation or use of `filepath.Clean`.

Unix preserves arbitrary OS bytes. Windows uses WTF-8, preserving unpaired
surrogates and combining newly adjacent leading/trailing surrogate halves for
OsString push. UTF-16 construction starts with capacity equal to input unit
count, then pushes decoded code points individually. Unix lossy conversion
follows UTF8Chunks and the individual source String pushes; Windows lossiness
replaces each three-byte surrogate with one three-byte replacement character.
The borrowed empty Unix lossy result is the static empty str; Windows retains
the empty source slice borrow.

`native.rs` calls the actual public std operations. `generate.py` compiles this
std-only client sequentially for Linux and the pinned Windows target and retains
deterministic scripts and complete observations in `native.jsonl.gz`. Windows
Rust PE executes under the existing authenticated Wine10 runtime on Linux; it
does not establish Windows kernel validation. The native manifest records the
exact target component, Wine packages, compiler, wrapper and source-selection
hashes. The imported prerequisite profile's historical wording mentions Rust
and Go PE binaries; this producer invokes only the Rust PE. Go package replay
of the portable Windows grammar is a separate check.

The scripts observe byte contents, length, capacity, move/conversion/clone
identity relations, lawful mutation, Cow alternatives and source panic payload
types/text with post-failure state. Pointer equality across allocator growth or
shrink is intentionally not asserted: native realloc may move or keep a buffer.
This helper does not expose allocator callback/ABI behavior. It does preserve
the source's observable buffer ownership and capacity through operations that
guarantee a transfer or reuse, including spare-capacity String conversions.
Checked capacity overflows panic through the current operation Context;
allocation failure terminates with the existing owning String helper's abort
profile. No forced out-of-memory experiment is claimed.

`source_map_owned_path_oracle_test.go` replays the actual std transcripts against
these production owners. `source_map_owned_path_ownership_test.go` additionally
checks consumed aliases/cells, unique mutation and invalid-Unicode owner
recovery. `test_integrity.py` verifies the retained checksums, every generated
request, trace cardinality and exact official SDK source/compiler provenance.

Validation commands and final file identities are retained in `validation.json`
and `GO-SOURCES.json` after the scoped package gate. Native clients are std-only
and do not expand Cargo dependencies. No upstream compiler is called by the Go
production implementation.
