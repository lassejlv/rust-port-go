#!/usr/bin/env python3
"""Read-only provenance and retained native sorting proof integrity."""
import ast
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
GROUPS = [
    ('manifest', 'unord_items_sort', 864, 268),
    ('vector_manifest', 'unord_items_vector', 864, 160),
    ('zero_manifest', 'unord_items_sort_zero', 828, 164),
    ('collections_manifest', 'unord_collections_sort', 3072, 800),
    ('values_manifest', 'unord_values_sorted', 2112, 528),
]

def digest(data):
    return hashlib.sha256(data).hexdigest()

class SortProofIntegrity(unittest.TestCase):
    def test_retained_corpora_and_bindings(self):
        for name, fixture, count, aborts in GROUPS:
            with self.subTest(name=name):
                metadata = json.loads((HERE / (name + '.json')).read_text())
                compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata' / (fixture + '.jsonl.gz')).read_bytes()
                raw = gzip.decompress(compressed)
                self.assertEqual(digest(compressed), metadata['fixture_sha256'])
                self.assertEqual(digest(raw), metadata['raw_sha256'])
                rows = [json.loads(line) for line in raw.splitlines()]
                self.assertEqual((len(rows), metadata['cases']), (count, count))
                self.assertEqual((sum(row['outcome'] == 'abort' for row in rows), metadata['aborts']), (aborts, aborts))
                for row in rows:
                    self.assertEqual(row['native_exit_code'], -6 if row['outcome'] == 'abort' else 0)
                for table in ['files', 'reference_inputs']:
                    self.assertTrue(metadata[table])
                    for path, expected in metadata[table].items():
                        self.assertEqual(digest((ROOT / path).read_bytes()), expected, path)

    def test_generators_use_complete_verified_loader(self):
        for name in ['generate.py', 'generate_vector.py', 'generate_zero.py', 'generate_collections.py', 'generate_values.py']:
            with self.subTest(name=name):
                tree = ast.parse((HERE / name).read_text())
                self.assertTrue(any(isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute) and isinstance(node.func.value, ast.Name) and node.func.value.id == 'REFERENCE' and node.func.attr == 'load_profile' for node in ast.walk(tree)))

    def test_values_allocation_evidence_is_retained_separately(self):
        compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata/unord_values_sorted.jsonl.gz').read_bytes()
        rows = [json.loads(line) for line in gzip.decompress(compressed).splitlines()]
        reused = borrowed = empty = partial = caught = 0
        for row in rows:
            native = row['native_stdout'].splitlines()
            self.assertEqual(row['source_allocation'], [line for line in native if line.startswith('ALLOC:')])
            self.assertEqual(row['stdout'], ''.join(line + '\n' for line in native if not line.startswith('ALLOC:')))
            request = row['request']
            if row['outcome'] != 'completed' or 'caught::' not in row['stdout']:
                continue
            if request['action'] == 0 and request['length'] and 'reuse:true\n' in row['stdout']:
                reused += 1
                partial += request['skip'] != 0
            if request['action'] == 1 and request['length'] and 'reuse:false\n' in row['stdout']:
                borrowed += 1
            empty += request['length'] == 0
            caught += request['action'] == 3 and 'inner_caught:636f6e73756d65:' in row['stdout'] and 'reuse:true\n' in row['stdout']
        self.assertTrue(all(n > 0 for n in [reused, borrowed, empty, partial, caught]))

if __name__ == '__main__':
    unittest.main()
