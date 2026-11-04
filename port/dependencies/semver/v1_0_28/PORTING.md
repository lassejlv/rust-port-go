# semver 1.0.28

This is a source-aligned Go port of the official `semver` 1.0.28 crate,
used by Cargo's semantic-version requirement extensions. The source/API port,
including the primary owned `IntoIterator`/Vec collector, passed independent
review and native validation. The
`iter.Seq` entry point remains the trivial-ownership convenience. Production code uses
only the Go standard library and never invokes Rust or an external version
parser. The official archive has SHA-256
`8a7852d02fc848982e0c167ef163aaff9cd91dc640ba85e263cb1ce46fae51cd`.
Its `.cargo_vcs_info.json` identifies revision
`7625c7aa3f0e8ba21e099d1765bcebcb72aa8816`.

`source-manifest.json` inventories every archive member, original byte/line
count and SHA-256, and its Go or retained-support-file mapping. All 25 archive
members are accounted for: 8 implementation files (2,117 Rust lines), 6 original
test/support files (893 lines), 1 benchmark file (21 lines), and 10 license,
manifest, documentation, and repository-metadata files. Original upstream
licenses are retained alongside this document. The dependency inventory does
not add these files to the Rust/Cargo project-source completion count.

The public value types are `Version`, `VersionReq`, `Comparator`, `Op`,
`Prerelease`, `BuildMetadata`, and `Error`. Source constructors and `FromStr`
map to `NewVersion`, `ParseVersion`, `ParseVersionReq`, `ParseComparator`,
`NewPrerelease`, and `NewBuildMetadata`. `Some`/`Option` distinguish absent
minor/patch fields from zero. `Star`, empty identifier constructors, and Go
zero values retain the source constants/defaults. `FromOwnedIterator` consumes the primary typed owned input/iterator witnesses in
neutral `rustvec`; `FromIterator` accepts Go 1.23 `iter.Seq[Comparator]` as a
trivial-ownership convenience. `FromComparators` also accepts a Go slice.
The collector preserves input/iterator cells, Drop and unwind/processing context,
next/hint/for_each/fold dispatch, trusted collection, and Vec::IntoIter capacity
and allocation reuse. A `VecFromIterator` static witness represents each
iterator type's extra monomorphized standard-library specialization; it is not
a Rust user override. Nil selects the proven generic/trusted source strategy
only when that is the actual strategy of the supplied type adapter. The native
map/in-place witness proves buffer reuse, mapping, destructor, and panic order.
The stated native Comparator layout is verified from the original Rust type;
resulting Go slice capacities match that profile. Broader stdVec implementation
and ABI/allocation-failure equivalence are separate obligations.
Clone, equality, ordering, partial ordering, precedence comparison, and matching
follow the actual source; requirement/comparator/Op ordering is not invented.
All six Clone types expose CloneFrom as the provided whole-value clone then
replacement operation. VersionReq clones allocate comparator storage with
capacity exactly equal to its length; CloneFrom replaces destination storage
even when its former capacity would suffice. The native clone corpus verifies
spare capacities, independent mutation, and all six default adapters.
Publicly constructible comparator combinations, including `Patch=Some` with
`Minor=None`, retain source behavior.

Rust string references and Deref map to immutable Go strings, `AsStr`, `Len`,
and `IsEmpty`. Parser inputs require valid UTF-8; invalid Go strings panic
instead of silently substituting a replacement character for a Rust `&str`.
Parsing remains ASCII byte based, including source error order, leading-zero
rules, exact `u64` overflow, ASCII-space handling, wildcard diagnostics, and the
32-comparator cap. Numeric prerelease/build segments may exceed 64-bit values:
ordering compares their text and lengths exactly as upstream.

`identifier.rs` is private storage machinery. Its tagged inline/heap pointer,
varint length header, endian conversion, allocation/deallocation, private
pointer equality shortcut, Clone, and Drop are represented by immutable Go
strings and garbage collection. This preserves public values, equality,
ordering, hashes, and independently mutable comparator-vector clones; it does
not reproduce Rust ABI layout, allocator calls, pointer identity, allocation
failure, or allocation timing. This is a storage representation adaptation,
not a parser or accepted-value restriction.

