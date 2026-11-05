#!/usr/bin/env python3
"""Original bitflags iterator/operation observations; no whole-file completion claim."""
import argparse,gzip,hashlib,io,json,os,pathlib,subprocess,tempfile
import reference
ROOT=pathlib.Path(__file__).resolve().parents[2];HERE=pathlib.Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--profile',type=pathlib.Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args();profile,environment=reference.load_profile(a.profile);assert profile['source_revision']=='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
 source=HERE/'native_flags_iter.rs';destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-flags-iter.tsv.gz';manifest=HERE/'flags-iter-manifest.json'
 with tempfile.TemporaryDirectory(prefix='ferrugo-flags-iter-')as directory:
  executable=pathlib.Path(directory)/'native';libraries=[lib for directory in profile['dependency_library_directories']for lib in pathlib.Path(directory).glob('libbitflags-*.rlib')];assert len(libraries)==1;subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'--extern','bitflags='+str(libraries[0].with_suffix('.rmeta')),'--extern','bitflags='+str(libraries[0]),'-o',str(executable)],check=True,cwd=ROOT,env=environment);raw=subprocess.run([str(executable)],check=True,stdout=subprocess.PIPE,cwd=ROOT).stdout
 assert len(raw)<16*1024*1024;compressed=reference.compressed_fixture(raw,destination,16*1024*1024)
 proof={'schema':1,'status':'partial-serializer-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'records':len(raw.splitlines()),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise SystemExit('bitflags iterator/operation evidence is stale')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f"bitflags iterator/operation native records: {proof['records']}")
if __name__=='__main__':
 try:main()
 except subprocess.CalledProcessError as error:raise SystemExit(f'native bitflags iterator/operation command failed with status {error.returncode}')
