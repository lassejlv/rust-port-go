#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Isolate accepted main plus exactly the frozen String10 and new path helpers.

This reads Git objects; it creates no checkout, branch, index or Git mutation.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile

ROOT=Path(__file__).resolve().parents[2]
HERE=Path(__file__).resolve().parent
OWN_GO=[
    'port/support/ruststr/owned_string_buffer_bridge.go',
    'port/rust/compiler/rustc_span/src/source_map_owned_path.go',
    'port/rust/compiler/rustc_span/src/source_map_owned_path_buf.go',
    'port/rust/compiler/rustc_span/src/source_map_owned_path_cow.go',
    'port/rust/compiler/rustc_span/src/source_map_owned_path_oracle_test.go',
    'port/rust/compiler/rustc_span/src/source_map_owned_path_ownership_test.go',
]

def snapshot(base='HEAD'):
    revision=subprocess.run(['git','rev-parse',base],cwd=ROOT,stdout=subprocess.PIPE,check=True).stdout.decode().strip()
    archive=subprocess.run(['git','archive',revision],cwd=ROOT,stdout=subprocess.PIPE,check=True).stdout
    destination=Path(tempfile.mkdtemp(prefix='ferrugo-owned-path-scope-'))
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        tar.extractall(destination,filter='data')
    string=json.loads((HERE/'STRING-OVERLAY.json').read_text())
    files=OWN_GO+[str(path.relative_to(ROOT)) for path in HERE.rglob('*') if path.is_file() and '__pycache__' not in path.parts]
    selected={}
    for name in list(string)+files:
        source=ROOT/name;data=source.read_bytes();digest=hashlib.sha256(data).hexdigest()
        if name in string and digest!=string[name]:raise RuntimeError('frozen String10 differs: '+name)
        output=destination/name;output.parent.mkdir(parents=True,exist_ok=True);output.write_bytes(data)
        selected[name]=digest
    (destination/'.toolchain').symlink_to(ROOT/'.toolchain',target_is_directory=True)
    binding={'base_revision':revision,'scope':'accepted main plus exact frozen String10 and NEW owning path files only; live Syntax/Unord/Packed/Symbol WIP excluded','overlay_sha256':selected}
    (destination/'owned-path-snapshot.json').write_text(json.dumps(binding,indent=2,sort_keys=True)+'\n')
    return destination

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--base',default='HEAD');args=parser.parse_args()
    print(snapshot(args.base))