`Hasher` supplies Rust's required byte-write method. Primitive write overrides,
`WriteString`, and `WriteLengthPrefix` preserve typed callback dispatch; defaults
retain native-endian integer bytes, string terminators, and vector length
prefixes. The oracle checks raw-byte, primitive-override, and prefix-free-hook
profiles against the original Rust implementations. Concrete `DefaultHasher`
is not reimplemented. Original upstream tests of its repeatability remain as
Rust witnesses and equivalent caller-owned Go hashing assertions.

`WriteDisplay` streams source writes to a fallible `io.Writer`. Only Version
Display pads, with its original default left alignment and fill behavior;
other Display implementations ignore outer width and precision. `WriteDebugFormatter`
is the primary Debug API: its `*DebugFormatter` retains caller options and sink
identity. Compact structures/options/lists share that state; pretty fields
receive copied options and the nested indentation adapter. Numeric padding
changes fill/alignment before writing and restores them only on success, so a
writer error leaves the same observable compact formatter state as Rust.
`WriteDebug(writer, options)` and string Debug helpers are convenience wrappers.
Sinks may independently override `WriteChar(rune) error`; the default encodes a
UTF-8 string write. Character/string dispatch, empty writes, indentation,
accepted prefixes, failing calls, and post-error state are tested against native
Rust. Arbitrary fill, alignment, width, precision, plus/minus, zero padding,
pretty output, and lower/upper hex Debug options have explicit representations.
Go `fmt.Formatter` is a convenience surface: string/value verbs use Display;
`%#v` uses compact Rust Debug. Full Rust options use the explicit APIs.

The Unicode compatibility table is regenerated from every valid character's
native Debug output on Rust 1.99.1 (Unicode 17.0.0); it does not depend on the
host Go version's Unicode tables. Unicode data terms are in `UNICODE-LICENSE`.

Build tags represent source features:

- Default: `std` enabled, `serde` disabled.
- `semver_no_std`: disables the `std::error::Error` source-chain adapter.
- `semver_serde`: enables all six original serializer/deserializer implementations.
- Both tags: source `no_std + serde` availability.

Go still uses its ordinary runtime; the `no_std` tag preserves source feature
availability rather than claiming a freestanding Go runtime. `Error()` maps
Display in every profile; `Source()` exists only with source `std`. Serde's
serializer/deserializer associated result and error values remain caller owned.
`SerializerAdapter` preserves default `collect_str` dispatch to `serialize_str`
and an independent override. The visitor adapter covers every inherited visitor
input, exact expected text (including VersionReq's original `semver version`
wording), default forwards, independent `invalid_type`/`custom` callbacks, and
alternate input-associated Error factories. Default invalid-type messages keep
Unexpected and Expected as lazy delegated Display values, preserving literal,
character, float-part, and expecting writes and their exact failure order.
The scoped floating-point compatibility preserves Rust shortest-decimal midpoint
rounding and native runs of at most 64 zeros; no general float-formatting crate
or arbitrary-precision version parser is substituted. Rejected Some/newtype/access data
is not traversed. Rust lifetimes map to immutable Go values and error/visitor
traits map to explicit Go protocols, not a port of the entire serde framework.

JSON/Text methods are convenience codecs around those same visitors. Canonical
accepted semver strings serialize to the exact ASCII JSON string, including
unescaped `<`/`>`; Go's outer `encoding/json` may apply HTML escaping unless its
encoder disables that host option. JSON rejects unpaired UTF-16 surrogates,
retains the receiver on failure, and preserves immediate container/type rejection
and visitor-before-trailing-input error order. Native codec probes compare
valid bytes, source visitor errors, and data-versus-syntax error classes. Host
JSON syntax diagnostic wording/locations are not a claim to port serde_json;
`SerdeError.Syntax` identifies that class. Text validates UTF-8 before invoking
the source string visitor.

The native evidence and commands are documented in
`tools/port-semver/README.md`. All original test functions, assertions, literals,
source order, optional Node matcher profile, and three benchmark workloads are
preserved; their independent mapping and unchanged Rust execution are recorded
in `tools/port-semver/original-tests`.
