#!/usr/bin/env python3
"""Actual whole-source UnordItems boolean dispatch and owned parameter cleanup."""
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
spec = importlib.util.spec_from_file_location('index_context_tools', HERE.parent / 'index-context/generate.py')
TOOLS = importlib.util.module_from_spec(spec)
spec.loader.exec_module(TOOLS)
spec = importlib.util.spec_from_file_location('verified_span_reference', ROOT / 'tools/port-rust-span-lib/reference.py')
REFERENCE = importlib.util.module_from_spec(spec)
spec.loader.exec_module(REFERENCE)

def sha(value):
    return hashlib.sha256(value).hexdigest()

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--skip-go', action='store_true')
    args = parser.parse_args()
    profile, environment = REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE', ROOT / '.toolchain/oracles/rustc-span-reference/build-profile.json'))
    actual = subprocess.check_output([profile['rustc'], '--version'], text=True).strip()
    if actual != profile['rustc_version'] or not actual.startswith('rustc 1.101.0-nightly (708c94ef0 '):
        raise SystemExit('Boolean SDK differs')
    if profile['source_revision'] != '2d20b42b883dcfc014f3c2c56081eeb6091c2179':
        raise SystemExit('Boolean source revision differs')
    # length, stop, next panic, call panic, item panic, Fn Drop panic,
    # token field panic, iterator Drop panic. Each independent source method
    # receives the same owned cells; all three dispatch families are retained.
    patterns = [
        (0, 0, -1, -1, -1, False, False, False),
        (4, 0, -1, -1, -1, False, False, False),
        (4, 2, -1, -1, -1, False, False, False),
        (4, 99, -1, -1, -1, False, False, False),
        (4, 9, 0, -1, -1, False, False, False),
        (4, 9, 1, -1, -1, False, False, False),
        (4, 9, -1, 0, -1, False, False, False),
        (4, 9, -1, 1, -1, False, False, False),
        (4, 9, -1, -1, 0, False, False, False),
        (4, 9, -1, -1, 99, False, False, False),
        (4, 9, -1, -1, -1, True, False, False),
        (4, 9, -1, -1, -1, False, True, False),
        (4, 9, -1, -1, -1, False, False, True),
        (4, 9, -1, 0, 0, False, False, False),
        (4, 9, -1, 0, -1, True, False, False),
        (4, 9, -1, -1, -1, True, True, False),
        (4, 9, -1, -1, -1, True, False, True),
    ]
    rows, locations = [], {}
    with tempfile.TemporaryDirectory(prefix='ferrugo-unord-boolean-') as temporary:
        executable = str(pathlib.Path(temporary) / 'oracle')
        result = subprocess.run([profile['rustc'], str((HERE / 'constructor_oracle.rs').relative_to(ROOT)), *profile['link_arguments'], '-o', executable], cwd=ROOT, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            raise SystemExit(result.stderr.decode(errors='replace'))
        for mode in ['normal', 'unwind', 'processing']:
            for action in range(4):
                for pattern in patterns:
                    for resume in [False, True]:
                        length, stop, next_fail, call_fail, item_fail, env_fail, token_fail, iterator_fail = pattern
                        arguments = ','.join(str(int(value)) for value in [*pattern, resume])
                        result = subprocess.run([executable, mode, str(action), arguments], stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10, preexec_fn=TOOLS.no_core, env=dict(environment, RUST_BACKTRACE='0'))
                        if result.returncode not in [0, -6]:
                            raise SystemExit(f'Boolean native outcome differs: {result.returncode}' + result.stderr.decode(errors='replace'))
                        stdout = result.stdout.decode()
                        for line in stdout.splitlines():
                            if not line.startswith('HOOK:'):
                                continue
                            fields = line.split(':')
                            payload = bytes.fromhex(fields[2]).decode()
                            key = payload if payload != 'outer' else 'outer.' + mode
                            if payload == 'constructor':
                                key += '.' + str(action)
                            location = {'File': bytes.fromhex(fields[6]).decode(), 'Line': int(fields[7]), 'Column': int(fields[8])}
                            if key in locations and locations[key] != location:
                                raise SystemExit('Boolean location varies: ' + key)
                            locations[key] = location
                        rows.append(dict(mode=mode, action=action, length=length, stop=stop, next_fail=next_fail, call_fail=call_fail, item_fail=item_fail, env_fail=env_fail, token_fail=token_fail, iterator_fail=iterator_fail, resume=resume, outcome='abort' if result.returncode == -6 else 'completed', native_exit_code=result.returncode, stdout=stdout))
    raw = b''.join((json.dumps(row, separators=(',', ':')) + '\n').encode() for row in rows)
    aborts = sum(row['outcome'] == 'abort' for row in rows)
    if len(rows) != 408:
        raise SystemExit('Boolean coverage differs')
    fixture = ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_constructor.jsonl.gz'
    compressed = gzip.compress(raw, mtime=0)
    if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw:
        compressed = fixture.read_bytes()
    names = [
        'port/rust/compiler/rustc_data_structures/src/unord_items.go','port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go',
        'port/rust/compiler/rustc_data_structures/src/unord_items_boolean.go',
        'port/rust/compiler/rustc_data_structures/src/unord_items_constructor_test.go',
        'port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go',
        'port/rust/compiler/rustc_data_structures/src/unord_function.go',
        'tools/port-rust-unord/items/constructor_oracle.rs',
        'tools/port-rust-unord/items/generate_constructor.py',
        'tools/port-rust-unord/items/README.md',
        'tools/port-rust-unord/items/test_tools.py',
    ]
    manifest = dict(scope='Actual unchanged UnordItems map/get_only/count, concrete std Map plus Iterator::map/flat_map overrides using FnOnce and the actual shared-Fn wrapper, complete parameter and field cleanup; remaining adapter proof pending', cases=len(rows), aborts=aborts, compiler=profile['rustc_version'], source_revision=profile['source_revision'], provenance_sha256=profile['provenance_sha256'], reference_inputs=REFERENCE.tool_inputs(), locations=locations, raw_sha256=sha(raw), fixture_sha256=sha(compressed), files={name: sha((ROOT / name).read_bytes()) for name in names})
    metadata = HERE / 'constructor_manifest.json'
    if args.check:
        if metadata.read_text() != json.dumps(manifest, indent=2) + '\n' or fixture.read_bytes() != compressed:
            raise SystemExit('Boolean proof differs')
    else:
        fixture.write_bytes(compressed)
        metadata.write_text(json.dumps(manifest, indent=2) + '\n')
    if not args.skip_go:
        subprocess.run([TOOLS.TOOLS.go_compiler(), 'test', './port/rust/compiler/rustc_data_structures/src', '-run', '^TestOriginalUnordItemsConstructor$', '-count=1'], cwd=ROOT, check=True)
    print(f'{len(rows)} actual Items constructor processes/{aborts} SIGABRT prefixes')

if __name__ == '__main__':
    main()
