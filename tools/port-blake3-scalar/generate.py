#!/usr/bin/env python3
"""Validate the scoped unkeyed adapter against the actual locked BLAKE3 crate."""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tomllib

ROOT = Path(__file__).resolve().parents[2]
HERE = ROOT / 'tools/port-blake3-scalar'
TARGET = ROOT / 'port/dependencies/blake3/v1_8_2'
CHECKSUM = '3888aaa89e4b2a40fca9848e400f6a658a5a3978de7be858e209cafa8be9a4a0'
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def run(args, env):
    return subprocess.check_output([str(x) for x in args], cwd=ROOT, env=env, stderr=subprocess.PIPE, timeout=180)


def executable(variable, relative, fallback):
    value = os.environ.get(variable, str(ROOT / relative) if (ROOT / relative).exists() else fallback)
    if not value:
        raise RuntimeError(variable + ' explicitly empty')
    resolved = shutil.which(value)
    if not resolved:
        raise RuntimeError('unavailable executable: ' + value)
    return os.path.abspath(resolved)


def requests():
    lengths = set(range(257))
    for power in range(6, 22):
        lengths.update([(1 << power) - 1, 1 << power, (1 << power) + 1])
    for count in range(1, 66):
        lengths.update([1024 * count - 1, 1024 * count, 1024 * count + 1])
    for length in sorted(lengths):
        if length > 2 * 1024 * 1024:
            continue
        strides = [0, 1, 63, 64, 65, 1023, 1024, 1025]
        if length > 8193:
            strides = [0, 63, 1023, 1025]
        for seed in [0, 255]:
            for stride in strides:
                yield f'{length} {seed} {stride}'


def json_bytes(data):
    return (json.dumps(data, indent=2) + '\n').encode()


