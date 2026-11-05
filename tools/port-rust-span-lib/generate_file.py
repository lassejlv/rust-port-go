#!/usr/bin/env python3
"""Original FileEncoder I/O/flush observations in four actual build profiles.

The standalone client links the unchanged, source-verified whole-span graph.
This scoped serializer evidence grants no rustc_serialize whole-file credit.
"""
import argparse
import gzip
import hashlib
import io
import json
import os
import pathlib
import subprocess
import tempfile
import reference

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = pathlib.Path(__file__).resolve().parent
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'
LIMIT = 2 * 1024 * 1024


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference-root', type=pathlib.Path, default=ROOT / '.toolchain/oracles/rustc-span-reference')
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    source = HERE / 'native_file.rs'
    destination = ROOT / 'port/rust/compiler/rustc_serialize/src/testdata/file.tsv.gz'
    manifest = HERE / 'file-manifest.json'
    profiles = []
    all_rows = []
    for debug in (False, True):
        for overflow in (False, True):
            name = f'd{int(debug)}o{int(overflow)}'
            profile, environment = reference.load_profile(args.reference_root / f'build-profile-{name}.json')
            if profile['source_revision'] != PIN or profile['debug_assertions'] != debug or profile['overflow_checks'] != overflow or profile['unicode'] != [18, 0, 0]:
                raise SystemExit(f'invalid original reference profile {name}')
            with tempfile.TemporaryDirectory(prefix='ferrugo-file-codec-') as directory:
                executable = pathlib.Path(directory) / 'native'
                subprocess.run([profile['rustc'], str(source), *profile['link_arguments'], '-o', str(executable)], check=True, cwd=ROOT, env=environment, timeout=120)
                output = pathlib.Path(directory) / 'output'
                with output.open('wb') as stream:
                    subprocess.run([str(executable), directory], check=True, stdout=stream, cwd=ROOT, timeout=30)
                if output.stat().st_size > LIMIT:
                    raise SystemExit('native file output exceeds bound')
                raw = output.read_bytes()
            all_rows.extend(name.encode() + b'\t' + row for row in raw.splitlines(keepends=True))
            profiles.append({'name': name, 'reference_provenance_sha256': profile['provenance_sha256'], 'rustc_version': profile['rustc_version'], 'raw_sha256': sha(raw), 'records': len(raw.splitlines())})
    raw = b''.join(all_rows)
    if len(raw) > LIMIT:
        raise SystemExit('file fixture exceeds bound')
    compressed = reference.compressed_fixture(raw, destination, LIMIT)
    proof = {'schema': 1, 'status': 'partial-serializer-validation', 'source_revision': PIN, 'profiles': profiles, 'tool_inputs_sha256': reference.tool_inputs(), 'client_sha256': sha(source.read_bytes()), 'generator_sha256': sha(pathlib.Path(__file__).read_bytes()), 'records': len(raw.splitlines()), 'raw_sha256': sha(raw), 'fixture_sha256': sha(compressed)}
    encoded = (json.dumps(proof, indent=2, sort_keys=True) + '\n').encode()
    if args.check:
        if not destination.exists() or destination.read_bytes() != compressed or not manifest.exists() or manifest.read_bytes() != encoded:
            raise SystemExit('file codec evidence is stale')
    else:
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(compressed)
        manifest.write_bytes(encoded)
    print(f'file codec native records: {proof["records"]}')


if __name__ == '__main__':
    try:
        main()
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
        raise SystemExit(f'native file command failed: {error}')
