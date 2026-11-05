#!/usr/bin/env python3
"""Observe source slicing within actual inherited-unwind and panic-hook paths."""
import argparse,hashlib,json,os
from pathlib import Path
import subprocess,tempfile
import prepare

HERE=Path(__file__).resolve().parent
ROOT=prepare.ROOT
OPS=['valid','boundary','backward','begin_oob','end_oob','inclusive_boundary','inclusive_len','from_boundary','to_boundary']
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--cache',type=Path);a=p.parse_args();rustc,compiler=prepare.selected(a.rustc);prepare.verified(a.cache)
 source=HERE/'ambient.rs';rows=[];aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-str-ambient-')as directory:
  executable=Path(directory)/'native';subprocess.run([rustc,'--edition=2024',str(source),'-o',str(executable)],cwd=ROOT,check=True,timeout=60)
  for ambient in ['normal','unwinding','processing']:
   for op in OPS:
    child=subprocess.run([str(executable),ambient,op],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
    if child.returncode not in [0,-6]:raise RuntimeError('unexpected original ambient status: '+str(child.returncode))
    abort=child.returncode==-6;aborts+=abort;events=[line.removeprefix('EVENT:')for line in child.stderr.decode().splitlines()if line.startswith('EVENT:')]
    rows.append('\t'.join([ambient,op,str(abort).lower(),','.join(events)])+'\n')
 raw=''.join(rows).encode();proof={'schema':1,'compiler':compiler.strip(),'sdk_source_sha256':sha((HERE/'SOURCES.json').read_bytes()),'cases':len(rows),'actual_aborts':aborts,'raw_sha256':sha(raw),'oracle_sha256':sha(source.read_bytes()),'tool_sha256':{name:sha((HERE/name).read_bytes())for name in ['generate_ambient.py','prepare.py']},'go_source_sha256':sha((ROOT/'port/support/ruststr/slice.go').read_bytes()),'scope':'Real SDK operation contexts; abort records retain events only, never post-abort state.'};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 fixture=ROOT/'port/support/ruststr/testdata/ambient.tsv';manifest=fixture.with_name('ambient-manifest.json')
 if a.check:
  if not fixture.exists()or fixture.read_bytes()!=raw or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('original ambient evidence differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(raw);manifest.write_bytes(encoded)
 print(f'Original ambient str records: {len(rows)}; actual aborts: {aborts}')
if __name__=='__main__':
 try:main()
 except(Exception)as error:raise SystemExit(str(error))
