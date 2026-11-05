#!/usr/bin/env python3
"""Retained original ordinary collection trait evidence integrity."""
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def corpus():
    return [json.loads(line) for line in gzip.decompress((ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_collections_iterator.jsonl.gz').read_bytes()).splitlines()]


class OrdinaryCollectionProof(unittest.TestCase):
    def test_native_corpus_and_bindings(self):
        compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_collections_iterator.jsonl.gz').read_bytes()
        raw = gzip.decompress(compressed)
        rows = corpus()
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual(hashlib.sha256(compressed).hexdigest(), metadata['fixture_sha256'])
            self.assertEqual(hashlib.sha256(raw).hexdigest(), metadata['raw_sha256'])
            self.assertEqual((len(rows), metadata['cases']), (720, 720))
            self.assertEqual((sum(r['outcome'] == 'abort' for r in rows), metadata['aborts']), (296, 296))
            self.assertEqual({r['family'] for r in rows}, set(range(4)))
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

    def test_conversion_self_and_prepared_iterator_boundaries(self):
        observed = set()
        for row in corpus():
            request = row['request']
            if row['mode'] != 'normal' or request['resume'] or request['nested'] or request['length'] != 4:
                continue
            if any(request[k] != -1 for k in ['next_fail', 'item_fail', 'hash_fail']) or request['input_fail'] or request['call_fail']:
                continue
            output = row['stdout']
            if request['producer_fail'] and not request['into_fail']:
                self.assertIn('into_iter:false\nproducer_drop:false:false\n', output)
                self.assertNotIn('input_drop:', output)
                self.assertNotIn('item:100:', output)
                self.assertNotIn('hint:', output)
                observed.add(('prepared', row['family']))
            if request['into_fail'] and not request['producer_fail']:
                self.assertIn('producer_drop:true:true\ninput_drop:0:4:true\n', output)
                self.assertIn('item:100:true\n', output)
                self.assertNotIn('hint:', output)
                observed.add(('early', row['family']))
        self.assertEqual(observed, {(kind, family) for kind in ['prepared', 'early'] for family in range(4)})

    def test_nested_current_consumer_and_independent_iterator_methods(self):
        observed = set()
        for row in corpus():
            output = row['stdout']
            self.assertNotIn('UNEXPECTED collect'.encode().hex(), output)
            request = row['request']
            if row['mode'] != 'normal' or not request['nested'] or request['length'] != 4 or request['resume']:
                continue
            if any(request[k] != -1 for k in ['next_fail', 'item_fail', 'hash_fail']) or any(request[k] for k in ['input_fail', 'call_fail', 'producer_fail', 'into_fail']):
                continue
            self.assertIn('nested_consume:true\n', output)
            self.assertIn('hash:100:true\n', output)
            self.assertIn('hash:101:false\n', output)
            self.assertIn('for_each\n' if row['family'] in [0, 2] else 'fold\n', output)
            self.assertIn('caught::false\n', output)
            observed.add(row['family'])
        self.assertEqual(observed, set(range(4)))


if __name__ == '__main__':
    unittest.main()
