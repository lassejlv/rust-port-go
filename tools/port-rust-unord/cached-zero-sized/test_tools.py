#!/usr/bin/env python3
import gzip
import hashlib
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]

def sha(data):
    return hashlib.sha256(data).hexdigest()

class ZeroSizedProof(unittest.TestCase):
    def test_native_records_and_bindings(self):
        compressed = (ROOT / 'port/support/rustsort/testdata/cached_zero_sized.jsonl.gz').read_bytes()
        raw = gzip.decompress(compressed)
        rows = [json.loads(line) for line in raw.splitlines()]
        self.assertEqual(len(rows), 624)
        self.assertEqual(sum(row['outcome'] == 'abort' for row in rows), 208)
        for name in ['native_manifest.json', 'manifest.json']:
            manifest = json.loads((HERE / name).read_text())
            self.assertEqual(manifest['raw_sha256'], sha(raw))
            self.assertEqual(manifest['fixture_sha256'], sha(compressed))
            self.assertEqual((manifest['cases'], manifest['aborts']), (624, 208))
            for path, expected in manifest['files'].items():
                self.assertEqual(sha((ROOT / path).read_bytes()), expected, path)
        self.assertTrue(all(not path.endswith('.go') for path in json.loads((HERE / 'native_manifest.json').read_text())['files']))

    def test_actual_allocation_and_full_uint_preflight(self):
        rows = [json.loads(line) for line in gzip.decompress((ROOT / 'port/support/rustsort/testdata/cached_zero_sized.jsonl.gz').read_bytes()).splitlines()]
        normal = [row for row in rows if row['context'] == 'normal' and not row['resume'] and not row['request']['chain']]
        observed = {}
        for row in normal:
            request = row['request']
            if request['length'] == -1:
                self.assertFalse(any(line.startswith('F:') for line in row['stdout'].splitlines()))
                self.assertIn('6361706163697479206f766572666c6f77', row['stdout'])
            if request['length'] == 4 and request['key_fail'] == request['cmp_fail'] == request['drop_fail'] == -1 and not any(request[name] for name in ['fn_fail', 'token_fail', 'moved']):
                observed[request['kind']] = [line for line in row['stdout'].splitlines() if line.startswith('ALLOC:')]
        self.assertEqual(observed, {0: ['ALLOC:32:4'], 1: ['ALLOC:96:8'], 3: ['ALLOC:16:4']})

if __name__ == '__main__':
    unittest.main()
