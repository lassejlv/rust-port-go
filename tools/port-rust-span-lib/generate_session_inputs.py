#!/usr/bin/env python3
"""Original session constructor input observations; no whole-file completion claim."""
import argparse,gzip,hashlib,io,json,os,pathlib,subprocess,tempfile
import reference
ROOT=pathlib.Path(__file__).resolve().parents[2];HERE=pathlib.Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--profile',type=pathlib.Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args();profile,environment=reference.load_profile(a.profile);assert profile['source_revision']=='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
 source=HERE/'native_session_inputs.rs';destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-session-inputs.tsv.gz';manifest=HERE/'session-inputs-manifest.json'
 with tempfile.TemporaryDirectory(prefix='ferrugo-session-inputs-')as directory:
  executable=pathlib.Path(directory)/'native';subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)],check=True,cwd=ROOT,env=environment);raw=subprocess.run([str(executable)],check=True,stdout=subprocess.PIPE,cwd=ROOT).stdout
  for op,mode in [('create',3),('create',4),('create',6),('construct',3),('construct',4),('construct',6),('present',5),('present',6)]:
   child=subprocess.run([str(executable),op,str(mode)],stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=dict(environment,FERRUGO_TRACE_EVENTS='1'),timeout=10)
   if child.returncode!=-6:raise SystemExit('expected constructor SIGABRT')
   events=[line.removeprefix('EVENT:')for line in child.stderr.decode().splitlines()if line.startswith('EVENT:')]
   raw+=('abort_'+op+'\t'+str(mode)+'\t'+','.join(events)+'\n').encode()
 assert len(raw)<4*1024*1024;compressed=reference.compressed_fixture(raw,destination,4*1024*1024)
 proof={'schema':1,'status':'partial-lib-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'records':len(raw.splitlines()),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise SystemExit('session constructor input evidence is stale')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f"session constructor input native records: {proof['records']}")
if __name__=='__main__':
 try:main()
 except subprocess.CalledProcessError as error:raise SystemExit(f'native session constructor input command failed with status {error.returncode}')
