"""Complete scoped Go dependency bindings for the two normal Span clients."""
from pathlib import Path
import reference

ROOT = Path(__file__).resolve().parents[2]

def inputs():
    packages = [
        'port/rust/compiler/rustc_span/src',
        'port/rust/compiler/rustc_serialize/src',
        'port/rust/compiler/rustc_data_structures/src',
        'port/rust/compiler/rustc_index/src',
        'port/rust/compiler/rustc_hashes/src',
        'port/dependencies/rustc-stable-hash/v0_1_2/src',
        'port/dependencies/blake3/v1_8_2/src',
        'port/dependencies/unicode-width/v0_2_2/src',
        'port/rust/library/core/src/unicode',
        'port/support/rustfmt', 'port/support/rustvec',
        'port/support/ruststr',
        'port/support/rustsort', 'port/support/rusthashmap',
        'port/support/rusttracing', 'port/support/rustindexmap',
        'port/support/ruststdio', 'port/support/rusterrno',
    ]
    paths = {path for package in packages for path in (ROOT / package).rglob('*.go')}
    paths.update(ROOT / name for name in reference.tool_inputs())
    paths.add(Path(__file__).resolve())
    return sorted(paths)
