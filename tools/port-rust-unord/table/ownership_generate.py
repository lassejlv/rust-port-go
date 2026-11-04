#!/usr/bin/env python3
"""Retain actual std-owned callback, cleanup, and partial-state outcomes."""
import argparse,gzip,hashlib,importlib.util,io,json,os,pathlib,subprocess,tarfile,tempfile,tomllib
HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('table_support',HERE/'generate.py');SUPPORT=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(SUPPORT)
ROOT=SUPPORT.ROOT
FIXTURE=ROOT/'port/support/rusthashmap/testdata/ownership.tsv.gz'
def sha(b):return hashlib.sha256(b).hexdigest()
def build(rustc,source,dest):
 lib=dest/'librustc_hash.rlib';exe=dest/'ownership'
 subprocess.run([rustc,'--edition=2021','--crate-name=rustc_hash','--crate-type=rlib','--cfg=feature="std"',str(source/'src/lib.rs'),'-o',str(lib)],check=True)
 subprocess.run([rustc,'--edition=2024',str(HERE/'ownership_oracle.rs'),'--extern','rustc_hash='+str(lib),'-o',str(exe)],env=dict(os.environ,RUSTC_BOOTSTRAP='1'),check=True)
 return exe
def execute(exe,requests):
 r=subprocess.run([str(exe)],input=requests,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 if r.returncode:raise SystemExit('Owned reference did not complete:'+str(r.returncode)+':'+r.stderr.decode(errors='replace'))
 if len(r.stdout.splitlines())!=len(requests.splitlines()):raise SystemExit('Owned reference record count differs')
 return r.stdout
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');a=p.parse_args()
 lock=tomllib.loads(subprocess.check_output(['git','-C',str(ROOT/'.toolchain/upstream-rust'),'show',SUPPORT.PIN+':Cargo.lock'],env=dict(os.environ,GIT_NO_LAZY_FETCH='1'),text=True))
 expected=next(x['checksum']for x in lock['package']if x['name']=='rustc-hash'and x['version']=='2.1.1')
 home=pathlib.Path(os.environ.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home')))
 archives=list((home/'registry/cache').glob('*/rustc-hash-2.1.1.crate'))
 if len(archives)!=1 or archives[0].is_symlink()or sha(archives[0].read_bytes())!=expected:raise SystemExit('Pinned source archive differs')
 profiles=[];proof=None;requests=None
 with tempfile.TemporaryDirectory(prefix='ferrugo-map-owned-')as temporary:
  tmp=pathlib.Path(temporary)
  with tarfile.open(fileobj=io.BytesIO(archives[0].read_bytes()))as archive:
   for member in archive.getmembers():
    path=pathlib.PurePosixPath(member.name)
    if path.is_absolute()or'..'in path.parts or not(member.isfile()or member.isdir()):raise SystemExit('Invalid source archive member')
   archive.extractall(tmp,filter='data')
  for index,(name,default,version)in enumerate(SUPPORT.PROFILES):
   rustc=SUPPORT.compiler(os.environ.get('FERRUGO_RUSTC'if index==0 else'FERRUGO_RUSTC_NIGHTLY'),default)
   if subprocess.check_output([rustc,'--version'],text=True).strip()!=version:raise SystemExit('Recorded compiler differs')
   dest=tmp/name;dest.mkdir();exe=build(rustc,tmp/'rustc-hash-2.1.1',dest)
   baseline_requests=''.join(f'{s}\t{ambient}\t-1\n'for s in range(28)for ambient in ['normal','unwinding']).encode()
   baseline=execute(exe,baseline_requests)
   expanded=[]
   for row in baseline.decode().splitlines():
    fields=row.split('\t');scenario,ambient=fields[:2];expanded.append('\t'.join(fields[:3])+'\n')
    # Constructor overflow is already unwinding when its Builder drops. A
    # second forced destructor panic would abort, requiring a separate prefix
    # corpus rather than fabricated catch_unwind state.
    if scenario!='24':
     expanded.extend(f'{scenario}\t{ambient}\t{fail}\n'for fail in range(int(fields[5])))
   current_requests=''.join(expanded).encode()
   current=execute(exe,current_requests)
   if proof is not None and(proof!=current or requests!=current_requests):raise SystemExit('Actual owned profiles differ')
   proof,requests=current,current_requests
   profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'raw_sha256':sha(current),'cases':len(current.splitlines())})
 compressed=gzip.compress(proof,mtime=0)
 if FIXTURE.exists()and gzip.decompress(FIXTURE.read_bytes())==proof:compressed=FIXTURE.read_bytes()
 manifest={'scope':'Actual std HashMap/Entry owned traits and temporary capture environments, normal and inherited unwind contexts; scoped supporting contract, no std or Unord wholefile completion credit','profiles':profiles,'cases':len(proof.splitlines()),'requests_sha256':sha(requests),'raw_sha256':sha(proof),'fixture_sha256':sha(compressed),'rustc_hash_archive_sha256':expected,'oracle_sha256':sha((HERE/'ownership_oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'support_generator_sha256':sha((HERE/'generate.py').read_bytes())}
 metadata=HERE/'ownership_manifest.json'
 if a.check:
  if json.loads(metadata.read_text())!=manifest or FIXTURE.read_bytes()!=compressed:raise SystemExit('Owned reference proof differs')
 else:FIXTURE.parent.mkdir(parents=True,exist_ok=True);FIXTURE.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print(str(manifest['cases'])+' actual owned-trait cases match both std profiles')
if __name__=='__main__':main()
