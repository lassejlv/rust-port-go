#!/usr/bin/env python3
"""Observe actual public diagnostic enum Box moves/clones and owned cleanup."""
import argparse,hashlib,json,os
from pathlib import Path
import subprocess,tempfile
import reference

ROOT=Path(__file__).resolve().parents[2];HERE=Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--profile',type=Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args();profile,environment=reference.load_profile(a.profile)
 source=HERE/'native_boxes.rs';rows=[];aborts=0;queries=[('identity',str(seed),'false')for seed in [0,1,31,(1<<32)-1]]+[(mode,str(fail),str(panic).lower())for mode in ['normal','unwinding','processing']for panic in [False,True]for fail in [-1,0,1,3,7,30]]
 with tempfile.TemporaryDirectory(prefix='ferrugo-lib-boxes-')as directory:
  executable=Path(directory)/'native';subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)],check=True,cwd=ROOT,env=environment,timeout=120)
  for mode,fail,panic in queries:
   result=subprocess.run([str(executable),mode,fail,panic],stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=ROOT,timeout=10)
   if result.returncode not in [0,-6]:raise RuntimeError('unexpected original box status: '+str(result.returncode))
   abort=result.returncode==-6;aborts+=abort
   if len(result.stderr)>1<<20:raise RuntimeError('native box observations exceed bound')
   events=','.join(line.removeprefix('EVENT:')for line in result.stderr.decode().splitlines()if line.startswith('EVENT:'))
   rows.append('\t'.join([mode,fail,panic,str(abort).lower(),events])+'\n')
 raw=''.join(rows).encode();destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-boxes.tsv.gz';compressed=reference.compressed_fixture(raw,destination,1<<20)
 proof={'schema':1,'status':'partial-lib-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'generator_sha256':sha(Path(__file__).read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'records':len(rows),'actual_aborts':aborts,'identity_domain':'Only within-process pointee and nonempty-string identity relations are compared; no absolute/cross-language address or allocator ABI. Caller-owned wrapper Drop observes the still-live boxed payload before its automatic field destruction.','raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode();manifest=HERE/'boxes-manifest.json'
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('original Box evidence differs')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'Original enum Box records: {len(rows)}; actual aborts: {aborts}')
if __name__=='__main__':
 try:main()
 except(Exception)as error:raise SystemExit(str(error))
