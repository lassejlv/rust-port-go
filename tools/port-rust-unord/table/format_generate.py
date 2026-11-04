#!/usr/bin/env python3
import argparse,gzip,hashlib,importlib.util,io,json,os,pathlib,subprocess,tarfile,tempfile,tomllib
HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('table_support',HERE/'generate.py');S=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(S);ROOT=S.ROOT
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');a=p.parse_args();lock=tomllib.loads(subprocess.check_output(['git','-C',str(ROOT/'.toolchain/upstream-rust'),'show',S.PIN+':Cargo.lock'],env=dict(os.environ,GIT_NO_LAZY_FETCH='1'),text=True));expected=next(p['checksum']for p in lock['package']if p['name']=='rustc-hash'and p['version']=='2.1.1')
 archives=list((pathlib.Path(os.environ.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home'))/'registry/cache').glob('*/rustc-hash-2.1.1.crate'))
 if len(archives)!=1 or archives[0].is_symlink()or sha(archives[0].read_bytes())!=expected:raise SystemExit('Pinned source differs')
 proof=None;profiles=[];requests=None
 with tempfile.TemporaryDirectory(prefix='ferrugo-map-format-')as temporary:
  tmp=pathlib.Path(temporary)
  with tarfile.open(fileobj=io.BytesIO(archives[0].read_bytes()))as archive:
   for m in archive.getmembers():
    path=pathlib.PurePosixPath(m.name)
    if path.is_absolute()or'..'in path.parts or not(m.isfile()or m.isdir()):raise SystemExit('Invalid member')
   archive.extractall(tmp,filter='data')
  for index,(name,fallback,version)in enumerate(S.PROFILES):
   rustc=S.compiler(os.environ.get('FERRUGO_RUSTC'if index==0 else'FERRUGO_RUSTC_NIGHTLY'),fallback)
   if subprocess.check_output([rustc,'--version'],text=True).strip()!=version:raise SystemExit('Compiler differs')
   dest=tmp/name;dest.mkdir();lib=dest/'librustc_hash.rlib';exe=dest/'format'
   subprocess.run([rustc,'--edition=2021','--crate-name=rustc_hash','--crate-type=rlib','--cfg=feature="std"',str(tmp/'rustc-hash-2.1.1/src/lib.rs'),'-o',str(lib)],check=True)
   subprocess.run([rustc,'--edition=2024',str(HERE/'format_oracle.rs'),'--extern','rustc_hash='+str(lib),'-o',str(exe)],env=dict(os.environ,RUSTC_BOOTSTRAP='1'),check=True)
   baseline=''.join(f'{kind}\t{pattern}\t{str(chars).lower()}\t-1\t{str(reuse).lower()}\n'for kind in range(7)for pattern in range(16)for chars in [False,True]for reuse in [False,True]).encode()
   rows=subprocess.check_output([str(exe)],input=baseline).decode().splitlines();expanded=[]
   for row in rows:
    f=row.split('\t');count=sum(event.startswith(('S:','C:'))for event in f[6].split('|'));expanded.append('\t'.join(f[:5])+'\n');expanded.extend('\t'.join([*f[:3],str(fail),f[4]])+'\n'for fail in range(count+1))
   current_requests=''.join(expanded).encode();current=subprocess.check_output([str(exe)],input=current_requests)
   if proof is not None and(proof!=current or requests!=current_requests):raise SystemExit('Actual format profiles differ')
   proof,requests=current,current_requests;profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'cases':len(current.splitlines()),'raw_sha256':sha(current)})
 fixture=ROOT/'port/support/rusthashmap/testdata/format.tsv.gz';compressed=gzip.compress(proof,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==proof:compressed=fixture.read_bytes()
 manifest={'scope':'Actual std Map/Set/Entry/OccupiedError Debug fallible C/S writes, every write error, formatter state and reuse; scoped support only','profiles':profiles,'cases':len(proof.splitlines()),'raw_sha256':sha(proof),'fixture_sha256':sha(compressed),'requests_sha256':sha(requests),'archive_sha256':expected,'oracle_sha256':sha((HERE/'format_oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'support_generator_sha256':sha((HERE/'generate.py').read_bytes())}
 metadata=HERE/'format_manifest.json'
 if a.check:
  if json.loads(metadata.read_text())!=manifest or fixture.read_bytes()!=compressed:raise SystemExit('Format proof differs')
 else:fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print(str(manifest['cases'])+' actual format/write/error/state/reuse cases match both profiles')
if __name__=='__main__':main()
