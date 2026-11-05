#!/usr/bin/env python3
"""Actual UnordBag StableHash complete Hash/Eq/StableHash bounds and unused effects."""
import argparse
import hashlib
import importlib.util
import json
import os
import pathlib
import subprocess
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
SPEC = importlib.util.spec_from_file_location('verified_reference', ROOT / 'tools/port-rust-span-lib/reference.py')
REFERENCE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REFERENCE)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--skip-go', action='store_true')
    parser.add_argument('--native-only', action='store_true')
    args = parser.parse_args()
    profile, environment = REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE', ROOT / '.toolchain/oracles/rustc-span-reference/build-profile.json'))
    bounds = []
    for name, success in [('positive', True), ('no_hash', False), ('no_eq', False), ('no_stable_hash', False)]:
        with tempfile.TemporaryDirectory(prefix='ferrugo-unord-bag-stable-') as temporary:
            executable = str(pathlib.Path(temporary) / 'bounds')
            result = subprocess.run([profile['rustc'], str((HERE / 'bounds.rs').relative_to(ROOT)), *profile['link_arguments'], '--cfg', name, '-o', executable], cwd=ROOT, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            if (result.returncode == 0) != success:
                raise SystemExit('Actual Bag StableHash bound differs: ' + name + result.stderr.decode(errors='replace'))
            stdout = subprocess.check_output([executable], env=environment).decode() if success else ''
            bounds.append(dict(configuration=name, compiles=success, diagnostic=result.stderr.decode(), stdout=stdout))
    if 'UNEXPECTED' in bounds[0]['stdout'] or bounds[0]['stdout'].count('stable:') != 7:
        raise SystemExit('Actual unused Hash/Eq effects differ')
    names = ['tools/port-rust-unord/bag-stable-bounds/bounds.rs', 'tools/port-rust-unord/bag-stable-bounds/generate.py']
    if not args.native_only:
        names += ['port/rust/compiler/rustc_data_structures/src/unord_bag_stable_bounds.go', 'port/rust/compiler/rustc_data_structures/src/unord_bag_stable_bounds_test.go', 'port/rust/compiler/rustc_data_structures/src/unord_bag.go', 'port/rust/compiler/rustc_data_structures/src/unord_stable_hash.go']
    metadata = dict(scope='Actual UnordBag StableHash Hash+Eq+StableHash bounds, unused Hash/Eq effects and original 0/1/2/4-item fingerprints; no whole Unord credit', bounds=bounds, source_revision=profile['source_revision'], compiler=profile['rustc_version'], provenance_sha256=profile['provenance_sha256'], reference_inputs=REFERENCE.tool_inputs(), files={name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in names})
    destination = HERE / ('native_manifest.json' if args.native_only else 'manifest.json')
    encoded = json.dumps(metadata, indent=2) + '\n'
    if args.check:
        if destination.read_text() != encoded:
            raise SystemExit('Actual Bag StableHash bound proof differs')
    else:
        destination.write_text(encoded)
    if not args.native_only and not args.skip_go:
        selected = os.environ.get('FERRUGO_GO', str(ROOT / '.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go'))
        subprocess.run([selected, 'test', './port/rust/compiler/rustc_data_structures/src', '-run', '^TestOriginalUnordBagStableBounds$', '-count=1'], cwd=ROOT, check=True)
    print('4 actual Bag StableHash compiler bounds / 4 original fingerprints and unused Hash/Eq effects')


if __name__ == '__main__':
    main()
