#!/usr/bin/env python3
"""Retain compact all-scalar digests for the explicit pinned Unicode18 profile."""
import argparse
import hashlib
import json
from pathlib import Path
import struct
import subprocess

import native_profile

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,**kwargs):
    result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120,**kwargs)
    if result.returncode:raise SystemExit(f'command failed: {command}\n{result.stderr.decode(errors="replace")[:4096]}')
    return result.stdout
def main():
    parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--offline',action='store_true');args=parser.parse_args()
    rustc,env,profile=native_profile.compiler_profile()
    cache=ROOT/'.toolchain/oracles/symbol-unicode18';cache.mkdir(parents=True,exist_ok=True)
    binary=cache/'oracle'
    run([rustc,'--edition=2024','-C','opt-level=2',str(HERE/'unicode18_oracle.rs'),'-o',str(binary)],env=env)
    raw=run([str(binary)],env=env)
    if len(raw)>256<<20:raise SystemExit('native Unicode18 output exceeds bound')
    blocks=[];offset=0;scalars=0
    while offset<len(raw):
        start,count,length=struct.unpack_from('<III',raw,offset);offset+=12
        if length>1<<20 or offset+length>len(raw):raise SystemExit('malformed native Unicode18 block')
        data=raw[offset:offset+length];offset+=length;scalars+=count
        blocks.append({'start':start,'scalars':count,'bytes':length,'sha256':sha(data)})
    if len(blocks)!=272 or scalars!=1112064:raise SystemExit('native Unicode18 scalar count mismatch')
    sources=[]
    for path in ['library/core/src/char/methods.rs','library/core/src/unicode/unicode_data.rs','library/core/src/fmt/mod.rs','library/alloc/src/str.rs']:
        data=native_profile.source_bytes(path)
        blob=hashlib.sha1(f'blob {len(data)}\0'.encode()+data).hexdigest()
        sources.append({'path':path,'git_blob':blob,'sha256':sha(data)})
    bindings=['port/rust/compiler/rustc_span/src/symbol_unicode.go','port/support/rustfmt/unicode18.go','port/rust/library/core/src/unicode/unicode_data.go']
    metadata={'rust_revision':native_profile.REVISION,**profile,'scope':'Source18 profile only: all valid scalar char/str Debug writer traces; four quote-flag escape results; scalar lowercase/uppercase and two Greek-sigma contexts per scalar. Actual matching-nightly std methods; does not replace neutral comparison-runtime Unicode17 helpers or claim complete std formatting/string ports.','scalar_values':scalars,'block_size':4096,'native_stream_bytes':len(raw),'blocks':blocks,'source_dependencies':sources,'go_bindings':[{'path':path,'sha256':sha((ROOT/path).read_bytes())}for path in bindings],'helpers':[{'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())} for path in [HERE/'generate_unicode18.py',HERE/'unicode18_oracle.rs',HERE/'native_profile.py']]}
    target=ROOT/'port/rust/compiler/rustc_span/src/symbol_unicode18_manifest.json';encoded=(json.dumps(metadata,ensure_ascii=False,indent=2)+'\n').encode()
    if args.check:
        if not target.exists() or target.read_bytes()!=encoded:raise SystemExit('Unicode18 manifest differs')
    else:target.write_bytes(encoded)
    print(json.dumps({'blocks':len(blocks),'scalars':scalars,'native_bytes':len(raw)}))
if __name__=='__main__':main()
