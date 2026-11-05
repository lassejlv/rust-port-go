#!/usr/bin/env python3
"""Read-only integrity gates for the pending UnordItems proofs."""
import gzip
import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]

def digest(data):
    return hashlib.sha256(data).hexdigest()

class ItemsProofIntegrity(unittest.TestCase):
    def test_retained_corpora_and_bindings(self):
        for name, fixture, count, aborts in [
            ('function', 'unord_function', 147, 74),
            ('core', 'unord_items_core', 126, 40),
            ('boolean', 'unord_items_boolean', 1224, 332),
            ('constructor', 'unord_items_constructor', 408, 180),
            ('collections', 'unord_items_collections', 2016, 374),
        ]:
            with self.subTest(name=name):
                metadata = json.loads((HERE / (name + '_manifest.json')).read_text())
                compressed = (ROOT / 'port/rust/compiler/rustc_data_structures/src/testdata' / (fixture + '.jsonl.gz')).read_bytes()
                raw = gzip.decompress(compressed)
                self.assertEqual(digest(compressed), metadata['fixture_sha256'])
                self.assertEqual(digest(raw), metadata['raw_sha256'])
                rows = [json.loads(line) for line in raw.splitlines()]
                self.assertEqual(len(rows), count)
                self.assertEqual(metadata['cases'], count)
                self.assertEqual(sum(row['outcome'] == 'abort' for row in rows), aborts)
                self.assertEqual(metadata['aborts'], aborts)
                for path, expected in metadata['files'].items():
                    self.assertEqual(digest((ROOT / path).read_bytes()), expected, path)
                for path, expected in metadata.get('reference_inputs', {}).items():
                    self.assertEqual(digest((ROOT / path).read_bytes()), expected, path)

    def test_original_clone_bounds_are_observed(self):
        metadata = json.loads((HERE / 'core_manifest.json').read_text())
        self.assertEqual([(row['configuration'], row['compiles']) for row in metadata['clone_bounds']], [
            ('positive', True), ('no_item_clone', False),
            ('opaque_map_clone', False), ('opaque_filter_clone', False),
        ])

    def test_source_verifier_rejects_changed_bytes_and_symlinks(self):
        spec = importlib.util.spec_from_file_location('index_context_tools', HERE.parent / 'index-context/generate.py')
        tools = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(tools)
        with tempfile.TemporaryDirectory() as directory:
            temporary = pathlib.Path(directory)
            original = temporary / 'compiler/test.rs'
            original.parent.mkdir()
            original.write_bytes(b'unchanged source')
            provenance = temporary / 'provenance.json'
            provenance.write_text(json.dumps({'source_files': [{'path': 'compiler/test.rs', 'sha256': digest(original.read_bytes())}]}))
            profile = {'provenance_path': str(provenance), 'provenance_sha256': digest(provenance.read_bytes()), 'original_root': str(temporary)}
            tools.verify_sources(profile)
            original.write_bytes(b'changed source')
            with self.assertRaises(ValueError):
                tools.verify_sources(profile)
            original.unlink()
            target = temporary / 'target.rs'
            target.write_bytes(b'unchanged source')
            original.symlink_to(target)
            with self.assertRaises(ValueError):
                tools.verify_sources(profile)

if __name__ == '__main__':
    unittest.main()
