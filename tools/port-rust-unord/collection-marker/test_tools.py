#!/usr/bin/env python3
"""Retained original private marker trait evidence integrity."""
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]


class MarkerProof(unittest.TestCase):
    def test_actual_compiler_bounds_and_dispatch(self):
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual({row['configuration']: row['compiles'] for row in metadata['bounds']}, {
                'positive': True, 'aliases': True, 'ordered': False,
                'wrapper': False, 'deref_wrapper': False,
            })
            self.assertEqual(metadata['bounds'][0]['stdout'], 'next\ndrop\n' * 3)
            for row in metadata['bounds']:
                if not row['compiles']:
                    self.assertIn('UnordCollection', row['diagnostic'])
                    self.assertIn('error[E0277]', row['diagnostic'])

    def test_source_and_verified_loader_bindings(self):
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)


if __name__ == '__main__':
    unittest.main()
