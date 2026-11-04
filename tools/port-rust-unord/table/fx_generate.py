#!/usr/bin/env python3
import argparse,gzip,hashlib,importlib.util,io,json,os,pathlib,subprocess,tarfile,tempfile,tomllib
HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('table_support',HERE/'generate.py');S=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(S)
ROOT=S.ROOT;FIXTURE=ROOT/'port/support/rusthashmap/testdata/fx.tsv.gz'
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');a=p.parse_args()
 lock=tomllib.loads(subprocess.check_output(['git','-C',str(ROOT/'.toolchain/upstream-rust'),'show',S.PIN+':Cargo.lock'],env=dict(os.environ,GIT_NO_LAZY_FETCH='1'),text=True));expected=next(p['checksum']for p in lock['package']if p['name']=='rustc-hash'and p['version']=='2.1.1')
 archives=list((pathlib.Path(os.environ.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home'))/'registry/cache').glob('*/rustc-hash-2.1.1.crate'))
 if len(archives)!=1 or archives[0].is_symlink()or sha(archives[0].read_bytes())!=expected:raise SystemExit('Pinned source archive differs')
 proof=None;profiles=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-fx-')as temporary:
  tmp=pathlib.Path(temporary)
  with tarfile.open(fileobj=io.BytesIO(archives[0].read_bytes()))as archive:
   for member in archive.getmembers():
    path=pathlib.PurePosixPath(member.name)
    if path.is_absolute()or'..'in path.parts or not(member.isdir()or member.isfile()):raise SystemExit('Invalid source member')
   archive.extractall(tmp,filter='data')
  for index,(name,fallback,version)in enumerate(S.PROFILES):
   rustc=S.compiler(os.environ.get('FERRUGO_RUSTC'if index==0 else'FERRUGO_RUSTC_NIGHTLY'),fallback)
   if subprocess.check_output([rustc,'--version'],text=True).strip()!=version:raise SystemExit('Reference compiler differs')
   dest=tmp/name;dest.mkdir();lib=dest/'librustc_hash.rlib';exe=dest/'fx'
   subprocess.run([rustc,'--edition=2021','--crate-name=rustc_hash','--crate-type=rlib','--cfg=feature="std"',str(tmp/'rustc-hash-2.1.1/src/lib.rs'),'-o',str(lib)],check=True)
   subprocess.run([rustc,'--edition=2024',str(HERE/'fx_oracle.rs'),'--extern','rustc_hash='+str(lib),'-o',str(exe)],env=dict(os.environ,RUSTC_BOOTSTRAP='1'),check=True)
   current=subprocess.check_output([str(exe)])
   if proof is not None and proof!=current:raise SystemExit('Actual Fx profiles differ')
   proof=current;profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'cases':len(current.splitlines()),'raw_sha256':sha(current)})
 compressed=gzip.compress(proof,mtime=0)
 if FIXTURE.exists()and gzip.decompress(FIXTURE.read_bytes())==proof:compressed=FIXTURE.read_bytes()
 manifest={'scope':'Used rustc-hash2.1.1 std-only hasher operations under actual64-bit compiler profiles; no fullcrate credit','profiles':profiles,'cases':len(proof.splitlines()),'raw_sha256':sha(proof),'fixture_sha256':sha(compressed),'archive_sha256':expected,'oracle_sha256':sha((HERE/'fx_oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'support_generator_sha256':sha((HERE/'generate.py').read_bytes())}
 metadata=HERE/'fx_manifest.json'
 if a.check:
  if json.loads(metadata.read_text())!=manifest or FIXTURE.read_bytes()!=compressed:raise SystemExit('Fx reference differs')
 else:FIXTURE.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print(str(manifest['cases'])+' actual Fx hasher cases match both std profiles')
if __name__=='__main__':main()
