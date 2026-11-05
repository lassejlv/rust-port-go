# SPDX-License-Identifier: MIT OR Apache-2.0
"""Environment-specific producer evidence; no tracing production behavior."""
import hashlib
import json
from pathlib import Path

HERE = Path(__file__).absolute().parent
ROOT = HERE.parents[2]
KINDS = {'borrow': ('BORROW_VALIDATION.json', 'borrow_panic.tsv'),
         'registry': ('REGISTRY_VALIDATION.json', 'registry_poison.tsv')}
LIMIT = 1 << 20


def sha(data):
    return hashlib.sha256(data).hexdigest()


def read(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > LIMIT:
        raise ValueError('invalid bounded tracing evidence: ' + str(path))
    return path.read_bytes()


def paths(kind, directory):
    manifest, fixture = KINDS[kind]
    if directory is None:
        return HERE / manifest, ROOT / 'port/support/rusttracing/testdata' / fixture
    if not str(directory):
        raise ValueError('explicit tracing producer directory is empty')
    directory = Path(directory)
    directory = directory if directory.is_absolute() else ROOT / directory
    if directory.is_symlink() or directory.exists() and not directory.is_dir():
        raise ValueError('invalid tracing producer directory')
    result = directory / manifest, directory / fixture
    for path in result:
        if path.exists() or path.is_symlink():
            read(path)
    return result


def bindings():
    # The accepted dispatch proof names the actual used neutral dependencies.
    # Do not accidentally add unrelated live Span/WIP source to this repair.
    names = set(json.loads((HERE.parent / 'dispatch-manifest.json').read_text())['go_bindings'])
    names.update(str(p.relative_to(ROOT)) for p in (ROOT / 'port/support/rusttracing').glob('*.go'))
    names.update(str((HERE / name).relative_to(ROOT)) for name in
                 ['borrow.rs', 'registry.rs', 'generate_borrow.py', 'generate_registry.py',
                  'producer.py', 'test_producer.py', 'README.md'])
    names.update(['tools/port-rust-fatal-error/reference.py',
                  'tools/port-rust-symbol/native_profile.py',
                  'tools/port-rust-span-reference/prepare.py',
                  'tools/port-rust-span-reference/verify.py',
                  'tools/port-rust-span-reference/sources.json',
                  'tools/port-rust-span-reference/toolchain.json',
                  'tools/port-rust-span-reference/Cargo.lock'])
    return {name: sha((ROOT / name).read_bytes()) for name in sorted(names)}


def persist(kind, raw, proof, directory, check):
    if len(raw) > LIMIT:
        raise ValueError('tracing producer raw bound exceeded')
    manifest, fixture = paths(kind, directory)
    if directory is None and read(fixture) != raw:
        raise RuntimeError('tracing native producer differs from history; use --output-dir to retain verbatim filenames')
    proof = {**proof, 'raw_bytes': len(raw), 'source_bindings': bindings(),
             'producer_profile': {'environment_specific': True, 'filenames': 'verbatim native producer'}}
    encoded = (json.dumps(proof, indent=2) + '\n').encode()
    if len(encoded) > LIMIT:
        raise ValueError('tracing producer manifest bound exceeded')
    if check:
        if read(fixture) != raw or read(manifest) != encoded:
            raise RuntimeError('tracing producer proof/bindings differ')
    else:
        fixture.parent.mkdir(parents=True, exist_ok=True)
        manifest.parent.mkdir(parents=True, exist_ok=True)
        fixture.write_bytes(raw)
        manifest.write_bytes(encoded)
