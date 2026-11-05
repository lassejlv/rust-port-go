# Consuming Into receiver

The public pinned `FilePathMapping::to_real_filename` accepts
`impl Into<Cow<Path>>` by value. A legal implementation may move, retain or
forget its whole Self. The existing borrowed-state conversion helpers remain
conveniences; `ToRealFilenameConsuming` receives `*rustvec.Owned[E]` through
`IntoCowPathOperation[E]` and uses the existing `ConsumeValue` operation.

Remaining Self destruction and field cleanup finish before mapping/tracing.
Moving or taking Self invalidates that cleanup guard. The returned Cow is
deref-transparent here: the unchanged mapping body borrows its path and clones
the actual filename fields. No mapping algorithm or path buffer is copied by
the new adapter. Borrowed results and callback borrows obey Rust lifetimes;
the operation descriptor contains borrowed code metadata, with complete owned
state and destruction in the receiver witness.

The native client originated in the independent reviewer’s safe full-crate
probe. Its tracked form links the entire unchanged original rustc_span through
the verified loader. It compares 16 cases: normal Drop, whole-Self retention
and later explicit Drop, forget, forget then panic, in normal/inherited-caught/
inherited-escaping/processing contexts. Actual owned path state, modified token,
user and field destruction, real Arc/drop counters, hooks and resulting filename
Debug are retained. Two native children terminate with actual SIGABRT; Go uses
the documented exit134 representation. Native terminal abort stderr is not
compared. Source locations come from the explicit selected producer profile;
the stable relative client filename is selected by a compiler prefix remap.

The complete source remains `compiler/rustc_span/src/source_map.rs` at
`2d20b42b883dcfc014f3c2c56081eeb6091c2179`. This adds no original file or standard
runtime credit. All 512 previous borrowed-state Into cases and old files remain
unchanged. The new manifest binds the exact new Go implementation/test, existing
mapping implementation, client, generator and native data. Matching nightly
2026-11-04 is verified on every generator invocation; cached linkage alone is
never trusted. Children and compilation have finite timeouts.

```sh
python3 tools/port-rust-source-map/consuming/generate.py --check
go test ./port/rust/compiler/rustc_span/src -run '^TestSourceMapConsumingIntoNativeOracle$'
```
