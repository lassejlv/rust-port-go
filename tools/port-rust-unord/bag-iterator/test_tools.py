#!/usr/bin/env python3
"""Retained original owned-S Bag Extend proof integrity."""
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def rows():
    return [json.loads(line) for line in gzip.decompress((ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_bag_iterator.jsonl.gz').read_bytes()).splitlines()]


class BagIteratorProof(unittest.TestCase):
    def test_native_corpus_and_bindings(self):
        compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_bag_iterator.jsonl.gz').read_bytes()
        raw = gzip.decompress(compressed)
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual(hashlib.sha256(compressed).hexdigest(), metadata['fixture_sha256'])
            self.assertEqual(hashlib.sha256(raw).hexdigest(), metadata['raw_sha256'])
            self.assertEqual((len(rows()), metadata['cases']), (768, 768))
            self.assertEqual((sum(r['outcome'] == 'abort' for r in rows()), metadata['aborts']), (184, 184))
            self.assertEqual({r['family'] for r in rows()}, {0, 1})
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

    def test_owned_self_forget_and_specialization(self):
        observed = set()
        for row in rows():
            request = row['request']
            if row['mode'] != 'normal' or request['resume'] or request['length'] != 4:
                continue
            if request['into_fail'] or request['input_fail'] or request['item_fail'] != -1 or request['next_fail'] != -1:
                continue
            output = row['stdout']
            if request['forget']:
                self.assertNotIn('producer_drop:', output)
                self.assertIn('returned:', output)
                observed.add(('forget', row['family']))
            if not request['producer_fail']:
                if row['family'] == 0:
                    self.assertIn('next:', output)
                    self.assertIn('hint:', output)
                    self.assertIn('input_drop:', output)
                else:
                    self.assertNotIn('next:', output)
                    self.assertNotIn('hint:', output)
                    self.assertNotIn('input_drop:', output)
                observed.add(('strategy', row['family']))
        self.assertEqual(observed, {(kind, family) for kind in ['forget', 'strategy'] for family in [0, 1]})

    def test_prepared_input_leak_and_early_conversion(self):
        observed = set()
        for row in rows():
            request = row['request']
            if row['mode'] != 'normal' or request['resume'] or request['forget'] or request['length'] != 4:
                continue
            if request['input_fail'] or request['item_fail'] != -1 or request['next_fail'] != -1:
                continue
            output = row['stdout']
            if request['producer_fail'] and not request['into_fail']:
                self.assertIn('producer_drop:false:false\n', output)
                self.assertNotIn('item:100:', output)
                self.assertNotIn('input_drop:', output)
                observed.add(('prepared', row['family']))
            if request['into_fail'] and not request['producer_fail']:
                self.assertIn('producer_drop:true:true\n', output)
                self.assertIn('item:100:true\n', output)
                observed.add(('early', row['family']))
        self.assertEqual(observed, {(kind, family) for kind in ['prepared', 'early'] for family in [0, 1]})

    def test_native_allocation_capacity_and_actual_iterator_dispatch(self):
        metadata = json.loads((HERE / 'native_manifest.json').read_text())
        allocations = metadata['allocations']
        self.assertEqual(len(allocations), 20)
        for row in allocations:
            self.assertEqual((row['rust_size'], row['rust_align']), (16, 8))
            self.assertFalse(row['reused'])
            if row['family'] == 1:
                self.assertEqual((row['next'], row['hint']), (0, 0))
            else:
                self.assertEqual(row['next'], row['length'] + 1)
            if row['length'] == 21:
                self.assertEqual(row['capacity'], 22 if row['initial'] else 21)
                self.assertEqual(row['maximum_size'], row['capacity'] * 16)
                self.assertEqual(row['allocations'], 1)


if __name__ == '__main__':
    unittest.main()
