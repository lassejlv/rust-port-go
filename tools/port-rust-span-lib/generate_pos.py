#!/usr/bin/env python3
"""Observe actual arbitrary Pos trait implementations and non-Copy ownership."""
import argparse,hashlib,json,os
from pathlib import Path
import subprocess,tempfile
import reference

ROOT=Path(__file__).resolve().parents[2];HERE=Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--profile',type=Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args();profile,environment=reference.load_profile(a.profile)
 source=HERE/'native_pos.rs';rows=[];aborts=0;queries=[(mode,str(fail),'false')for mode in ['normal','unwinding','processing']for fail in range(5)]
 with tempfile.TemporaryDirectory(prefix='ferrugo-lib-pos-')as directory:
  executable=Path(directory)/'native';subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)],check=True,cwd=ROOT,env=environment,timeout=120)
  for mode,fail,panic in queries:
   result=subprocess.run([str(executable),mode,fail,panic],stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=ROOT,timeout=10)
   if result.returncode not in [0,-6]:raise RuntimeError('unexpected original Pos status: '+str(result.returncode))
   abort=result.returncode==-6;aborts+=abort
   if len(result.stderr)>1<<20:raise RuntimeError('native Pos observations exceed bound')
   events=','.join(line.removeprefix('EVENT:')for line in result.stderr.decode().splitlines()if line.startswith('EVENT:'))
   rows.append('\t'.join([mode,fail,panic,str(abort).lower(),events])+'\n')
 raw=''.join(rows).encode();destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-pos.tsv.gz';compressed=reference.compressed_fixture(raw,destination,1<<20)
 proof={'schema':1,'status':'partial-lib-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'generator_sha256':sha(Path(__file__).read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'records':len(rows),'actual_aborts':aborts,'scope':'All four arbitrary Pos callbacks, actual receiver cells with interior mutation, non-Copy receiver Drop, current inherited/processing panic contexts; no global TLS/allocator ABI claim.','raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode();manifest=HERE/'pos-manifest.json'
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('original Pos evidence differs')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'Original Pos trait records: {len(rows)}; actual aborts: {aborts}')
if __name__=='__main__':
 try:main()
 except(Exception)as error:raise SystemExit(str(error))
