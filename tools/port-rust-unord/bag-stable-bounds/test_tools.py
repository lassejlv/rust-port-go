#!/usr/bin/env python3
"""Original Bag Hash/Eq/StableHash bound and unused-effect evidence integrity."""
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]


class StableBoundsProof(unittest.TestCase):
    def test_actual_source_bounds_and_unused_effects(self):
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual({row['configuration']: row['compiles'] for row in metadata['bounds']}, {
                'positive': True, 'no_hash': False, 'no_eq': False, 'no_stable_hash': False,
            })
            output = metadata['bounds'][0]['stdout']
            self.assertNotIn('UNEXPECTED', output)
            self.assertEqual(output.count('stable:'), 7)
            self.assertEqual(output.count('fingerprint:'), 4)
            for row in metadata['bounds'][1:]:
                self.assertIn('error[E0277]', row['diagnostic'])
                self.assertIn({'no_hash': 'Hash', 'no_eq': 'Eq', 'no_stable_hash': 'StableHash'}[row['configuration']], row['diagnostic'])

    def test_source_and_verified_loader_bindings(self):
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)


if __name__ == '__main__':
    unittest.main()
