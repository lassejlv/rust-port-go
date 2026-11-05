#!/usr/bin/env python3
"""Original generic aggregate/session observations; scoped dependency, no file credit."""
import argparse,gzip,hashlib,io,json,os,pathlib,subprocess,tempfile
import reference
ROOT=pathlib.Path(__file__).resolve().parents[2];HERE=pathlib.Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--profile',type=pathlib.Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args();profile,environment=reference.load_profile(a.profile);assert profile['source_revision']=='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
 source=HERE/'native_generic.rs';destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-generic.tsv.gz';manifest=HERE/'generic-manifest.json'
 with tempfile.TemporaryDirectory(prefix='ferrugo-generic-')as directory:
  executable=pathlib.Path(directory)/'native';subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)],check=True,cwd=ROOT,env=environment);raw=subprocess.run([str(executable)],check=True,stdout=subprocess.PIPE,cwd=ROOT).stdout
  for op in ["create","default","with_absent","if_absent","set","with_present","if_present","create_present","set_present"]:
   call=op not in ["with_absent","create_present","set_present"]
   child=subprocess.run([str(executable),op,str(call).lower()],env=dict(os.environ,FERRUGO_TRACE_EVENTS='1'),stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=ROOT,timeout=10)
   if child.returncode != -6:raise SystemExit(f'expected native SIGABRT: {op}: {child.returncode}')
   events=[line.removeprefix('EVENT:')for line in child.stderr.decode().splitlines()if line.startswith('EVENT:')]
   raw+=('abort\t'+op+'\t'+str(call).lower()+'\ttrue\t'+','.join(events)+'\n').encode()
 assert len(raw)<4*1024*1024;compressed=reference.compressed_fixture(raw,destination,4*1024*1024)
 proof={'schema':1,'status':'partial-serializer-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'records':len(raw.splitlines()),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise SystemExit('generic aggregate/session evidence is stale')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f"generic aggregate/session native records: {proof['records']}")
if __name__=='__main__':
 try:main()
 except subprocess.CalledProcessError as error:raise SystemExit(f'native generic aggregate/session command failed with status {error.returncode}')
