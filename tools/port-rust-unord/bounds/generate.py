#!/usr/bin/env python3
"""Actual unchanged source admission and effects for the complete bound family."""
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
    names = ['positive', 'no_default', 'new_no_default', 'clone_from_self', 'clone_from_opaque', 'clone_item', 'clone_iterator', 'cloned_value', 'copied_value', 'max_ord', 'min_ord', 'sum_trait', 'product_trait', 'key_eq', 'key_partial_ord', 'key_ord', 'key_stable_hash', 'key_sized']
    with tempfile.TemporaryDirectory(prefix='ferrugo-unord-bounds-') as temporary:
        for name in names:
            executable = str(pathlib.Path(temporary) / name)
            result = subprocess.run([profile['rustc'], str((HERE / 'bounds.rs').relative_to(ROOT)), *profile['link_arguments'], '--cfg', name, '-o', executable], cwd=ROOT, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            success = name in ['positive', 'new_no_default']
            if (result.returncode == 0) != success:
                raise SystemExit('Actual complete source bound differs: ' + name + result.stderr.decode(errors='replace'))
            if success:
                subprocess.run([executable], env=environment, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            bounds.append(dict(configuration=name, compiles=success, diagnostic=result.stderr.decode()))
        executable = str(pathlib.Path(temporary) / 'oracle')
        result = subprocess.run([profile['rustc'], str((HERE / 'oracle.rs').relative_to(ROOT)), *profile['link_arguments'], '-o', executable], cwd=ROOT, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            raise SystemExit(result.stderr.decode(errors='replace'))
        cases = []
        for group, family, cache in [('primitive', n, False) for n in range(8)] + [('key', n, cache) for n in range(7) for cache in [False, True]]:
            result = subprocess.run([executable, group, str(family), str(int(cache))], env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10)
            if result.returncode or 'UNEXPECTED' in result.stdout.decode():
                raise SystemExit('Actual unused bound effect differs: ' + result.stdout.decode() + result.stderr.decode())
            lines = result.stdout.decode().splitlines(keepends=True)
            allocation = [line.strip() for line in lines if line.startswith('ALLOC:')]
            cases.append(dict(group=group, family=family, cache=cache, stdout=''.join(line for line in lines if not line.startswith('ALLOC:')), allocation=allocation))
    files = ['tools/port-rust-unord/bounds/bounds.rs', 'tools/port-rust-unord/bounds/oracle.rs', 'tools/port-rust-unord/bounds/generate.py']
    if not args.native_only:
        files += ['port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go', 'port/rust/compiler/rustc_data_structures/src/unord_bound_primaries_test.go', 'port/rust/compiler/rustc_data_structures/src/unord_items.go', 'port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go', 'port/rust/compiler/rustc_data_structures/src/unord_items_borrowed.go', 'port/rust/compiler/rustc_data_structures/src/unord_items_sort.go', 'port/rust/compiler/rustc_data_structures/src/unord_items_vector.go', 'port/rust/compiler/rustc_data_structures/src/unord_collections_sort.go', 'port/rust/compiler/rustc_data_structures/src/unord_values_sorted.go', 'port/rust/compiler/rustc_data_structures/src/unord_bag.go']
    metadata = dict(scope='Actual full Unord Default/Clone/Copy/Ord/Sum/Product/associated key admission bounds, same iterator Self and opaque Clone exposure; unused borrowed trait effects absent; no full std or Unord credit', bounds=bounds, cases=cases, source_revision=profile['source_revision'], compiler=profile['rustc_version'], provenance_sha256=profile['provenance_sha256'], reference_inputs=REFERENCE.tool_inputs(), files={name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in files})
    destination = HERE / ('native_manifest.json' if args.native_only else 'manifest.json')
    encoded = json.dumps(metadata, indent=2) + '\n'
    if args.check:
        if destination.read_text() != encoded:
            raise SystemExit('Actual complete bound proof differs')
    else:
        destination.write_text(encoded)
    if not args.native_only and not args.skip_go:
        selected = os.environ.get('FERRUGO_GO', str(ROOT / '.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go'))
        subprocess.run([selected, 'test', './port/rust/compiler/rustc_data_structures/src', '-run', '^TestOriginalUnordCompleteBounds$', '-count=1'], cwd=ROOT, check=True)
    print('18 actual source admission programs / 22 original bound-effect programs')


if __name__ == '__main__':
    main()
