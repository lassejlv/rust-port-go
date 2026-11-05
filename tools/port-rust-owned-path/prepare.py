#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Verify scoped owning path SDK sources and the paired official compiler."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent

def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

BASE = load('owned_path_verified_sdk', ROOT/'tools/port-rust-str/prepare.py')

def verified():
    original, cache = BASE.verified()
    expected = json.loads((HERE/'SOURCES.json').read_text())
    for key in ['compiler_commit', 'sha256', 'manifest_sha256', 'licenses']:
        if expected[key] != original[key]:
            raise RuntimeError('owning path SDK selection differs: '+key)
    with tarfile.open(cache/'rust-src-nightly.tar.xz', 'r:xz') as tar:
        for name, proof in expected['sources'].items():
            member = tar.getmember(proof['member'])
            if not member.isfile() or member.size > 2 << 20:
                raise RuntimeError('invalid SDK source member: '+name)
            data = tar.extractfile(member).read()
            if len(data) != proof['bytes'] or hashlib.sha256(data).hexdigest() != proof['sha256']:
                raise RuntimeError('owning path source differs: '+name)
    compiler, version = BASE.selected()
    return compiler, version, expected

if __name__ == '__main__':
    _, version, proof = verified()
    print(version.splitlines()[0], len(proof['sources']), 'exact scoped SDK source modules verified')
