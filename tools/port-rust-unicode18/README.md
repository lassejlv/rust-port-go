# Scoped Unicode 18 formatting

This adapter supplies the selected compiler SDK's `char` and `str` Debug
formatting and scalar escaping. The separately supported Unicode 17 comparison
profile remains available through its existing entrypoints. No standard-library
source-file completion credit is granted.

The authored native client calls the actual official nightly-2026-11-04 SDK.
Every valid scalar contributes its exact character/string writer calls and
public `char::escape_debug` result. All 1,112,064 scalars match 272 retained
SHA-256 blocks; the Go replay requires neither Rust nor network access.

The generator verifies the official SDK source archive and the exact original
character/formatter modules. Its manifest binds the helper, independent replay,
Unicode data and native observer. Run:

```sh
python3 tools/port-rust-str/prepare.py --fetch
python3 tools/port-rust-unicode18/generate.py --check
go test ./port/support/rustfmt -run '^TestUnicode18OriginalScalarFormatting$'
```

Source preconditions require valid Rust strings/scalars. This is a used-operation
dependency, independent of the work-in-progress compiler Symbol implementation.
