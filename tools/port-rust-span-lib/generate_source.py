#!/usr/bin/env python3
"""Bounded external-client comparison against the unchanged whole span crate.

This is in-progress lib.rs evidence, not a whole-file completion attestation.
The shared reference preparer verifies all original/compiler/registry sources.
"""
import argparse,gzip,hashlib,io,json,os,pathlib,subprocess,tempfile
import reference
ROOT=pathlib.Path(__file__).resolve().parents[2]
HERE=pathlib.Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--profile',type=pathlib.Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args()
 profile,environment=reference.load_profile(a.profile);assert profile['source_revision']=='2d20b42b883dcfc014f3c2c56081eeb6091c2179';assert profile['unicode']==[18,0,0];assert profile['debug_assertions']and profile['overflow_checks']
 source=HERE/'native_source.rs';destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-source.tsv.gz';manifest=HERE/'source-manifest.json'
 with tempfile.TemporaryDirectory(prefix='ferrugo-span-lib-source-')as directory:
  executable=pathlib.Path(directory)/'native'
  command=[profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)]
  subprocess.run(command,check=True,cwd=ROOT,env=environment)
  raw=subprocess.run([str(executable)],check=True,stdout=subprocess.PIPE,cwd=ROOT).stdout
 assert len(raw)<32*1024*1024
 compressed=reference.compressed_fixture(raw,destination,32*1024*1024)
 proof={'schema':1,'status':'partial-lib-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'unicode':profile['unicode'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'records':len(raw.splitlines()),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed)}
 encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise SystemExit('lib source evidence is stale')
 else:
  destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f"source-file native records: {proof['records']}")
if __name__=='__main__':
 try:main()
 except subprocess.CalledProcessError as error:raise SystemExit(f'native source command failed with status {error.returncode}')
