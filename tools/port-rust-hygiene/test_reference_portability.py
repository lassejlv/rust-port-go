"""Fast Hygiene selection/cold-profile/artifact checks; no native build runs."""
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'


def module(name):
    spec = importlib.util.spec_from_file_location('isolated_hygiene_' + name, HERE / (name + '.py'))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


def selected(debug, overflow):
    return {'source_revision': PIN, 'debug_assertions': debug, 'overflow_checks': overflow,
            'rustc': 'fresh-verified-compiler', 'link_arguments': ['fresh-verified-linkage']}


def historical_snapshot():
    paths = list(HERE.glob('*-manifest.json'))
    paths += list((ROOT / 'port/rust/compiler/rustc_span/src/testdata').glob('hygiene_*.jsonl.gz'))
    return {str(path): hashlib.sha256(path.read_bytes()).hexdigest() for path in paths}


class VerifiedReferenceTests(unittest.TestCase):
    def test_all_four_cold_profiles_delegate_to_complete_loader(self):
        reference = module('reference')
        calls = []

        def load(debug, overflow, rustc):
            calls.append((debug, overflow, rustc))
            return selected(debug, overflow), {'VERIFIED': '1'}, {'fresh': True}

        with tempfile.TemporaryDirectory() as directory, mock.patch.object(reference, 'ROOT', Path(directory)), \
             mock.patch.object(reference._verified, 'load', side_effect=load):
            self.assertFalse((Path(directory) / '.toolchain').exists())
            profiles = reference.all_profiles('explicit-compiler')
        self.assertEqual(calls, [(d, o, 'explicit-compiler') for d in [False, True] for o in [False, True]])
        self.assertEqual([(d, o) for d, o, *_ in profiles], [(False, False), (False, True), (True, False), (True, True)])
        self.assertTrue(all(p['link_arguments'] == ['fresh-verified-linkage'] for _, _, p, _, _ in profiles))

    def test_main_cold_default_and_untrusted_profile_commands_use_verified_linkage(self):
        reference = module('reference')
        fresh = selected(True, True)
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(reference, 'ROOT', Path(directory)), \
             mock.patch.object(reference._verified, 'load', return_value=(fresh, {'fresh': '1'}, {})) as load:
            self.assertIs(reference.select_main_profile()[0], fresh)
            path = Path(directory) / 'untrusted-profile.json'
            path.write_text(json.dumps({**selected(True, True), 'rustc': '/untrusted/cached/command', 'link_arguments': ['untrusted']}))
            self.assertIs(reference.select_main_profile(path)[0], fresh)
            self.assertEqual(load.call_args_list, [mock.call(True, True, None), mock.call(True, True, None)])

    def test_prepared_never_bypasses_verified_selection(self):
        class RejectedSource(RuntimeError):
            pass
        for name, selector in [('generate', 'select_main_profile'), ('owned_generate', 'all_profiles'), ('commit_generate', 'all_profiles')]:
            for prepared in [False, True]:
                with self.subTest(tool=name, prepared=prepared):
                    tool = module(name)
                    argv = [str(tool.__file__), '--check'] + (['--prepared'] if prepared else [])
                    with mock.patch.object(tool.reference, selector, side_effect=RejectedSource('changed actual source')), \
                         mock.patch.object(tool, 'run', side_effect=AssertionError('must verify before native command')), \
                         mock.patch.object(sys, 'argv', argv):
                        with self.assertRaisesRegex(RejectedSource, 'changed actual source'):
                            tool.main()

    def test_hash_selects_both_required_profiles_without_cached_profile_files(self):
        tool = module('generate')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            here, reference = root / 'tools/port-rust-hygiene', root / 'reference'
            here.mkdir(parents=True)
            reference.mkdir()
            (here / 'hash.rs').write_text('isolated selection input\n')
            (root / 'port/rust/compiler/rustc_span/src/testdata').mkdir(parents=True)
            graph = {'revision': PIN, 'files': []}
            graph_bytes = json.dumps(graph).encode()
            (reference / 'sources.json').write_bytes(graph_bytes)
            (reference / 'toolchain.json').write_text(json.dumps({'rustc': 'matching rustc'}))
            provenance = reference / 'provenance.json'
            provenance.write_text(json.dumps({'source_graph_sha256': hashlib.sha256(graph_bytes).hexdigest()}))

            def profile(debug):
                return {**selected(debug, True), 'rustc': 'verified-' + str(int(debug)),
                        'link_arguments': ['verified-linkage-' + str(int(debug))],
                        'rustc_version': 'matching rustc', 'provenance_path': str(provenance),
                        'provenance_sha256': hashlib.sha256(provenance.read_bytes()).hexdigest()}

            calls, compiled = [], []

            def load(debug, overflow):
                calls.append((debug, overflow))
                return profile(debug), {'FLAG': str(int(debug))}, {}

            def run(command, env):
                if str(command[0]).startswith('verified-'):
                    self.assertEqual(command[0], 'verified-' + env['FLAG'])
                    self.assertIn('verified-linkage-' + env['FLAG'], command)
                    compiled.append(env['FLAG'])
                    return b''
                self.assertEqual(command[-1], '--count')
                return b'0\n'

            with mock.patch.object(tool, 'ROOT', root), mock.patch.object(tool, 'HERE', here), \
                 mock.patch.object(tool, 'REFERENCE', reference), \
                 mock.patch.object(tool.reference, 'select_main_profile', return_value=(profile(True), {'FLAG': '1'}, {})), \
                 mock.patch.object(tool.reference, 'load', side_effect=load), \
                 mock.patch.object(tool.reference, 'tool_inputs', return_value={}), \
                 mock.patch.object(tool, 'run', side_effect=run), mock.patch('builtins.print'), \
                 mock.patch.object(sys, 'argv', [str(tool.__file__), '--prepared', '--suite', 'hash']):
                tool.main()
            self.assertEqual(calls, [(False, True), (True, True)])
            self.assertEqual(compiled, ['1', '0', '1'])
            self.assertFalse((root / '.toolchain/oracles/rustc-span-reference').exists())

    def test_explicit_empty_compiler_rejects_all_prepared_clis_without_evidence_mutation(self):
        before = historical_snapshot()
        for name in ['generate', 'owned_generate', 'commit_generate']:
            result = subprocess.run([sys.executable, str(HERE / (name + '.py')), '--prepared', '--check'],
                                    cwd=ROOT, env=dict(os.environ, FERRUGO_RUSTC=''), capture_output=True, timeout=10)
            self.assertNotEqual(result.returncode, 0, name)
            self.assertIn(b'unavailable executable', result.stderr, name)
        self.assertEqual(before, historical_snapshot())


