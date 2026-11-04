#!/usr/bin/env python3
import argparse,hashlib,importlib.util,io,json,os,pathlib,subprocess,tarfile,tempfile,tomllib
HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('table_support',HERE/'generate.py');S=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(S)
ROOT=S.ROOT
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');a=p.parse_args();lock=tomllib.loads(subprocess.check_output(['git','-C',str(ROOT/'.toolchain/upstream-rust'),'show',S.PIN+':Cargo.lock'],env=dict(os.environ,GIT_NO_LAZY_FETCH='1'),text=True));expected=next(p['checksum']for p in lock['package']if p['name']=='hashbrown'and p['version']=='0.17.1')
 archives=list((pathlib.Path(os.environ.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home'))/'registry/cache').glob('*/hashbrown-0.17.1.crate'))
 if len(archives)!=1 or archives[0].is_symlink()or sha(archives[0].read_bytes())!=expected:raise SystemExit('Pinned hashbrown archive differs')
 proof=None;profiles=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-scalar-')as temporary:
  tmp=pathlib.Path(temporary)
  with tarfile.open(fileobj=io.BytesIO(archives[0].read_bytes()))as archive:
   for member in archive.getmembers():
    path=pathlib.PurePosixPath(member.name)
    if path.is_absolute()or'..'in path.parts or not(member.isdir()or member.isfile()):raise SystemExit('Invalid source member')
   archive.extractall(tmp,filter='data')
  for index,(name,fallback,version)in enumerate(S.PROFILES):
   rustc=S.compiler(os.environ.get('FERRUGO_RUSTC'if index==0 else'FERRUGO_RUSTC_NIGHTLY'),fallback)
   if subprocess.check_output([rustc,'--version'],text=True).strip()!=version:raise SystemExit('Reference compiler differs')
   dest=tmp/name;dest.mkdir();lib=dest/'libhashbrown.rlib';exe=dest/'scalar';env=dict(os.environ,RUSTC_BOOTSTRAP='1')
   subprocess.run([rustc,'--edition=2024','--crate-name=hashbrown','--crate-type=rlib','--cfg=miri','--cfg=feature="nightly"','--cfg=feature="rustc-internal-api"',str(tmp/'hashbrown-0.17.1/src/lib.rs'),'-o',str(lib)],env=env,check=True)
   subprocess.run([rustc,'--edition=2024',str(HERE/'scalar_oracle.rs'),'--extern','hashbrown='+str(lib),'-o',str(exe)],env=env,check=True)
   current=subprocess.check_output([str(exe)])
   if proof is not None and proof!=current:raise SystemExit('Actual scalar profiles differ')
   proof=current;profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'cfg':['miri','feature=nightly','feature=rustc-internal-api'],'raw_sha256':sha(current)})
 manifest={'scope':'Original hashbrown0.17.1 scalar eight-byte little-endian control-group branch, forced by cfg(miri) on host; not Miri interpretation and no fullcrate credit','profiles':profiles,'raw_sha256':sha(proof),'hashbrown_archive_sha256':expected,'oracle_sha256':sha((HERE/'scalar_oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'support_generator_sha256':sha((HERE/'generate.py').read_bytes())}
 fixture=ROOT/'port/support/rusthashmap/testdata/scalar.tsv';metadata=HERE/'scalar_manifest.json'
 if a.check:
  if fixture.read_bytes()!=proof or json.loads(metadata.read_text())!=manifest:raise SystemExit('Scalar proof differs')
 else:fixture.write_bytes(proof);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print('Original scalar false-positive equality traces match both compilers')
if __name__=='__main__':main()
