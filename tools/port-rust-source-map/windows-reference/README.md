# Workspace Windows-target comparison

The Rust reference and Go implementation are Windows64 PE binaries executed
under the same authenticated Wine10 runtime on Linux. This is explicitly an
emulated comparison. It does not establish native Windows kernel/runtime
behavior; a genuine windows-latest test job remains authoritative.

`prepare.py` verifies the retained Debian13.7 clear-signed InRelease with the
system Debian archive keyring, validates the complete amd64 package-index hash,
then verifies each selected package record and archive hash. Every regular
extracted member is checked against its authenticated archive before runtime
execution. Extraction is confined to `.toolchain/port-windows-proof/root`.
Copyright/license files remain in that tree. No apt/system installation occurs.

The official dated Rust Windows std component has a pinned SHA-256 and retained
per-file inventory. It is installed into a separate workspace sysroot; the
canonical Rust compiler/sysroot and full compiler reference graph are unchanged.
Compiler identity must match official nightly2026-11-04 exactly. Linking uses
that compiler's LLD plus authenticated MinGW CRT/static support libraries.

Three explicit workspace layout adaptations make the unmodified Debian Wine
binaries portable: its absolute dispatcher script is retained and replaced by a
local symlink to the unchanged wineserver64 ELF, the NLS layout gets a local
symlink, and the dedicated prefix's system32 entries point at the unchanged
authenticated builtin/runtime DLLs. These are recorded in profile.json; they
are not hidden system changes or changes to Rust source/library bodies.

The portable path bridge mechanically compiles the complete actual Go path,
path-traits and Unicode helper files, the actual Option declarations, and the
existing identical path replay/formatter option declarations. There are no
fake SourceFile/session types or replacement path algorithms. The I/O replay
compiles the actual entire Go span package, including its native Windows
backend. Generated fixtures bind selected source declarations and the producer
profile. Native field values and every C/S writer call remain unnormalized,
including actual borrowed-buffer offsets.

```sh
python3 tools/port-rust-source-map/windows-reference/prepare.py --fetch
python3 tools/port-rust-source-map/generate_windows_path.py --check
python3 tools/port-rust-source-map/generate_windows_io.py --check
```

Wine uses a local Unix socket, so a sandbox must allow that process-local socket
binding. All child processes have bounded timeouts. The workspace runtime stays
within the parent's3GB budget; its current installed footprint is about1.5GB.
The Go package fixture replay can validate the portable Windows grammar on
Linux, independently of Wine, while retaining the evidence's emulated label.
