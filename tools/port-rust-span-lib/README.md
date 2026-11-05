# Whole-span lib.rs development comparisons

Prepare the genuine original reference using
[port-rust-span-reference](../port-rust-span-reference/README.md). These tools
require Python 3.11+, the pinned official 1.101 dated nightly, and the complete
verified native reference. Go production code has no Rust fallback.

Each `generate_*.py` creates its named fixture/manifest; `--check` recomputes
the actual Rust observations and requires exact retained evidence. Existing
compressed bytes are preserved only after bounded full decompression, CRC and
raw-byte equality. A corrupted, expanded or different fixture is rebuilt in
generation mode and fails check mode. Symlink fixture paths are rejected.

```sh
python3 tools/port-rust-span-lib/generate_source.py --check
python3 tools/port-rust-span-lib/generate_format.py --check
python3 tools/port-rust-span-lib/generate_display.py --check
python3 tools/port-rust-span-lib/generate_boxes.py --check
python3 tools/port-rust-span-lib/generate_pos.py --check
python3 tools/port-rust-span-lib/generate_panic.py --check
python3 tools/port-rust-span-lib/generate_generic.py --check
python3 tools/port-rust-span-lib/generate_generic_ambient.py --check
python3 tools/port-rust-span-lib/generate_session_inputs.py --check
python3 tools/port-rust-span-lib/generate_flags_iter.py --check
python3 tools/port-rust-span-lib/generate_flags_format.py --check
python3 tools/port-rust-span-lib/generate_memory.py --check
python3 tools/port-rust-span-lib/generate_file.py --check
python3 tools/port-rust-span-lib/generate_map.py --check
python3 tools/port-rust-span-lib/generate_opaque_callbacks.py --check
python3 tools/port-rust-span-lib/generate_owned_hash.py --check
python3 tools/port-rust-span-lib/generate_trait_defaults.py --check
python3 tools/port-rust-span-lib/generate_trait_failures.py --check
python3 tools/port-rust-span-lib/generate_tracing_profile.py --check
python3 tools/port-rust-span-lib/generate_byte_slice_strategy.py --check
python3 tools/port-rust-span-lib/generate_macro_backtrace_context.py --check
python3 tools/port-rust-span-lib/generate_remaining_context.py --check
python3 -m unittest discover -s tools/port-rust-span-lib -p 'test_*.py'
```

`FERRUGO_RUSTC`, `FERRUGO_RUSTDOC`, `FERRUGO_CARGO`, `FERRUGO_RUST_SOURCE` and
`CARGO_HOME` use the shared preparer's exact tool/PATH/source selection
contract. Invalid or explicitly empty selections fail; they are never silently
ignored. Default execution is offline. `--profile PATH` chooses independent
debug/overflow flags from a pinned profile; the profile's compiler/artifact
commands are never executed. A fresh full preparation returns the genuine
verified linkage. The file driver uses all four profiles from
`--reference-root` (default canonical reference directory).

The trait/default, trait-failure, tracing-profile, byte-slice strategy and
separately owned macro-backtrace context drivers also use `--reference-root`.
The tracing profile retains actual selected-build metadata verbatim, including
its source-file spelling. The Go profile installs immutable callsite cells
before filtering or registration. No observer strips source-path prefixes.

The memory/file/map/opaque-callback serializer subset is independently usable:
its Go package imports no Go Span implementation. Those four manifests bind
the original reference, their clients/generators, the reference selectors and
their fixtures. They do not bind this directory's broad Lib validation report
or unfinished sibling Go ports. This is a scoped codec dependency port and
does not grant rustc_serialize source-file completion credit.

The actual original FileEncoder `/dev/full` error observations are a Linux
reference subset; default Go replay skips only those device-specific rows on
other hosts. Numeric/opaque/generic comparisons retain the 64-bit original
reference domain. No allocator ABI, SIMD performance or transparent Go-local
panic/defers claim is made.

Run Go replay with a selected Go 1.23 or newer executable:

```sh
go test ./port/rust/compiler/rustc_span/src ./port/rust/compiler/rustc_serialize/src \
  -run '^(TestLib|TestOriginalMemory|TestOriginalFileEncoder|TestOriginalDeterministicMap|TestOriginalOpaque)'
```

Full-file status and the explicit remaining proof boundaries are in
[PORT.md](PORT.md). These are development comparisons, not the upstream Rust
compiler's complete regression suite or a production-compiler claim.
