import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).absolute().parent
sys.path.insert(0, str(HERE))
import producer


class ProducerTests(unittest.TestCase):
    def test_default_is_preserved_and_explicit_output_is_independent(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            manifest, raw = directory / 'history.json', directory / 'history.tsv'
            manifest.write_bytes(b'old metadata'); raw.write_bytes(b'verbatim /old/cache\n')
            paths = producer.paths
            def selected(kind, output):
                return (manifest, raw) if output is None else paths(kind, output)
            with mock.patch.object(producer, 'paths', side_effect=selected), mock.patch.object(producer, 'bindings', return_value={'source': 'hash'}):
                with self.assertRaisesRegex(RuntimeError, '--output-dir'):
                    producer.persist('borrow', b'verbatim /new/cache\n', {}, None, False)
                self.assertEqual(raw.read_bytes(), b'verbatim /old/cache\n')
                self.assertEqual(manifest.read_bytes(), b'old metadata')
                explicit = directory / 'producer'
                producer.persist('borrow', b'verbatim /new/cache\n', {}, explicit, False)
                producer.persist('borrow', b'verbatim /new/cache\n', {}, explicit, True)
                self.assertEqual((explicit / 'borrow_panic.tsv').read_bytes(), b'verbatim /new/cache\n')
                (explicit / 'borrow_panic.tsv').write_bytes(b'corrupt')
                with self.assertRaises(RuntimeError):
                    producer.persist('borrow', b'verbatim /new/cache\n', {}, explicit, True)

    def test_empty_symlink_and_oversized_inputs_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            with self.assertRaises(ValueError): producer.paths('borrow', '')
            link = directory / 'link'; link.symlink_to(directory, target_is_directory=True)
            with self.assertRaises(ValueError): producer.paths('registry', link)
            oversized = directory / 'large'
            with oversized.open('wb') as stream: stream.truncate(producer.LIMIT + 1)
            with self.assertRaises(ValueError): producer.read(oversized)

    def test_output_selection_never_bypasses_complete_reference_loader(self):
        for kind in ['borrow', 'registry']:
            spec = importlib.util.spec_from_file_location('tracing_' + kind, HERE / ('generate_' + kind + '.py'))
            module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
            with mock.patch.object(sys, 'argv', ['generator', '--check', '--output-dir', '/selected/producer']), mock.patch.object(module.reference, 'load', side_effect=RuntimeError('complete verifier')) as load:
                with self.assertRaisesRegex(RuntimeError, 'complete verifier'): module.main()
                load.assert_called_once_with(rustc=None)


if __name__ == '__main__': unittest.main()
