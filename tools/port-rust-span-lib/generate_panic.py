#!/usr/bin/env python3
"""Original lib.rs panic metadata in four actual build profiles.

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
import span_wrapper_evidence

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = pathlib.Path(__file__).resolve().parent
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'
LIMIT = 2 * 1024 * 1024


def sha(data):
    return hashlib.sha256(data).hexdigest()


def evidence_paths(output_dir=None):
    if output_dir is None:
        return (HERE / 'panic-manifest.json',
                ROOT / 'port/rust/compiler/rustc_span/src/testdata/lib-panic.tsv.gz', None)
    if output_dir == '':
        raise RuntimeError('panic --output-dir explicitly empty')
    directory = pathlib.Path(output_dir).absolute()
    if directory.is_symlink() or directory.exists() and not directory.is_dir():
        raise RuntimeError('invalid panic output directory')
    paths = (directory / 'panic-manifest.json', directory / 'lib-panic.tsv.gz', directory / 'lib-panic.tsv')
    if any(path.resolve() == old.resolve() for path in paths for old in evidence_paths()[:2]):
        raise RuntimeError('fresh --output-dir overlaps historical evidence')
    for path in paths:
        if path.is_symlink() or path.exists() and not path.is_file():
            raise RuntimeError('panic evidence is not a regular owned file: ' + str(path))
    return paths


def producer_fixture(raw, destination, historical):
    if len(raw) > LIMIT:
        raise RuntimeError('panic fixture exceeds bound')
    if historical:
        if destination.is_symlink() or not destination.is_file():
            raise RuntimeError('historical panic evidence unavailable; use --output-dir')
        old = destination.read_bytes()
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(old)) as stream:
                previous = stream.read(LIMIT + 1)
        except (OSError, EOFError):
            raise RuntimeError('historical panic evidence invalid; use --output-dir')
        if previous != raw:
            raise RuntimeError('historical producer filenames/observations differ; use --output-dir')
        return old
    return reference.compressed_fixture(raw, destination, LIMIT)


def producer_inputs():
    # Bind complete production dependencies and this self-contained replay,
    # without coupling the producer to unrelated sibling test additions.
    paths = [p for p in span_wrapper_evidence.inputs() if not p.name.endswith('_test.go')]
    paths += [HERE / name for name in ['generate_panic.py', 'native_lib_panic.rs', 'native_source_helpers.rs', 'test_panic_producer.py']]
    paths += [ROOT / 'port/rust/compiler/rustc_span/src' / name for name in ['lib_panic_oracle_test.go', 'lib_panic_profile_test.go', 'lib_span_proof_bindings_test.go']]
    return sorted(set(paths))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference-root', type=pathlib.Path, default=ROOT / '.toolchain/oracles/rustc-span-reference')
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--output-dir', help='fresh verbatim producer manifest, gzip and raw TSV; preserves historical evidence')
    args = parser.parse_args()
    source = HERE / 'native_lib_panic.rs'
    manifest, destination, raw_artifact = evidence_paths(args.output_dir)
    paths = producer_inputs()
    bindings = {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in paths}
    selected_tools = reference.tool_inputs()
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
                    raise SystemExit('native panic output exceeds bound')
                raw = output.read_bytes()
            all_rows.extend(name.encode() + b'\t' + row for row in raw.splitlines(keepends=True))
            profiles.append({'name': name, 'reference_provenance_sha256': profile['provenance_sha256'], 'rustc_version': profile['rustc_version'], 'raw_sha256': sha(raw), 'records': len(raw.splitlines())})
    raw = b''.join(all_rows)
    if len(raw) > LIMIT:
        raise SystemExit('file fixture exceeds bound')
    compressed = producer_fixture(raw, destination, args.output_dir is None)
    if paths != producer_inputs() or bindings != {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in paths} or selected_tools != reference.tool_inputs():
        raise RuntimeError('panic producer source/replay/tool inputs changed during execution')
    proof = {'schema': 1, 'status': 'partial-lib-validation', 'source_revision': PIN,  'profiles': profiles, 'location_profile': {'name':'pinned-whole-span-1.101-portable-paths','lib_file':'original/compiler/rustc_span/src/lib.rs','scoped_tls_file':'registry/scoped-tls-1.0.1/src/lib.rs','normalization':'Only an absolute original build prefix and Cargo registry host/index prefix are normalized; std /rustc/commit paths remain exact. Each hook record also retains the unmodified native filename.','raw_native_filenames_retained':True}, 'tool_inputs_sha256': reference.tool_inputs(), 'client_sha256': sha(source.read_bytes()), 'source_helper_sha256': sha((HERE / 'native_source_helpers.rs').read_bytes()), 'generator_sha256': sha(pathlib.Path(__file__).read_bytes()), 'records': len(raw.splitlines()), 'raw_sha256': sha(raw), 'fixture_sha256': sha(compressed)}
    if args.output_dir is not None:
        proof.update({'schema': 2, 'artifact_kind': 'span-lib-panic-producer-v1', 'files': bindings,
                      'comparison_profile': 'Exact unmodified native filename field; legacy normalized field retained for historical audit only. No observer normalization.'})
    encoded = (json.dumps(proof, indent=2, sort_keys=True) + '\n').encode()
    if args.check:
        if not destination.exists() or destination.read_bytes() != compressed or not manifest.exists() or manifest.read_bytes() != encoded or raw_artifact is not None and (not raw_artifact.is_file() or raw_artifact.read_bytes() != raw):
            raise SystemExit('lib panic evidence is stale')
    else:
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(compressed)
        manifest.parent.mkdir(parents=True, exist_ok=True)
        manifest.write_bytes(encoded)
        if raw_artifact is not None:
            raw_artifact.write_bytes(raw)
    print(f'lib panic native records: {proof["records"]}')


if __name__ == '__main__':
    try:
        main()
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
        raise SystemExit(f'native panic command failed: {error}')
