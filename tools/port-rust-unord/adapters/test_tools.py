#!/usr/bin/env python3
"""Integrity of retained actual source adapter traces and dispatch bounds."""
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
FIXTURE = ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_adapter.jsonl.gz'

def rows():
    return [json.loads(line) for line in gzip.decompress(FIXTURE.read_bytes()).splitlines()]

class AdapterProof(unittest.TestCase):
    def test_native_corpus_and_verified_bindings(self):
        compressed = FIXTURE.read_bytes()
        raw = gzip.decompress(compressed)
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual(hashlib.sha256(compressed).hexdigest(), metadata['fixture_sha256'])
            self.assertEqual(hashlib.sha256(raw).hexdigest(), metadata['raw_sha256'])
            self.assertEqual((len(rows()), metadata['cases']), (1650, 1650))
            self.assertEqual((sum(row['outcome'] == 'abort' for row in rows()), metadata['aborts']), (442, 442))
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

    def test_independent_actual_methods(self):
        found = set()
        for row in rows():
            request = row['request']
            if row['mode'] != 'normal' or request['resume'] or request['length'] != 4 or request['inner'] != 2 or request['keep'] != 0:
                continue
            if any(request[k] != -1 for k in ['next_fail', 'call_fail', 'item_fail']) or any(request[k] for k in ['input_fail', 'env_fail', 'token_fail']):
                continue
            family, output = row['family'], row['stdout']
            if family in [0, 1, 2, 3]:
                self.assertIn('mut:', output)
                self.assertNotIn('shared:', output)
                self.assertNotIn('once:', output)
                if family % 2:
                    self.assertIn('fold:0:false', output)
                    self.assertLess(output.index('function:'), output.index('input:0:'))
                else:
                    self.assertLess(output.index('input:0:'), output.index('function:'))
            if family in [4, 5]:
                self.assertIn('shared:', output)
                self.assertNotIn('mut:', output)
                if family == 5:
                    self.assertIn('fold:0:false', output)
                    self.assertIn('count:100:false', output)
                    self.assertNotIn('next:100:', output)
            if family == 7:
                self.assertIn('count:0:false', output)
                self.assertIn('count:1:false', output)
                self.assertNotIn('next:', output)
            if family == 8:
                self.assertIn('filter_override', output)
                self.assertIn('once:0:99:false', output)
            if family == 9:
                self.assertIn('filter_map_override', output)
                self.assertIn('once:0:99:false', output)
            if family == 10:
                self.assertIn('chain_override', output)
                self.assertIn('input:1:0:false', output)
            found.add(family)
        self.assertEqual(found, set(range(11)))

    def test_original_type_bounds(self):
        bounds = json.loads((HERE / 'native_manifest.json').read_text())['bounds']
        self.assertEqual({r['configuration']: r['compiles'] for r in bounds}, {
            'positive': True, 'filter_not_fn': False, 'filter_map_not_fn': False,
            'flat_map_not_fn': False, 'chain_mismatch': False,
            'opaque_filter': False, 'opaque_filter_map': False,
            'opaque_flat_map': False, 'opaque_chain': False,
        })
        for row in bounds:
            if not row['compiles']:
                self.assertIn('error[', row['diagnostic'])

if __name__ == '__main__':
    unittest.main()