def persist(path, data, check):
    if check:
        if path.read_bytes() != data:
            raise RuntimeError('retained evidence differs: ' + str(path))
    else:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--fetch', action='store_true')
    args = parser.parse_args()
    env = os.environ.copy()
    env['RUSTC'] = executable('FERRUGO_RUSTC', Path('.toolchain/rust/bin/rustc'), 'rustc')
    cargo = executable('FERRUGO_CARGO', Path('.toolchain/rust/bin/cargo'), 'cargo')
    env['CARGO_HOME'] = str(Path(env.get('CARGO_HOME', ROOT / '.toolchain/port-cargo-home')).absolute())
    env['CARGO_TARGET_DIR'] = str(ROOT / '.toolchain/oracles/blake3-scalar-target')
    env['GIT_NO_LAZY_FETCH'] = '1'
    for key in ['RUSTFLAGS', 'CARGO_ENCODED_RUSTFLAGS', 'RUSTC_WRAPPER', 'RUSTC_WORKSPACE_WRAPPER']:
        env.pop(key, None)
    compiler = run([env['RUSTC'], '--version'], env).decode().strip()
    if compiler != 'rustc 1.99.1 (58cf8f9c3 2026-10-13)':
        raise RuntimeError('reference compiler differs: ' + compiler)
    command = [cargo, '--manifest-path', HERE / 'Cargo.toml']
    if args.fetch:
        run([cargo, 'fetch', '--locked', '--manifest-path', HERE / 'Cargo.toml'], env)
    lock = (HERE / 'Cargo.lock').read_bytes()
    cache = Path(env['CARGO_HOME'])
    upstream = Path(env.get('FERRUGO_RUST_SOURCE', ROOT / '.toolchain/upstream-rust'))
    original_lock = run(['git', '-C', upstream, 'show', PIN + ':Cargo.lock'], env)
    pins = {(p['name'], p['version']): p.get('checksum') for p in tomllib.loads(original_lock.decode())['package']}
    spec = importlib.util.spec_from_file_location('blake3_registry_proof', ROOT / 'tools/port-semver/original-tests/registry_source.py')
    registry = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(registry)
    packages = [p for p in tomllib.loads(lock.decode())['package'] if p.get('source')]

    def verify():
        evidence = []
        for package in packages:
            name, version = package['name'], package['version']
            if package['source'] != 'registry+https://github.com/rust-lang/crates.io-index' or pins.get((name, version)) != package['checksum']:
                raise RuntimeError('dependency differs from pinned Rust lock: ' + name)
            archives = list((cache / 'registry/cache').glob(f'*/{name}-{version}.crate'))
            if len(archives) != 1 or archives[0].is_symlink() or sha(archives[0].read_bytes()) != package['checksum']:
                raise RuntimeError('missing/changed archive: ' + name)
            detail = registry.verify_registry_source(archives[0], cache, name, version)
            evidence.append({'name': name, 'version': version, 'sha256': package['checksum'], **detail})
        return evidence

    dependencies = verify()
    archive = list((cache / 'registry/cache').glob('*/blake3-1.8.2.crate'))[0]
    if sha(archive.read_bytes()) != CHECKSUM:
        raise RuntimeError('BLAKE3 archive differs')
    originals = {}
    with tarfile.open(archive) as contents:
        for member in contents:
            if not member.isfile():
                continue
            relative = Path(member.name).relative_to('blake3-1.8.2')
            if '..' in relative.parts:
                raise RuntimeError('invalid archive member')
            originals[str(relative)] = contents.extractfile(member).read()
    retained = ['LICENSE_A2', 'LICENSE_A2LLVM', 'LICENSE_CC0', 'README.md', 'Cargo.toml', 'Cargo.toml.orig', '.cargo_vcs_info.json']
    for name in retained:
        destination = TARGET / ('upstream/' + name if name not in retained[:3] else name)
        persist(destination, originals[name], args.check)
    inventory = []
    for name, data in sorted(originals.items()):
        status = 'partial' if name in ['src/lib.rs', 'src/portable.rs', 'src/platform.rs'] else 'unported'
        inventory.append({'path': name, 'bytes': len(data), 'sha256': sha(data), 'status': 'retained' if name in retained else status})
    source_info = {'crate': 'blake3', 'version': '1.8.2', 'archive_sha256': CHECKSUM, 'status': 'partial', 'scope': 'Only unkeyed NewHasher/Update/nonconsuming Finalize/Hash operations; portable scalar execution. No complete source-file or crate credit.', 'files': inventory}
    persist(TARGET / 'source-manifest.json', json_bytes(source_info), args.check)
    inputs = [HERE / 'Cargo.toml', HERE / 'Cargo.lock', HERE / 'oracle.rs', Path(__file__), TARGET / 'src/hash.go']
    frozen = {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in inputs}
    build = run([cargo, 'build', '--locked', '--offline', '--manifest-path', HERE / 'Cargo.toml', '--message-format=json'], env)
    binary = next(json.loads(line)['executable'] for line in build.splitlines() if json.loads(line).get('reason') == 'compiler-artifact' and json.loads(line).get('executable'))
    rows = list(requests())
    output = subprocess.check_output([binary], input=('\n'.join(rows) + '\n').encode(), env=env, timeout=180).decode().splitlines()
    if len(rows) != len(output) or any(len(line.split()) != 4 for line in output):
        raise RuntimeError('native response count/shape differs')
    raw = ''.join(request + ' ' + answer + '\n' for request, answer in zip(rows, output)).encode()
    fixture = TARGET / 'src/testdata/native.tsv.gz'
    compressed = fixture.read_bytes() if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw else gzip.compress(raw, mtime=0)
    if args.check and (not fixture.exists() or gzip.decompress(fixture.read_bytes()) != raw):
        raise RuntimeError('native hash observations changed')
    if dependencies != verify() or frozen != {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in inputs}:
        raise RuntimeError('verified source inputs changed during execution')
    persist(fixture, compressed, args.check)
    info = {'format': 1, 'compiler': compiler, 'rust_revision': PIN, 'source_scope': 'partial-dependency-used-operations', 'requests': len(rows), 'max_input': 2 * 1024 * 1024, 'request_sha256': sha(('\n'.join(rows) + '\n').encode()), 'raw_sha256': sha(raw), 'fixture_sha256': sha(compressed), 'inputs_sha256': frozen, 'dependencies': dependencies}
    persist(HERE / 'manifest.json', json_bytes(info), args.check)
    print(f'BLAKE3 scoped portable hashing: {len(rows)} actual native requests; fixture {"checked" if args.check else "retained"}.')


if __name__ == '__main__':
    main()
