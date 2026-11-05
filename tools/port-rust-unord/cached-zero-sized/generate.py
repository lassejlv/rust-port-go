#!/usr/bin/env python3
"""Actual unchanged SDK cached-key sorting on logical ZST slices."""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
import pathlib
import subprocess
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
spec = importlib.util.spec_from_file_location('sdk_tools', HERE.parent / 'sort/generate.py')
TOOLS = importlib.util.module_from_spec(spec)
spec.loader.exec_module(TOOLS)

def sha(data):
    return hashlib.sha256(data).hexdigest()

def no_core():
    import resource
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))

def requests():
    for kind in [0, 1, 3]:
        base = dict(kind=kind, chain=False, length=4, seed=1, key_fail=-1,
                    cmp_fail=-1, drop_fail=-1, fn_fail=False,
                    token_fail=False, moved=False)
        variants = [dict(length=n) for n in [0, 1, 2, 4, 21, -1]]
        variants += [dict(key_fail=n) for n in [0, 2]]
        variants += [dict(cmp_fail=n) for n in [0, 2]]
        if kind != 0:
            variants += [dict(drop_fail=n) for n in [0, 2]]
        variants += [dict(fn_fail=True), dict(token_fail=True),
                     dict(fn_fail=True, token_fail=True),
                     dict(length=-1, fn_fail=True),
                     dict(key_fail=0, fn_fail=True), dict(moved=True)]
        for chain in [False, True]:
            for variant in variants:
                yield dict(base, chain=chain, **variant)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--skip-go', action='store_true')
    parser.add_argument('--native-only', action='store_true')
    args = parser.parse_args()
    rustc = TOOLS.selected('FERRUGO_RUSTC_NIGHTLY', ROOT / '.toolchain/rust-nightly-2026-11-04/bin/rustc')
    version = subprocess.check_output([rustc, '--version'], text=True).strip()
    if version != TOOLS.VERSION:
        raise SystemExit('ZST cached-key SDK differs')
    sysroot = pathlib.Path(subprocess.check_output([rustc, '--print', 'sysroot'], text=True).strip())
    libraries = {}
    for name in ['core', 'alloc', 'std']:
        paths = list((sysroot / 'lib/rustlib/x86_64-unknown-linux-gnu/lib').glob('lib' + name + '-*.rlib'))
        if len(paths) != 1 or paths[0].is_symlink():
            raise SystemExit('Expected exactly one regular SDK library')
        libraries[paths[0].name] = sha(paths[0].read_bytes())
    rows, locations = [], {}
    with tempfile.TemporaryDirectory(prefix='ferrugo-zst-cached-') as temporary:
        executable = str(pathlib.Path(temporary) / 'oracle')
        result = subprocess.run([rustc, '--edition=2024', str((HERE / 'oracle.rs').relative_to(ROOT)), '-o', executable], cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            raise SystemExit(result.stderr.decode(errors='replace'))
        for mode in ['normal', 'unwind', 'processing']:
            for request in requests():
                for resume in [False, True]:
                    values = [request[name] for name in ['kind', 'chain', 'length', 'seed', 'key_fail', 'cmp_fail', 'drop_fail', 'fn_fail', 'token_fail', 'moved']]
                    command = [executable, '--child', mode, ','.join(str(int(value)) for value in [*values, resume])]
                    result = subprocess.run(command, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10, preexec_fn=no_core, env=dict(os.environ, RUST_BACKTRACE='0'))
                    if result.returncode not in [0, -6]:
                        raise SystemExit(f'Unexpected ZST SDK outcome: {result.returncode}' + result.stderr.decode(errors='replace'))
                    stdout = result.stdout.decode()
                    for line in stdout.splitlines():
                        if not line.startswith('HOOK:'):
                            continue
                        fields = line.split(':')
                        payload = bytes.fromhex(fields[2]).decode()
                        key = payload if payload != 'outer' else 'outer.' + mode
                        location = dict(File=bytes.fromhex(fields[6]).decode(), Line=int(fields[7]), Column=int(fields[8]))
                        if key in locations and locations[key] != location:
                            raise SystemExit('ZST source location differs: ' + key)
                        locations[key] = location
                    rows.append(dict(context=mode, request=request, resume=resume, outcome='abort' if result.returncode == -6 else 'completed', native_exit_code=result.returncode, stdout=stdout))
    if len(rows) != 624:
        raise SystemExit('ZST coverage differs: ' + str(len(rows)))
    raw = b''.join((json.dumps(row, separators=(',', ':')) + '\n').encode() for row in rows)
    aborts = sum(row['outcome'] == 'abort' for row in rows)
    fixture = ROOT / 'port/support/rustsort/testdata/cached_zero_sized.jsonl.gz'
    compressed = gzip.compress(raw, mtime=0)
    if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw:
        compressed = fixture.read_bytes()
    names = ['tools/port-rust-unord/cached-zero-sized/oracle.rs',
             'tools/port-rust-unord/cached-zero-sized/generate.py',
             'tools/port-rust-unord/cached-zero-sized/README.md',
             'tools/port-rust-unord/cached-zero-sized/test_tools.py']
    if not args.native_only:
        names += ['port/support/rustsort/cached_zero_sized.go',
                  'port/support/rustsort/cached_zero_sized_test.go',
                  'port/support/rustsort/cached_oracle_test.go',
                  'port/support/rustsort/cached_context_test.go']
        names += [str(path.relative_to(ROOT)) for path in sorted((ROOT / 'port/support/rustsort').glob('*.go')) if not path.name.endswith('_test.go') and path.name != 'cached_zero_sized.go']
        names += ['port/support/rustvec/collect.go', 'port/support/rustvec/scoped_consumer.go', 'port/support/rustvec/ownership.go', 'port/support/rustvec/panic_sites.go']
    manifest = dict(scope='Actual unchanged SDK cached-key sort of full-uint logical ZST slices; no std file completion claim', cases=len(rows), aborts=aborts, compiler=subprocess.check_output([rustc, '-vV'], text=True).strip(), sdk_libraries=libraries, locations=locations, raw_sha256=sha(raw), fixture_sha256=sha(compressed), files={name: sha((ROOT / name).read_bytes()) for name in names}, allocation_evidence='ALLOC is an actual native global-allocator observation of the first key-vector allocation before the first key callback. Go source index layout is checked against it; no Go allocator equivalence is claimed.')
    metadata = HERE / ('native_manifest.json' if args.native_only else 'manifest.json')
    if args.check:
        if metadata.read_text() != json.dumps(manifest, indent=2) + '\n' or fixture.read_bytes() != compressed:
            raise SystemExit('ZST cached-key proof differs')
    else:
        fixture.write_bytes(compressed)
        metadata.write_text(json.dumps(manifest, indent=2) + '\n')
    if not args.skip_go and not args.native_only:
        subprocess.run([TOOLS.go_compiler(), 'test', './port/support/rustsort', '-run', '^TestOriginalCachedZeroSized$', '-count=1'], cwd=ROOT, check=True)
    print(f'{len(rows)} actual logical ZST SDK cases/{aborts} SIGABRT prefixes')

if __name__ == '__main__':
    main()
