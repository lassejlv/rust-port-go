#!/usr/bin/env python3
"""Exhaustive actual-SDK scalar formatting proof, independent of Symbol WIP."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import subprocess
import sys
import tarfile

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
sys.path.insert(0,str(ROOT/'tools/port-rust-symbol'))
import native_profile
SOURCE={
 'library/core/src/char/methods.rs':(97609,'64f399f062cd94ff28dfc05960d9918b72876277b37e189c264137f1d9991d7f'),
 'library/core/src/fmt/mod.rs':(110834,'72cd4601ca6a6b61612d2edbb58d534e1d5c6403cf23d6228d332e372dd716ba'),
}
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 rustc,env,compiler=native_profile.compiler_profile()
 spec=importlib.util.spec_from_file_location('unicode18_sdk_sources',ROOT/'tools/port-rust-str/prepare.py');sdk=importlib.util.module_from_spec(spec);spec.loader.exec_module(sdk)
 _,cache=sdk.verified();sources=[]
 with tarfile.open(cache/'rust-src-nightly.tar.xz','r:xz')as archive:
  for path,(size,digest)in SOURCE.items():
   body=archive.extractfile('rust-src-nightly/rust-src/lib/rustlib/src/rust/'+path).read()
   if len(body)!=size or sha(body)!=digest:raise SystemExit('Unicode18 original SDK source differs:'+path)
   sources.append({'path':path,'bytes':size,'sha256':digest})
 work=ROOT/'.toolchain/oracles/unicode18-format';work.mkdir(parents=True,exist_ok=True);binary=work/'oracle'
 p=subprocess.run([rustc,'--edition=2024','-C','opt-level=2',str(HERE/'oracle.rs'),'-o',str(binary)],env=env,capture_output=True,timeout=60)
 if p.returncode:raise SystemExit(p.stderr.decode(errors='replace')[:4000])
 p=subprocess.run([str(binary)],env=env,capture_output=True,timeout=120)
 if p.returncode:raise SystemExit('Unicode18 native formatter failed')
 raw=p.stdout
 if len(raw)>128<<20:raise SystemExit('Unicode18 native proof exceeds bound')
 offset=scalars=0;blocks=[]
 while offset<len(raw):
  if offset+12>len(raw):raise SystemExit('truncated Unicode18 native header')
  start,count,length=struct.unpack_from('<III',raw,offset);offset+=12
  if length>1<<20 or offset+length>len(raw):raise SystemExit('invalid Unicode18 native block')
  data=raw[offset:offset+length];offset+=length;scalars+=count
  blocks.append({'start':start,'scalars':count,'bytes':length,'sha256':sha(data)})
 if len(blocks)!=272 or scalars!=1112064:raise SystemExit('Unicode18 native scalar count differs')
 paths=[HERE/'generate.py',HERE/'oracle.rs',ROOT/'tools/port-rust-symbol/native_profile.py',ROOT/'tools/port-rust-str/prepare.py',ROOT/'tools/port-rust-str/SOURCES.json',ROOT/'port/support/rustfmt/unicode18.go',ROOT/'port/support/rustfmt/unicode18_oracle_test.go',ROOT/'port/rust/library/core/src/unicode/unicode_data.go']
 metadata={'complete':False,'scope':'Scoped Unicode18 char/str Debug exact writer traces and public char escape_debug for every valid scalar. Independent of compiler Symbol implementation; no full standard-library source-file credit.','rust_revision':native_profile.REVISION,**compiler,'sdk_source_archive_sha256':sdk.ARCHIVE,'sdk_sources':sources,'scalar_values':scalars,'block_size':4096,'native_bytes':len(raw),'blocks':blocks,'files':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in paths}}
 target=ROOT/'port/support/rustfmt/testdata/unicode18-manifest.json';encoded=(json.dumps(metadata,indent=2)+'\n').encode()
 if args.check:
  if target.read_bytes()!=encoded:raise SystemExit('Unicode18 native formatter evidence differs')
 else:target.write_bytes(encoded)
 print(f'Unicode18 formatting:{scalars} native scalars in {len(blocks)} exact writer/escape blocks')
if __name__=='__main__':main()
