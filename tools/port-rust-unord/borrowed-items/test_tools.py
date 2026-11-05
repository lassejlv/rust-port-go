#!/usr/bin/env python3
"""Integrity of the retained original borrowed-adapter evidence."""
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]


class BorrowedItemsProof(unittest.TestCase):
    def test_native_corpus_and_input_bindings(self):
        fixture = ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_borrowed.jsonl.gz'
        compressed = fixture.read_bytes()
        raw = gzip.decompress(compressed)
        rows = [json.loads(row) for row in raw.splitlines()]
        for name in ['manifest.json', 'native_manifest.json']:
            metadata = json.loads((HERE / name).read_text())
            self.assertEqual(hashlib.sha256(compressed).hexdigest(), metadata['fixture_sha256'])
            self.assertEqual(hashlib.sha256(raw).hexdigest(), metadata['raw_sha256'])
            self.assertEqual((len(rows), metadata['cases']), (720, 720))
            self.assertEqual((sum(row['outcome'] == 'abort' for row in rows), metadata['aborts']), (74, 74))
            self.assertEqual({row['family'] for row in rows}, set(range(10)))
            for table in ['files', 'reference_inputs']:
                self.assertTrue(metadata[table])
                for path, expected in metadata[table].items():
                    self.assertEqual(hashlib.sha256((ROOT / path).read_bytes()).hexdigest(), expected, path)

    def test_distinct_actual_count_dispatch(self):
        fixture = ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_borrowed.jsonl.gz'
        rows = [json.loads(row) for row in gzip.decompress(fixture.read_bytes()).splitlines()]
        observed = set()
        for row in rows:
            output = row['stdout']
            self.assertNotIn('UNEXPECTED copy_clone', output)
            self.assertNotIn('UNEXPECTED next'.encode().hex(), output)
            if row['mode'] != 'normal' or row['family'] not in [8, 9]:
                continue
            request = row['request']
            if request['clone_fail'] != -1 or request['drop_fail'] != -1:
                continue
            if row['family'] == 8:
                self.assertIn('fold_override\n', output)
                self.assertNotIn('count_override\n', output)
                self.assertEqual(output.count('clone:99:false\n'), 2)
                self.assertEqual(output.count('item:1099:false\n'), 2)
                self.assertIn('returned:2\n', output)
            else:
                self.assertIn('count_override\n', output)
                self.assertNotIn('fold_override\n', output)
                self.assertIn('returned:777\n', output)
            observed.add(row['family'])
        self.assertEqual(observed, {8, 9})

    def test_actual_bounds_and_opaque_return_types(self):
        metadata = json.loads((HERE / 'native_manifest.json').read_text())
        self.assertEqual({row['configuration']: row['compiles'] for row in metadata['bounds']}, {
            'positive': True, 'no_clone': False, 'no_copy': False,
            'opaque_cloned': False, 'opaque_copied': False,
        })
        for row in metadata['bounds']:
            if not row['compiles']:
                self.assertIn('error[', row['diagnostic'])


if __name__ == '__main__':
    unittest.main()
