# SPDX-License-Identifier: MIT OR Apache-2.0
"""Output-selection and historical-preservation integrity, no Rust/Go builds."""
import gzip
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).absolute().parent
sys.path.insert(0, str(HERE))
import panic_site_producer as producer
spec = importlib.util.spec_from_file_location('source_map_panic_sites', HERE / 'generate_panic_sites.py')
generator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generator)


class ProducerTests(unittest.TestCase):
    def test_paths_reject_empty_symlinks_files_history_and_unsupported_scope(self):
        for scope in ('session', 'assert'):
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                fresh = root / 'fresh'
                paths = producer.evidence_paths(scope, str(fresh))
                self.assertEqual(paths[1].name, 'source_map_panic_' + scope + '.tsv.gz')
                self.assertFalse(fresh.exists())
                file = root / 'file'
                file.write_bytes(b'unowned')
                link = root / 'link'
                link.symlink_to(root, target_is_directory=True)
                for value in ('', str(file), str(link), str(producer.evidence_paths(scope)[0].parent)):
                    with self.subTest(scope=scope, value=value), self.assertRaises(RuntimeError):
                        producer.evidence_paths(scope, value)
                fresh.mkdir()
                paths[0].symlink_to(file)
                with self.assertRaisesRegex(RuntimeError, 'regular owned file'):
                    producer.evidence_paths(scope, str(fresh))
                self.assertEqual(file.read_bytes(), b'unowned')
        with self.assertRaises(RuntimeError):
            producer.evidence_paths('all', '/tmp/unused')

    def test_historical_never_overwritten_on_path_drift_corruption_or_missing(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'history.gz'
            old = gzip.compress(b'/old/native/path', mtime=123)
            path.write_bytes(old)
            self.assertEqual(producer.fixture(b'/old/native/path', path, True), old)
            for value in (b'/different/native/path', b'x' * (producer.LIMIT + 1)):
                with self.assertRaises(RuntimeError):
                    producer.fixture(value, path, True)
                self.assertEqual(path.read_bytes(), old)
            corrupt = bytearray(old)
            corrupt[-8] ^= 1
            path.write_bytes(corrupt)
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                producer.fixture(b'/old/native/path', path, True)
            self.assertEqual(path.read_bytes(), corrupt)
            path.unlink()
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                producer.fixture(b'x', path, True)
            self.assertFalse(path.exists())

    def test_fresh_keeps_existing_bytes_and_has_bounded_output(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fresh.gz'
            old = gzip.compress(b'exact filename', mtime=123)
            path.write_bytes(old)
            self.assertEqual(producer.fixture(b'exact filename', path, False), old)
            self.assertEqual(gzip.decompress(producer.fixture(b'new exact filename', path, False)), b'new exact filename')
            self.assertEqual(path.read_bytes(), old)
            with self.assertRaisesRegex(RuntimeError, 'exceeds bound'):
                producer.fixture(b'x' * (producer.LIMIT + 1), path, False)

    def test_invalid_output_rejected_before_reference_compilation(self):
        for args in (['--session-map', '--output-dir', ''], ['--assert-id', '--output-dir', '']):
            with mock.patch.object(sys, 'argv', [str(generator.__file__), *args]), \
                 mock.patch.object(generator.reference, 'load', side_effect=AssertionError('must reject first')):
                with self.assertRaisesRegex(RuntimeError, 'explicitly empty'):
                    generator.main()
        for args in (['--output-dir', '/tmp/unused'], ['--session-map', '--assert-id']):
            with mock.patch.object(sys, 'argv', [str(generator.__file__), *args]), \
                 mock.patch.object(generator.reference, 'load', side_effect=AssertionError('must reject first')):
                with self.assertRaises(SystemExit):
                    generator.main()

    def test_fresh_main_all_profiles_raw_verbatim_and_check(self):
        for scope, option in (('session', '--session-map'), ('assert', '--assert-id')):
            with tempfile.TemporaryDirectory() as directory:
                fresh = Path(directory) / 'fresh'
                def load(debug, overflow, rustc):
                    return {'rustc': '/verified/rustc', 'link_arguments': [], 'provenance_sha256': 'a' * 64,
                            'rustc_version': 'rustc 1.101.0-nightly'}, {}, {}
                def run(command, **kwargs):
                    if 'stdout' in kwargs:
                        key = Path(command[0]).name
                        caught = scope == 'session' or key[1] == '1'
                        event = ('owned_string|/actual producer/filename.rs|168|9|61').encode().hex() if caught else ''
                        kwargs['stdout'].write(('0\t' + str(caught).lower() + '\t' + event + '\n').encode())
                arguments = [str(generator.__file__), option, '--output-dir', str(fresh)]
                with mock.patch.object(sys, 'argv', arguments), \
                     mock.patch.object(generator.reference, 'load', side_effect=load) as loader, \
                     mock.patch.object(generator.reference, 'source_bytes', return_value=b'actual source'), \
                     mock.patch.object(generator.producer, 'input_bindings', return_value={'source.go': 'a' * 64}), \
                     mock.patch.object(generator.subprocess, 'run', side_effect=run):
                    generator.main()
                    self.assertEqual(loader.call_count, 4)
                    with mock.patch.object(sys, 'argv', arguments + ['--check']):
                        generator.main()
                manifest, fixture, raw = producer.evidence_paths(scope, str(fresh))
                self.assertEqual(gzip.decompress(fixture.read_bytes()), raw.read_bytes())
                import json
                proof = json.loads(manifest.read_text())
                self.assertEqual(proof['scope_id'], scope)
                self.assertEqual(proof['schema'], 2)
                self.assertEqual(proof['artifact_kind'], 'source-map-panic-producer-v1')
                location = proof['scoped_tls_location' if scope == 'session' else 'assert_location']
                self.assertEqual(location['File'], '/actual producer/filename.rs')
                self.assertEqual(len(proof['native_profiles']), 4)


if __name__ == '__main__':
    unittest.main()
