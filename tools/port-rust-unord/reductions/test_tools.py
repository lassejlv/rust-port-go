#!/usr/bin/env python3
"""Integrity of actual original reduction traces and trait dispatch."""
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def rows():
    return [json.loads(line) for line in gzip.decompress((ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_reduction.jsonl.gz').read_bytes()).splitlines()]


class ReductionProof(unittest.TestCase):
    def test_retained_corpus_and_source_bindings(self):
        compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_reduction.jsonl.gz').read_bytes()
        raw = gzip.decompress(compressed)
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual(hashlib.sha256(compressed).hexdigest(), metadata['fixture_sha256'])
            self.assertEqual(hashlib.sha256(raw).hexdigest(), metadata['raw_sha256'])
            self.assertEqual((len(rows()), metadata['cases']), (2112, 2112))
            self.assertEqual((sum(row['outcome'] == 'abort' for row in rows()), metadata['aborts']), (234, 234))
            self.assertEqual({row['family'] for row in rows()}, set(range(16)))
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

    def test_actual_independent_method_selection(self):
        observed = set()
        for row in rows():
            output = row['stdout']
            self.assertNotIn('UNEXPECTED'.encode().hex(), output)
            request = row['request']
            if row['mode'] != 'normal' or request['length'] != 4 or request['resume']:
                continue
            if any(request[k] != -1 for k in ['next_fail', 'compare_fail', 'item_fail', 'total_fail']) or request['input_fail'] or request['trait_fail']:
                continue
            family = row['family']
            if family <= 5 or family >= 14:
                self.assertIn('ord_max\n' if family % 2 == 0 else 'ord_min\n', output)
            if family in [4, 5]:
                self.assertIn('reduce_override\n', output)
                self.assertNotIn('next:', output)
            if family in [6, 7, 10, 11]:
                self.assertNotIn('next:', output)
                self.assertIn(['max', 'min', 'sum', 'product'][[6, 7, 10, 11].index(family)] + '_override\n', output)
            if family >= 12:
                self.assertIn('fold_override\n', output)
                if family <= 13:
                    self.assertNotIn('next:', output)
                else:
                    self.assertEqual(output.count('next:'), 1)
            if 8 <= family <= 13:
                self.assertIn('sum_trait\n' if family % 2 == 0 else 'product_trait\n', output)
            observed.add(family)
        self.assertEqual(observed, set(range(16)))

    def test_actual_bounds(self):
        bounds = json.loads((HERE / 'native_manifest.json').read_text())['bounds']
        self.assertEqual({r['configuration']: r['compiles'] for r in bounds}, {
            'positive': True, 'no_ord': False, 'no_sum': False, 'no_product': False,
        })
        for row in bounds:
            if not row['compiles']:
                self.assertIn('error[', row['diagnostic'])


if __name__ == '__main__':
    unittest.main()
