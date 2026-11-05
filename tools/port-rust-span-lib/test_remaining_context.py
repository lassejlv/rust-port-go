"""Fresh producer selection and historical-byte preservation; no builds."""
import gzip
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('remaining_context_producer',HERE/'generate_remaining_context.py')
producer=importlib.util.module_from_spec(spec);spec.loader.exec_module(producer)

class ProducerSelectionTests(unittest.TestCase):
    def test_explicit_output_is_separate_and_empty_file_or_symlink_fails(self):
        historical=producer.evidence_paths()
        with tempfile.TemporaryDirectory()as directory:
            root=Path(directory);fresh=root/'fresh'
            selected=producer.evidence_paths(str(fresh))
            self.assertEqual(selected,(fresh/'remaining-context-manifest.json',fresh/'lib-remaining-context.jsonl.gz',fresh/'lib-remaining-context.jsonl'))
            self.assertNotEqual(selected[:2],historical[:2]);self.assertFalse(fresh.exists())
            file=root/'file';file.write_bytes(b'unowned')
            link=root/'link';link.symlink_to(root,target_is_directory=True)
            for value in ['',str(file),str(link),str(producer.HERE),str(historical[1].parent)]:
                with self.subTest(value=value),self.assertRaises(RuntimeError):producer.evidence_paths(value)
            fresh.mkdir();(fresh/'remaining-context-manifest.json').symlink_to(file)
            with self.assertRaisesRegex(RuntimeError,'regular owned file'):producer.evidence_paths(str(fresh))
            self.assertEqual(file.read_bytes(),b'unowned')
    def test_invalid_output_rejects_before_reference_selection(self):
        with mock.patch.object(sys,'argv',[str(producer.__file__),'--output-dir','']),mock.patch.object(producer.reference,'load_profile',side_effect=AssertionError('must reject before any verification/compiler')):
            with self.assertRaisesRegex(RuntimeError,'explicitly empty'):producer.main()
    def test_historical_observation_or_filename_drift_never_overwrites(self):
        with tempfile.TemporaryDirectory()as directory:
            path=Path(directory)/'history.gz';old=gzip.compress(b'original absolute producer filename',mtime=123)
            path.write_bytes(old)
            self.assertEqual(producer.producer_fixture(b'original absolute producer filename',path,True),old)
            with self.assertRaisesRegex(RuntimeError,'use --output-dir'):producer.producer_fixture(b'new absolute producer filename',path,True)
            self.assertEqual(path.read_bytes(),old)
            broken=bytearray(old);broken[-8]^=1;path.write_bytes(broken)
            with self.assertRaisesRegex(RuntimeError,'use --output-dir'):producer.producer_fixture(b'original absolute producer filename',path,True)
            self.assertEqual(path.read_bytes(),broken)
            path.unlink()
            with self.assertRaisesRegex(RuntimeError,'use --output-dir'):producer.producer_fixture(b'new',path,True)
            self.assertFalse(path.exists())
    def test_fresh_producer_preserves_equal_gzip_and_rebuilds_changed_data(self):
        with tempfile.TemporaryDirectory()as directory:
            path=Path(directory)/'fresh.gz';old=gzip.compress(b'old',mtime=123);path.write_bytes(old)
            self.assertEqual(producer.producer_fixture(b'old',path,False),old)
            self.assertEqual(gzip.decompress(producer.producer_fixture(b'new',path,False)),b'new')
            self.assertEqual(path.read_bytes(),old)

if __name__=='__main__':unittest.main()
