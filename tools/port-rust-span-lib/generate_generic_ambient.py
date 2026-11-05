#!/usr/bin/env python3
"""Actual inherited-unwind and panic-hook session FnOnce observations."""
import argparse,gzip,hashlib,io,json,os,pathlib,subprocess,tempfile
import reference
ROOT=pathlib.Path(__file__).resolve().parents[2]
HERE=pathlib.Path(__file__).resolve().parent
LIMIT=1024*1024

def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--profile',type=pathlib.Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args()
 profile,environment=reference.load_profile(a.profile)
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179' or profile['unicode']!=[18,0,0]:raise SystemExit('wrong original source profile')
 source=HERE/'native_generic.rs';destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-generic-ambient.tsv.gz';manifest=HERE/'generic-ambient-manifest.json';rows=[];aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-generic-ambient-')as directory:
  executable=pathlib.Path(directory)/'native';subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)],check=True,cwd=ROOT,env=environment,timeout=120)
  for ambient in ['unwinding','processing']:
   for op in ['create','default','with_absent','if_absent','set','with_present','if_present','create_present','set_present']:
    for call in [False,True]:
     for drop in [False,True]:
      child=subprocess.run([str(executable),op,str(call).lower(),str(drop).lower(),ambient],env=dict(os.environ,FERRUGO_TRACE_EVENTS='1'),stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=ROOT,timeout=10)
      if child.returncode not in [0,-6]:raise SystemExit(f'unexpected native status: {child.returncode}')
      abort=child.returncode==-6;aborts+=abort
      events=','.join(line.removeprefix('EVENT:')for line in child.stderr.decode().splitlines()if line.startswith('EVENT:'))if abort else child.stdout.decode().strip()
      rows.append('\t'.join([op,str(call).lower(),str(drop).lower(),ambient,str(abort).lower(),events])+'\n')
 raw=''.join(rows).encode()
 if len(raw)>LIMIT:raise SystemExit('ambient fixture exceeds bound')
 compressed=reference.compressed_fixture(raw,destination,LIMIT)
 proof={'schema':1,'status':'partial-lib-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'records':len(rows),'actual_aborts':aborts,'raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise SystemExit('generic ambient evidence is stale')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'generic ambient native records: {len(rows)}, actual aborts: {aborts}')
if __name__=='__main__':
 try:main()
 except(subprocess.CalledProcessError,subprocess.TimeoutExpired)as error:raise SystemExit(f'native generic command failed: {error}')
