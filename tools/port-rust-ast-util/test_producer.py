import gzip
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import generate
import reference


class ProducerIntegrity(unittest.TestCase):
    def test_historical_raw_cannot_be_replaced_implicitly(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / 'fixture.gz'
            original = gzip.compress(b'actual original\n', mtime=123)
            target.write_bytes(original)
            self.assertEqual(generate.compressed(b'actual original\n', target, False), original)
            with self.assertRaisesRegex(ValueError, '--output-dir'):
                generate.compressed(b'other valid producer path\n', target, False)
            self.assertEqual(target.read_bytes(), original)
            replacement = generate.compressed(b'other valid producer path\n', target, True)
            self.assertEqual(gzip.decompress(replacement), b'other valid producer path\n')
            self.assertEqual(target.read_bytes(), original)

    def test_crc_and_symbolic_link_are_not_trusted(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / 'fixture.gz'
            data = bytearray(gzip.compress(b'original', mtime=0)); data[-8] ^= 1
            target.write_bytes(data)
            with self.assertRaisesRegex(ValueError, 'historical'):
                generate.compressed(b'original', target, False)
            target.unlink()
            other = Path(directory) / 'other'; other.write_bytes(gzip.compress(b'original'))
            target.symlink_to(other)
            with self.assertRaisesRegex(ValueError, 'symlink'):
                generate.compressed(b'original', target, True)

    def test_raw_bound_is_enforced_before_compression(self):
        with mock.patch.object(generate, 'MAX_RAW', 4):
            with self.assertRaisesRegex(ValueError, 'bound'):
                generate.compressed(b'12345', Path('unused'), True)

    def test_seeded_inputs_are_reproducible_and_include_original_literals(self):
        first = generate.requests()
        self.assertEqual(first, generate.requests())
        self.assertGreater(len(first.splitlines()), 25000)
        self.assertIn(b'B\t1\t' + b'\n * Test\n *  Test\n'.hex().encode(), first)
        self.assertIn(b'B\t1\t' + b'\n let a: *i32;\n *a = 5;\n'.hex().encode(), first)
        self.assertIn(b'C\t3\t4294967295', first)

    def test_cold_reference_always_invokes_complete_preparation(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(reference, 'ROOT', Path(directory)), mock.patch.object(reference.subprocess, 'run') as run:
            # There is intentionally no cached profile. Complete preparation
            # is still called, and its absent result fails closed afterwards.
            with self.assertRaisesRegex(ValueError, 'metadata'):
                reference.load(False, True)
            run.assert_called_once()
            arguments = run.call_args.args[0]
            self.assertIn('--debug-assertions', arguments)
            self.assertEqual(arguments[arguments.index('--debug-assertions') + 1], 'false')
            self.assertEqual(arguments[arguments.index('--overflow-checks') + 1], 'true')
            self.assertNotIn('--prepared', arguments)


if __name__ == '__main__':
    unittest.main()
