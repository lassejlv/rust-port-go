#!/usr/bin/env python3
"""Retained actual-native collection conversion proof integrity."""
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]

class ConversionProofIntegrity(unittest.TestCase):
    def test_retained_native_corpus_and_bindings(self):
        metadata = json.loads((HERE / 'conversion_manifest.json').read_text())
        compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_conversion.jsonl.gz').read_bytes()
        raw = gzip.decompress(compressed)
        self.assertEqual(hashlib.sha256(compressed).hexdigest(), metadata['fixture_sha256'])
        self.assertEqual(hashlib.sha256(raw).hexdigest(), metadata['raw_sha256'])
        rows = [json.loads(line) for line in raw.splitlines()]
        self.assertEqual((len(rows), metadata['cases']), (1296, 1296))
        self.assertEqual((sum(row['outcome'] == 'abort' for row in rows), metadata['aborts']), (302, 302))
        self.assertEqual({row['family'] for row in rows}, set(range(9)))
        for table in ['files', 'reference_inputs']:
            self.assertTrue(metadata[table])
            for path, expected in metadata[table].items():
                self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

    def test_conversion_bypasses_collect_and_preserves_selected_consumers(self):
        raw = gzip.decompress((ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_conversion.jsonl.gz').read_bytes())
        rows = [json.loads(line) for line in raw.splitlines()]
        map_each = set_fold = fx_transfer = owned_vec = 0
        for row in rows:
            self.assertNotIn('UNEXPECTED collect'.encode().hex(), row['stdout'])
            map_each += row['family'] in [0, 3] and 'for_each\n' in row['stdout']
            set_fold += row['family'] in [1, 4] and 'fold\n' in row['stdout']
            if row['family'] == 6:
                self.assertNotIn('hash:', row['stdout'])
                fx_transfer += row['request']['length'] > 0 and 'returned:' in row['stdout']
            if row['family'] in [7, 8]:
                self.assertNotIn('next:', row['stdout'])
                self.assertNotIn('hint:', row['stdout'])
                owned_vec += row['request']['length'] > 0 and 'returned:' in row['stdout']
        self.assertTrue(all(n > 0 for n in [map_each, set_fold, fx_transfer, owned_vec]))

    def test_nested_destructor_calls_use_current_context_and_resume(self):
        raw = gzip.decompress((ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_conversion.jsonl.gz').read_bytes())
        rows = [json.loads(line) for line in raw.splitlines()]
        observed = set()
        for row in rows:
            request = row['request']
            if row['mode'] != 'normal' or row['family'] not in [0, 1, 3, 4] or not request['nested'] or request['length'] != 4:
                continue
            if any(request[name] != -1 for name in ['next_fail', 'item_fail', 'hash_fail']) or request['call_fail'] or request['input_fail']:
                continue
            self.assertIn('nested_consume:true\n', row['stdout'])
            self.assertIn('hash:100:true\n', row['stdout'])
            self.assertIn('hash:101:false\n', row['stdout'])
            self.assertIn('caught::false\n', row['stdout'])
            observed.add(row['family'])
        self.assertEqual(observed, {0, 1, 3, 4})

if __name__ == '__main__':
    unittest.main()
