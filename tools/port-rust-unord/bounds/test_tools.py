#!/usr/bin/env python3
"""Retained original full-bound admissions/effects and exact proof bindings."""
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]

class CompleteBoundsProof(unittest.TestCase):
    def test_original_admission_programs(self):
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual(len(metadata['bounds']), 18)
            self.assertEqual({row['configuration'] for row in metadata['bounds'] if row['compiles']}, {'positive', 'new_no_default'})
            for row in metadata['bounds']:
                if not row['compiles']:
                    self.assertIn('error[', row['diagnostic'])
                    self.assertIn({'no_default':'Default','clone_from_self':'mismatched types','clone_from_opaque':'Clone','clone_item':'Clone','clone_iterator':'Clone','cloned_value':'Clone','copied_value':'Copy','max_ord':'Ord','min_ord':'Ord','sum_trait':'Sum','product_trait':'Product','key_eq':'Eq','key_partial_ord':'PartialOrd','key_ord':'Ord','key_stable_hash':'StableHash','key_sized':'Sized'}[row['configuration']],row['diagnostic'])

    def test_actual_unused_effects_and_used_dispatch(self):
        metadata = json.loads((HERE / 'native_manifest.json').read_text())
        self.assertEqual(len(metadata['cases']), 22)
        for row in metadata['cases']:
            self.assertNotIn('UNEXPECTED', row['stdout'])
            if row['group'] == 'key':
                self.assertIn('project:', row['stdout'])
                selected = 'partial:' if row['cache'] or row['family'] == 1 else 'lt:'
                self.assertIn(selected, row['stdout'])
            elif row['family'] == 0:
                self.assertEqual(row['allocation'], ['ALLOC:0:1:32'])
                self.assertEqual(row['stdout'], 'empty:0\npush:1:capacity4\nitem:7\n')
            elif row['family'] == 1:
                self.assertEqual(row['stdout'], 'clone:1\ninput:101\nclone:1\ninput:2\ninput:101\ninput:1\n')

    def test_source_and_verified_loader_bindings(self):
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

if __name__ == '__main__':
    unittest.main()
