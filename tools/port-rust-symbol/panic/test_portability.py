"""Producer selection, mandatory verification and compressed evidence laws."""
import gzip
import importlib.util
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock
from contextlib import redirect_stdout

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
spec = importlib.util.spec_from_file_location('symbol_panic_producer', HERE / 'generate.py')
producer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(producer)


class ProducerSelection(unittest.TestCase):
    def test_cold_four_profiles_use_complete_verification(self):
        configs = [(f'd{int(d)}o{int(o)}', d, o) for d in (False, True) for o in (False, True)]
        with mock.patch.object(producer.symbol_reference, 'load', side_effect=lambda d, o: (d, o)) as load:
            selected = producer.load_profiles(configs)
        self.assertEqual(list(selected), ['d0o0', 'd0o1', 'd1o0', 'd1o1'])
        self.assertEqual(load.call_args_list, [mock.call(d, o) for _, d, o in configs])

    def test_prepared_cli_cannot_skip_complete_verifier(self):
        with tempfile.TemporaryDirectory() as directory:
            with mock.patch.object(sys, 'argv', ['generate.py', '--prepared', '--check', '--output-dir', directory]), mock.patch.object(producer.resource, 'setrlimit'), mock.patch.object(producer, 'load_profiles', side_effect=RuntimeError('verification reached')) as load:
                with self.assertRaisesRegex(RuntimeError, 'verification reached'):
                    producer.main()
                self.assertEqual(len(load.call_args.args[0]), 4)

    def test_output_directory_is_explicit_and_relocatable(self):
        with tempfile.TemporaryDirectory() as directory:
            manifest, compressed, raw = producer.output_paths(directory)
            self.assertEqual([p.name for p in [manifest, compressed, raw]], ['manifest.json', 'symbol_panic.jsonl.gz', 'symbol_panic.jsonl'])
            with self.assertRaises(ValueError):
                producer.output_paths('')
            invalid = Path(directory) / 'file'
            invalid.write_text('not a directory')
            with self.assertRaises(ValueError):
                producer.output_paths(invalid)
            link = Path(directory) / 'link'
            link.symlink_to(directory, target_is_directory=True)
            with self.assertRaises(ValueError):
                producer.output_paths(link)

    def test_gzip_preserves_valid_bytes_and_rejects_corrupt_crc_and_bounds(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'data.gz'
            raw = b'verbatim producer filename /another/cache/source.rs\n'
            existing = gzip.compress(raw, mtime=123)
            path.write_bytes(existing)
            self.assertEqual(producer.symbol_reference.compressed_fixture(raw, path), existing)
            corrupt = bytearray(existing)
            corrupt[-8] ^= 1
            path.write_bytes(corrupt)
            fresh = producer.symbol_reference.compressed_fixture(raw, path)
            self.assertNotEqual(fresh, bytes(corrupt))
            self.assertEqual(gzip.decompress(fresh), raw)
            with self.assertRaises(ValueError):
                producer.symbol_reference.compressed_fixture(raw, path, limit=3)

    def test_changed_producer_requires_explicit_output_and_preserves_history(self):
        with tempfile.TemporaryDirectory() as directory:
            historical = Path(directory) / 'historical.gz'
            original = b'verbatim /original/cache/source.rs:168:9\n'
            changed = b'verbatim /alternate/cache/source.rs:168:9\n'
            compressed = gzip.compress(original, mtime=123)
            historical.write_bytes(compressed)
            producer.preserve_historical_producer(original, historical, False)
            self.assertEqual(producer.symbol_reference.compressed_fixture(original, historical), compressed)
            with self.assertRaisesRegex(RuntimeError, '--output-dir'):
                producer.preserve_historical_producer(changed, historical, False)
            self.assertEqual(historical.read_bytes(), compressed)
            # Explicit producers can contain the new verbatim filename and do
            # not need a historical file at the selected destination.
            explicit = Path(directory) / 'new-producer' / 'symbol_panic.jsonl.gz'
            producer.preserve_historical_producer(changed, explicit, True)
            explicit.parent.mkdir()
            explicit.write_bytes(producer.symbol_reference.compressed_fixture(changed, explicit))
            self.assertEqual(gzip.decompress(explicit.read_bytes()), changed)
            self.assertEqual(historical.read_bytes(), compressed)

    def test_historical_crc_or_missing_data_never_refreshes_by_default(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'historical.gz'
            with self.assertRaisesRegex(RuntimeError, '--output-dir'):
                producer.preserve_historical_producer(b'raw', path, False)
            damaged = bytearray(gzip.compress(b'raw'))
            damaged[-8] ^= 1
            path.write_bytes(damaged)
            with self.assertRaisesRegex(RuntimeError, '--output-dir'):
                producer.preserve_historical_producer(b'raw', path, False)
            self.assertEqual(path.read_bytes(), bytes(damaged))

    def test_cli_persistence_rejects_changed_default_but_writes_explicit_bundle(self):
        # Mock only observation/verification here: this isolates the actual CLI
        # persistence edge without building a second complete compiler graph.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            old_manifest, old_fixture = root / 'old.json', root / 'old.gz'
            old_manifest.write_bytes(b'historical metadata')
            old_fixture.write_bytes(gzip.compress(b'historical raw\n', mtime=123))
            preserved = (old_manifest.read_bytes(), old_fixture.read_bytes())
            provenance = root / 'provenance.json'
            provenance.write_text(json.dumps({'registry_dependencies': []}))
            row = {'request': 'probe', 'want': {'abort': False, 'trace': ['verbatim /alternate/cache/source.rs']}}
            original_paths = producer.output_paths
            def paths(value):
                return (old_manifest, old_fixture, None) if value is None else original_paths(value)
            def profiles(configs):
                return {name: ({'provenance_path': str(provenance)}, {}, {}) for name, _, _ in configs}
            def observe(config, selected, cases, work):
                return [row], {'name': config[0]}, []
            with mock.patch.object(producer, 'output_paths', side_effect=paths), mock.patch.object(producer, 'load_profiles', side_effect=profiles), mock.patch.object(producer, 'profile', side_effect=observe), mock.patch.object(producer.native_profile, 'source_bytes', return_value=b'pinned source'), mock.patch.object(producer.resource, 'setrlimit'), redirect_stdout(io.StringIO()):
                with mock.patch.object(sys, 'argv', ['generate.py', '--prepared']):
                    with self.assertRaisesRegex(RuntimeError, '--output-dir'):
                        producer.main()
                self.assertEqual((old_manifest.read_bytes(), old_fixture.read_bytes()), preserved)
                explicit = root / 'explicit'
                with mock.patch.object(sys, 'argv', ['generate.py', '--prepared', '--output-dir', str(explicit)]):
                    producer.main()
                raw = (explicit / 'symbol_panic.jsonl').read_bytes()
                self.assertIn(b'verbatim /alternate/cache/source.rs', raw)
                self.assertEqual(gzip.decompress((explicit / 'symbol_panic.jsonl.gz').read_bytes()), raw)
                self.assertTrue((explicit / 'manifest.json').is_file())
                self.assertEqual((old_manifest.read_bytes(), old_fixture.read_bytes()), preserved)


if __name__ == '__main__':
    unittest.main()
