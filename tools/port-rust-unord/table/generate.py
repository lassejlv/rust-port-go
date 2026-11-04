#!/usr/bin/env python3
"""Compare exact std/FxHashMap operations in both recorded compiler profiles.

This validates a scoped supporting dependency, not the complete Unord source.
"""
import argparse,gzip,hashlib,io,json,os,pathlib,shutil,subprocess,tarfile,tempfile,tomllib
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
PIN='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
PROFILES=[('rust1.99',ROOT/'.toolchain/rust/bin/rustc','rustc 1.99.1 (58cf8f9c3 2026-10-13)'),('rust1.101',ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc','rustc 1.101.0-nightly (708c94ef0 2026-11-03)')]
FIXTURE=ROOT/'port/support/rusthashmap/testdata/table_blocks.tsv.gz'
def sha(b):return hashlib.sha256(b).hexdigest()
def compiler(selected,fallback):
 if selected is not None:
  if not selected:raise SystemExit('Explicit Rust compiler is empty')
  resolved=shutil.which(selected)
  if not resolved:raise SystemExit('Explicit Rust compiler does not resolve: '+selected)
  return resolved
 return str(fallback)
def run_reference(rustc,source,tmp):
 library=tmp/'librustc_hash.rlib';exe=tmp/'table'
 subprocess.run([rustc,'--edition=2021','--crate-name=rustc_hash','--crate-type=rlib','--cfg=feature="std"',str(source/'src/lib.rs'),'-o',str(library)],check=True)
 env=dict(os.environ,RUSTC_BOOTSTRAP='1')
 subprocess.run([rustc,'--edition=2024',str(HERE/'oracle.rs'),'--extern','rustc_hash='+str(library),'-o',str(exe)],env=env,check=True)
 rawhash=hashlib.sha256();blockhash=hashlib.sha256();blocks=[];count=0
 child=subprocess.Popen([str(exe)],stdout=subprocess.PIPE)
 for line in child.stdout:
  rawhash.update(line);blockhash.update(line);count+=1
  if count%64==0:blocks.append(f'64\t{blockhash.hexdigest()}\n');blockhash=hashlib.sha256()
 if child.wait()!=0 or count!=96000:raise SystemExit('Original reference did not complete96,000 operations')
 if count%64:raise SystemExit('Partial proof block')
 return ''.join(blocks).encode(),rawhash.hexdigest(),count
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 git_env=dict(os.environ,GIT_NO_LAZY_FETCH='1')
 lock=tomllib.loads(subprocess.check_output(['git','-C',str(ROOT/'.toolchain/upstream-rust'),'show',PIN+':Cargo.lock'],env=git_env,text=True))
 package=next(x for x in lock['package'] if x['name']=='rustc-hash' and x['version']=='2.1.1')
 home=pathlib.Path(os.environ.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home')))
 archives=list((home/'registry/cache').glob('*/rustc-hash-2.1.1.crate'))
 if len(archives)!=1 or archives[0].is_symlink():raise SystemExit('Expected one real pinned rustc-hash archive')
 data=archives[0].read_bytes()
 if sha(data)!=package['checksum']:raise SystemExit('Pinned rustc-hash archive checksum differs')
 profiles=[];proof=None
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-table-')as temp:
  tmp=pathlib.Path(temp)
  with tarfile.open(fileobj=io.BytesIO(data))as archive:
   for member in archive.getmembers():
    path=pathlib.PurePosixPath(member.name)
    if path.is_absolute() or '..'in path.parts or not (member.isfile() or member.isdir()):raise SystemExit('Nonregular source archive member')
   archive.extractall(tmp,filter='data')
  src=tmp/'rustc-hash-2.1.1'
  for index,(name,default,expected)in enumerate(PROFILES):
   selected=os.environ.get('FERRUGO_RUSTC' if index==0 else 'FERRUGO_RUSTC_NIGHTLY')
   rustc=compiler(selected,default)
   actual=subprocess.check_output([rustc,'--version'],text=True).strip()
   if actual!=expected:raise SystemExit('Reference compiler differs: '+actual)
   dest=tmp/name;dest.mkdir()
   blocks,raw,count=run_reference(rustc,src,dest)
   if proof is not None and blocks!=proof:raise SystemExit('Actual std profiles differ; retain distinct fixtures before proceeding')
   proof=blocks
   profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'operations':count,'raw_output_sha256':raw,'blocks_sha256':sha(blocks),'hashbrown_version':'0.17.1','features':['default','std']})
 compressed=gzip.compress(proof,mtime=0)
 if FIXTURE.exists() and gzip.decompress(FIXTURE.read_bytes())==proof:compressed=FIXTURE.read_bytes()
 manifest={'scope':'Scoped actual std FxHashMap supporting table proof; Unord wholefile completion remains pending','rust_revision':PIN,'profiles':profiles,'blocks':len(proof.splitlines()),'operations_per_block':64,'fixture_sha256':sha(compressed),'fixture_raw_sha256':sha(proof),'rustc_hash_archive_sha256':package['checksum'],'rustc_hash_version':'2.1.1','oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes())}
 metadata=HERE/'manifest.json'
 if a.check:
  if json.loads(metadata.read_text())!=manifest or FIXTURE.read_bytes()!=compressed:raise SystemExit('Original table proof differs')
 else:
  FIXTURE.parent.mkdir(parents=True,exist_ok=True);FIXTURE.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:
  selected=os.environ.get('FERRUGO_GO');go=shutil.which('go'if selected is None else selected)
  if selected is not None and not go:raise SystemExit('FERRUGO_GO does not resolve')
  if not go:go=str(ROOT/'.toolchain/go/bin/go')
  subprocess.run([go,'test','./port/support/rusthashmap','-run','^TestOriginalHashMapTable$','-count=1'],cwd=ROOT,check=True)
 print('96,000 operations match both actual std profiles;1,500 SHA256 blocks')
if __name__=='__main__':main()
