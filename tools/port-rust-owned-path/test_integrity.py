#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Retained native producer shape, checksums and exact source provenance."""
import gzip
import hashlib
import json
from pathlib import Path
import unittest
import generate
import prepare

class Integrity(unittest.TestCase):
    def test_native_artifact_and_requests(self):
        here=Path(__file__).resolve().parent
        manifest=json.loads((here/'native-manifest.json').read_text())
        compressed=(here/'native.jsonl.gz').read_bytes();raw=gzip.decompress(compressed)
        self.assertEqual(hashlib.sha256(compressed).hexdigest(),manifest['gzip_sha256'])
        self.assertEqual(hashlib.sha256(raw).hexdigest(),manifest['raw_sha256'])
        self.assertEqual(hashlib.sha256((here/'native.rs').read_bytes()).hexdigest(),manifest['native_wrapper_sha256'])
        self.assertEqual(hashlib.sha256((here/'SOURCES.json').read_bytes()).hexdigest(),manifest['sources_sha256'])
        rows=[json.loads(line) for line in raw.splitlines()]
        requests=[]
        for windows in [False,True]:
            for request in generate.requests(windows):
                request['windows']=windows;requests.append(request)
        self.assertEqual(len(rows),manifest['records'])
        self.assertEqual(sum(len(r['ops']) for r in rows),manifest['operations'])
        self.assertEqual([{k:v for k,v in row.items() if k!='want'} for row in rows],requests)
        for row in rows:
            self.assertEqual(len(row['want'].split('|')),len(row['ops'])+1)
        self.assertIn('P:static:6361706163697479206f766572666c6f77',raw.decode())
        self.assertIn('P:owned:',raw.decode())

    def test_official_sdk_sources_and_compiler(self):
        _,version,proof=prepare.verified()
        self.assertIn('commit-hash: '+proof['compiler_commit'],version)
        self.assertEqual(len(proof['sources']),10)

    def test_accepted_go_scope_and_frozen_string_overlay(self):
        here=Path(__file__).resolve().parent
        proof=json.loads((here/'GO-SOURCES.json').read_text())
        self.assertEqual(proof['base_revision'],'2b747ed465d99b22b05992b28800918a211c59cb')
        self.assertEqual(len(proof['go_sha256']),6)
        self.assertEqual(proof['frozen_string_overlay_sha256'],json.loads((here/'STRING-OVERLAY.json').read_text()))
        for name,digest in {**proof['go_sha256'],**proof['frozen_string_overlay_sha256']}.items():
            self.assertEqual(hashlib.sha256((prepare.ROOT/name).read_bytes()).hexdigest(),digest,name)
        self.assertEqual(hashlib.sha256((here/'native-manifest.json').read_bytes()).hexdigest(),proof['native_manifest_sha256'])
        self.assertEqual(hashlib.sha256((here/'generate.py').read_bytes()).hexdigest(),proof['producer_sha256'])

if __name__=='__main__':unittest.main()
