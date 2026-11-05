"""Fast integrity/selection tests; no installed Rust compiler is required."""
import gzip
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import reference

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
GENERATORS = ['pos','boxes','display','file','flags_format','flags_iter','format','generic','generic_ambient',
              'map','memory','panic','opaque_callbacks','owned_hash','session_inputs','source',
              'trait_defaults','trait_failures','tracing_profile','byte_slice_strategy','macro_backtrace_context','remaining_context']


def minimal_profile(debug=True, overflow=True):
    return {'source_revision': reference.PIN, 'unicode': [18, 0, 0],
            'cfg_bootstrap': False, 'cfg_release': '1.101.0',
            'debug_assertions': debug, 'overflow_checks': overflow}


def evidence_snapshot():
    roots = [HERE, ROOT / 'port/rust/compiler/rustc_span/src/testdata',
             ROOT / 'port/rust/compiler/rustc_serialize/src/testdata']
    return {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
            for root in roots for path in root.iterdir()
            if path.is_file() and (path.name.endswith('-manifest.json') or path.name.endswith('.tsv.gz') or path.name.endswith('.jsonl.gz'))}


class FixtureTests(unittest.TestCase):
    def test_same_raw_preserves_original_gzip_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.gz'
            old = gzip.compress(b'original', compresslevel=1, mtime=123)
            path.write_bytes(old)
            with mock.patch.object(reference.gzip, 'compress', side_effect=AssertionError('must preserve original compressed bytes')):
                self.assertEqual(reference.compressed_fixture(b'original', path, 1024), old)

    def test_bad_crc_is_rebuilt(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.gz'
            old = bytearray(gzip.compress(b'original', mtime=0))
            old[-8] ^= 1
            path.write_bytes(old)
            rebuilt = reference.compressed_fixture(b'original', path, 1024)
            self.assertNotEqual(rebuilt, old)
            self.assertEqual(gzip.decompress(rebuilt), b'original')

    def test_trailing_invalid_data_is_not_preserved(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.gz'
            old = gzip.compress(b'original', mtime=0) + b'invalid'
            path.write_bytes(old)
            self.assertNotEqual(reference.compressed_fixture(b'original', path, 1024), old)

    def test_bounded_decompression_does_not_preserve_expansion(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.gz'
            path.write_bytes(gzip.compress(b'x' * 1000000, mtime=0))
            self.assertEqual(gzip.decompress(reference.compressed_fixture(b'x', path, 1000)), b'x')

    def test_changed_raw_is_rebuilt(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.gz'
            path.write_bytes(gzip.compress(b'old', mtime=0))
            self.assertEqual(gzip.decompress(reference.compressed_fixture(b'new', path, 1024)), b'new')

    def test_new_raw_over_bound_fails(self):
        with self.assertRaisesRegex(RuntimeError, 'exceeds bound'):
            reference.compressed_fixture(b'12345', Path('/unused'), 4)

    def test_symlink_output_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            real = Path(directory) / 'real'
            real.write_bytes(b'unowned')
            path = Path(directory) / 'fixture.gz'
            path.symlink_to(real)
            with self.assertRaisesRegex(RuntimeError, 'symlink'):
                reference.compressed_fixture(b'new', path, 1024)
            self.assertEqual(real.read_bytes(), b'unowned')


class ReferenceTests(unittest.TestCase):
    def test_changed_source_pin_rejected_before_preparation(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'profile.json'
            profile = minimal_profile()
            profile['source_revision'] = '0' * 40
            path.write_text(json.dumps(profile))
            with mock.patch.object(reference.importlib.util, 'spec_from_file_location', side_effect=AssertionError('must reject first')):
                with self.assertRaisesRegex(RuntimeError, 'selection differs'):
                    reference.load_profile(path)

    def test_non_boolean_profile_flags_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'profile.json'
            profile = minimal_profile()
            profile['debug_assertions'] = 'false'
            path.write_text(json.dumps(profile))
            with self.assertRaisesRegex(RuntimeError, 'build flags'):
                reference.load_profile(path)

    def test_all_loads_revalidate_and_use_returned_linkage(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'profile.json'
            profile = minimal_profile(False, True)
            profile.update({'rustc': '/untrusted/cached/command', 'link_arguments': ['untrusted']})
            path.write_text(json.dumps(profile))
            selected = {**minimal_profile(False, True), 'rustc': 'verified', 'link_arguments': ['verified']}
            load = mock.Mock(return_value=(selected, {'VERIFIED': '1'}, {}))
            module = SimpleNamespace(load=load)
            spec = SimpleNamespace(loader=SimpleNamespace(exec_module=lambda value: None))
            with mock.patch.object(reference.importlib.util, 'spec_from_file_location', return_value=spec), mock.patch.object(reference.importlib.util, 'module_from_spec', return_value=module):
                for _ in range(2):
                    actual, environment = reference.load_profile(path)
                    self.assertIs(actual, selected)
                    self.assertEqual(environment, {'VERIFIED': '1'})
            self.assertEqual(load.call_args_list, [mock.call(False, True), mock.call(False, True)])

    def test_explicit_empty_compiler_fails_every_cli_without_evidence_mutation(self):
        before = evidence_snapshot()
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            path = base / 'profile.json'
            path.write_text(json.dumps(minimal_profile()))
            for debug in (False, True):
                for overflow in (False, True):
                    (base / f'build-profile-d{int(debug)}o{int(overflow)}.json').write_text(json.dumps(minimal_profile(debug, overflow)))
            for stem in GENERATORS:
                tool=HERE / ('generate_'+stem+'.py')
                selection = ['--reference-root', str(base)] if tool.name in ['generate_file.py','generate_panic.py','generate_trait_defaults.py','generate_trait_failures.py','generate_tracing_profile.py','generate_byte_slice_strategy.py','generate_macro_backtrace_context.py','generate_remaining_context.py'] else ['--profile', str(path)]
                result = subprocess.run([sys.executable, str(tool), '--check', *selection], cwd=ROOT,
                                        env=dict(os.environ, FERRUGO_RUSTC=''), capture_output=True, timeout=10)
                self.assertNotEqual(result.returncode, 0, tool.name)
                self.assertIn(b'unavailable executable', result.stderr, tool.name)
        self.assertEqual(before, evidence_snapshot())


if __name__ == '__main__':
    unittest.main()
