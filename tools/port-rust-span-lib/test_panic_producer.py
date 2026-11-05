"""Panic producer paths and historical preservation; no Rust/Go builds."""
import gzip
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('panic_producer', HERE / 'generate_panic.py')
producer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(producer)


class PanicProducerTests(unittest.TestCase):
    def test_fresh_paths_reject_empty_existing_file_symlinks_and_history_overlap(self):
        history = producer.evidence_paths()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fresh = root / 'fresh'
            self.assertEqual(producer.evidence_paths(str(fresh)),
                             (fresh / 'panic-manifest.json', fresh / 'lib-panic.tsv.gz', fresh / 'lib-panic.tsv'))
            self.assertFalse(fresh.exists())
            file = root / 'file'
            file.write_bytes(b'unowned')
            link = root / 'link'
            link.symlink_to(root, target_is_directory=True)
            for value in ['', str(file), str(link), str(producer.HERE), str(history[1].parent)]:
                with self.subTest(value=value), self.assertRaises(RuntimeError):
                    producer.evidence_paths(value)
            fresh.mkdir()
            (fresh / 'panic-manifest.json').symlink_to(file)
            with self.assertRaisesRegex(RuntimeError, 'regular owned file'):
                producer.evidence_paths(str(fresh))
            self.assertEqual(file.read_bytes(), b'unowned')

    def test_invalid_output_rejects_before_reference_or_compiler(self):
        with mock.patch.object(sys, 'argv', [str(producer.__file__), '--output-dir', '']), \
                mock.patch.object(producer.reference, 'load_profile', side_effect=AssertionError('must reject first')):
            with self.assertRaisesRegex(RuntimeError, 'explicitly empty'):
                producer.main()

    def test_historical_raw_path_drift_corruption_missing_never_overwrites(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'history.gz'
            original = gzip.compress(b'/old/native/registry/path', mtime=123)
            path.write_bytes(original)
            self.assertEqual(producer.producer_fixture(b'/old/native/registry/path', path, True), original)
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                producer.producer_fixture(b'/new/native/registry/path', path, True)
            self.assertEqual(path.read_bytes(), original)
            corrupt = bytearray(original)
            corrupt[-8] ^= 1
            path.write_bytes(corrupt)
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                producer.producer_fixture(b'/old/native/registry/path', path, True)
            self.assertEqual(path.read_bytes(), corrupt)
            path.unlink()
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                producer.producer_fixture(b'new', path, True)
            self.assertFalse(path.exists())

    def test_fresh_equal_gzip_reused_different_data_rebuilt_without_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fresh.gz'
            old = gzip.compress(b'old', mtime=123)
            path.write_bytes(old)
            self.assertEqual(producer.producer_fixture(b'old', path, False), old)
            self.assertEqual(gzip.decompress(producer.producer_fixture(b'new', path, False)), b'new')
            self.assertEqual(path.read_bytes(), old)

    def test_fresh_main_keeps_history_and_captures_raw_all_profiles(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            history = base / 'historical.gz'
            original = gzip.compress(b'historical actual filename', mtime=123)
            history.write_bytes(original)
            fresh = base / 'fresh'
            rows = b'case\ttrue\tstatic_str:61:62:1:2:true:false:2f766572626174696d\t\n'
            profile = {'source_revision': producer.PIN, 'unicode': [18, 0, 0],
                       'rustc': '/verified/rustc', 'link_arguments': ['--verified'],
                       'provenance_sha256': 'a' * 64, 'rustc_version': 'rustc 1.101.0-nightly'}
            def load(path):
                name = path.stem.rsplit('-', 1)[1]
                return dict(profile, debug_assertions=name[1] == '1', overflow_checks=name[3] == '1'), {}
            def run(command, **kwargs):
                if 'stdout' in kwargs:
                    kwargs['stdout'].write(rows)
            with mock.patch.object(sys, 'argv', [str(producer.__file__), '--output-dir', str(fresh)]), \
                    mock.patch.object(producer.reference, 'load_profile', side_effect=load) as loader, \
                    mock.patch.object(producer.reference, 'tool_inputs', return_value={}), \
                    mock.patch.object(producer, 'producer_inputs', return_value=[]), \
                    mock.patch.object(producer.subprocess, 'run', side_effect=run):
                producer.main()
            self.assertEqual(loader.call_count, 4)
            expected = b''.join(name.encode() + b'\t' + rows for name in ['d0o0', 'd0o1', 'd1o0', 'd1o1'])
            self.assertEqual((fresh / 'lib-panic.tsv').read_bytes(), expected)
            self.assertEqual(gzip.decompress((fresh / 'lib-panic.tsv.gz').read_bytes()), expected)
            self.assertEqual(history.read_bytes(), original)
            import json
            manifest = json.loads((fresh / 'panic-manifest.json').read_text())
            self.assertEqual(manifest['artifact_kind'], 'span-lib-panic-producer-v1')
            self.assertIn('No observer normalization', manifest['comparison_profile'])


if __name__ == '__main__':
    unittest.main()
