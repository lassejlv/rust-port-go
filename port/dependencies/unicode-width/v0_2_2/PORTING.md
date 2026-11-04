# unicode-width 0.2.2: scalar width only

This is an exact functional adapter for the builtin `char` implementation of
`UnicodeWidthChar::width`, used by `rustc_span::char_width`. It does not port a
complete upstream source file or the complete dependency. `UnicodeWidthStr`,
CJK width methods, custom trait implementations, original tests and benchmarks
remain unported. In particular, rustc's `str_width` sums its own `char_width`;
it does not use this dependency's sequence/ligature width algorithm.

The exact official archive has SHA-256
`b4ac048d71ede7ee76d585517add45da530660ef4390e49b098733c6e897f254`,
VCS revision `9d98411769fe13c7c18cab0b3fbbab29ba8350ea`, and Unicode version
17.0.0. `source-manifest.json` inventories every one of its 15 members, including
the unported source, data, tests and benchmark. Both licenses and COPYRIGHT are
retained unchanged. No source-file completion credit is implied.

`CharWidth(rune) (uint, bool)` represents `Option<usize>`; false is None. Inputs
must be Unicode scalars, matching Rust `char`. `UnicodeVersion()` returns the
immutable version tuple. Production uses only the Go standard library and the
generated 522 exceptional ranges; no host Unicode approximation or Rust runtime
is used. The ordinary Some(1) result is implicit between exceptional ranges.

The development checker compiles the unchanged complete crate source in its
default CJK-enabled profile, then queries its actual `UnicodeWidthChar::width`
implementation for all 1,112,064 valid scalars. Its complete response is retained
as a checked gzip fixture and replayed by offline Go tests. Surrogates and values
outside the scalar domain are rejected separately. No test of the string/CJK
APIs, architecture-specific runtime or whole-crate parity is claimed.

Run `python3 tools/port-unicode-width-scalar/generate.py --check` from the project
root to verify the archive, regenerate the original reference in an ignored
cache and compare evidence without modifying it. Select an archive with
`--archive`, and a compiler with `--rustc` or `FERRUGO_RUSTC`; invalid selections
fail. Without an archive override the checker uses the project's Cargo cache.
Without a compiler override it uses the project toolchain if installed, then
PATH. Go 1.23 package tests and live regeneration passed on native Linux amd64.
