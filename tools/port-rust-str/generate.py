#!/usr/bin/env python3
"""Observe exact SDK str slicing, boundaries and panic metadata."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import random
import subprocess
import tempfile

import prepare

ROOT=prepare.ROOT
HERE=Path(__file__).resolve().parent
LIMIT=64<<20
OPS=['range','from','to','inclusive','to_inclusive','exhausted','get','get_inclusive','get_exhausted','boundaries']
def sha(data):return hashlib.sha256(data).hexdigest()
def requests():
 result=[];maximum=(1<<64)-1
 texts=['','ascii','é','🦀','aé🦀z','\x00\t\n\\"\'é','α\u200dβ\u0301🦀','\u323a','\U000323af','\ue000','\u2028','\u2003','\ufeff','é'*257,'🦀'*129]
 rng=random.Random(0x708C94)
 # Representative characters plus seeded Unicode scalars compose with the
 # independently exhaustive Unicode18 char-Debug proof.
 scalars=[0x80,0x85,0xa0,0x301,0x378,0x600,0x200b,0x200d,0x2028,0x2069,0x323af,0x10ffff]
 while len(scalars)<512:
  c=rng.randrange(0x80,0x110000)
  if not 0xd800<=c<=0xdfff:scalars.append(c)
 for c in scalars:texts.append('a'+chr(c)+'z')
 for text in texts:
  length=len(text.encode());points=sorted({0,1,2,3,4,max(0,length-1),length,length+1,maximum})
  if length>20:points+= [127,128,255,256,257]
  pairs=[(a,b)for a in points for b in points]
  for op in OPS:
   if op in ['from','exhausted','get_exhausted','boundaries']:
    selected=[(a,0)for a in points]
   elif op in ['to','to_inclusive']:selected=[(0,b)for b in points]
   else:selected=pairs
   for begin,end in selected:result.append(f'{text.encode().hex()}\t{op}\t{begin}\t{end}\n')
 atoms=['a','β','🦀','é','\0','\n','\u0301','\u200d','\U000323af','\ue000']
 for _ in range(2000):
  text=''.join(rng.choice(atoms)for _ in range(rng.randrange(0,40)));length=len(text.encode());begin=rng.choice([rng.randrange(length+4),maximum]);end=rng.choice([rng.randrange(length+4),maximum]);op=rng.choice(OPS);result.append(f'{text.encode().hex()}\t{op}\t{begin}\t{end}\n')
 return result
def compressed(raw,path):
 if len(raw)>LIMIT:raise RuntimeError('str corpus exceeds bound')
 if path.is_symlink():raise RuntimeError('symlink fixture output')
 if path.exists()and path.stat().st_size<=LIMIT:
  old=path.read_bytes()
  try:
   with gzip.GzipFile(fileobj=io.BytesIO(old))as stream:previous=stream.read(LIMIT+1)
   if previous==raw:return old
  except(OSError,EOFError):pass
 return gzip.compress(raw,mtime=0)
def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--check',action='store_true');parser.add_argument('--rustc');parser.add_argument('--cache',type=Path);args=parser.parse_args()
 rustc,compiler=prepare.selected(args.rustc);sources,_=prepare.verified(args.cache)
 cfg=subprocess.check_output([rustc,'--print','cfg']).decode().splitlines()
 if 'target_pointer_width="64"'not in cfg:raise RuntimeError('retained reference requires64-bit SDK')
 inputs=requests();request=''.join(inputs).encode();source=HERE/'oracle.rs';locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-str-')as directory:
  executable=Path(directory)/'native';subprocess.run([rustc,'--edition=2024','-C','opt-level=1',str(source),'-o',str(executable)],cwd=ROOT,env=dict(os.environ,RUSTC_BOOTSTRAP='1',FERRUGO_ORACLE_ROOT=str(ROOT)),check=True,timeout=60)
  output=Path(directory)/'observations'
  with output.open('wb')as stream:subprocess.run([str(executable)],input=request,stdout=stream,cwd=ROOT,check=True,timeout=90)
  if output.stat().st_size>LIMIT:raise RuntimeError('original str output exceeds bound')
  observed=output.read_bytes().decode().splitlines()
 if len(observed)!=len(inputs):raise RuntimeError('original str request/output count differs')
 rows=[];panics=0
 for query,response in zip(inputs,observed):
  op=query.split('\t')[1];result,hook=response.split('\t')
  if result=='panic':
   panics+=1;event=hook.split(':')
   if len(event)!=8:raise RuntimeError('unexpected original hook shape')
   location={'File':bytes.fromhex(event[2]).decode(),'Line':int(event[3]),'Column':int(event[4])}
   if op in locations and locations[op]!=location:raise RuntimeError('caller location differs within one original operation')
   locations[op]=location
  elif hook:raise RuntimeError('successful str operation invoked panic hook')
  rows.append(query.rstrip('\n')+'\t'+response+'\n')
 raw=''.join(rows).encode();path=ROOT/'port/support/ruststr/testdata/cases.tsv.gz';gz=compressed(raw,path)
 gofiles=['port/support/ruststr/slice.go','port/support/rustfmt/unicode18.go']
 proof={'schema':1,'scope':'Scoped actual SDK str indexing/boundaries/panic payloads, no std-file credit','compiler':compiler.strip(),'sdk_source_sha256':sha((HERE/'SOURCES.json').read_bytes()),'unicode':[18,0,0],'cases':len(inputs),'panics':panics,'locations':locations,'location_profile':'Caller filenames are exact oracle-relative paths after stripping only repository-root prefix; production never normalizes supplied locations.','request_sha256':sha(request),'raw_sha256':sha(raw),'fixture_sha256':sha(gz),'oracle_sha256':sha(source.read_bytes()),'tool_sha256':{name:sha((HERE/name).read_bytes())for name in ['generate.py','prepare.py']},'go_sources':{name:sha((ROOT/name).read_bytes())for name in gofiles}}
 encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode();manifest=path.with_name('manifest.json')
 if args.check:
  if not path.exists()or path.read_bytes()!=gz or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('original str evidence differs')
 else:path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(gz);manifest.write_bytes(encoded)
 print(f'Original str requests: {len(inputs):,}; caught panics: {panics:,}')
if __name__=='__main__':
 try:main()
 except(Exception)as error:raise SystemExit(str(error))