class ArtifactTests(unittest.TestCase):
    def test_changed_producer_cannot_rewrite_historical_raw_or_gzip(self):
        tool = module('owned_generate')
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'hygiene_owned.jsonl.gz'
            historical = gzip.compress(b'verbatim historical filename\n', mtime=123)
            path.write_bytes(historical)
            self.assertEqual(tool.compressed_fixture(b'verbatim historical filename\n', path, True), historical)
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                tool.compressed_fixture(b'different valid producer filename\n', path, True)
            self.assertEqual(path.read_bytes(), historical)
            fresh = tool.compressed_fixture(b'different valid producer filename\n', path, False)
            self.assertEqual(gzip.decompress(fresh), b'different valid producer filename\n')
            self.assertEqual(path.read_bytes(), historical)

    def test_fresh_output_selects_only_requested_manifest_gzip_and_raw_paths(self):
        tool = module('owned_generate')
        historical = tool.evidence_paths()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fresh = tool.evidence_paths(str(root / 'current producer'))
            self.assertEqual(fresh, (root / 'current producer/owned-manifest.json',
                                     root / 'current producer/hygiene_owned.jsonl.gz',
                                     root / 'current producer/hygiene_owned.jsonl'))
            self.assertFalse(any(path in historical for path in fresh))
            self.assertEqual(tool.evidence_paths(), historical)
            self.assertFalse(fresh[0].parent.exists())

    def test_invalid_output_directory_is_rejected_before_verification(self):
        tool = module('owned_generate')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            file = root / 'file'
            file.write_text('unowned')
            link = root / 'link'
            link.symlink_to(root, target_is_directory=True)
            for value in ['', str(file), str(link)]:
                with self.subTest(directory=value), mock.patch.object(tool.reference, 'all_profiles', side_effect=AssertionError('must reject output selection first')), \
                     mock.patch.object(sys, 'argv', [str(tool.__file__), '--output-dir', value]):
                    with self.assertRaisesRegex(RuntimeError, 'output directory'):
                        tool.main()


if __name__ == '__main__':
    unittest.main()
