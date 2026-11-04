#!/usr/bin/env python3
"""Retain cryptographic commitments to actual std stable-sort observations."""
import argparse,gzip,hashlib,json,os,pathlib,shutil,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
VERSION='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'
def sha(data):return hashlib.sha256(data).hexdigest()
def selected(variable,default):
 value=os.environ.get(variable)
 if value is None:return str(default)
 if not value or not shutil.which(value):raise SystemExit(variable+' is explicitly empty or unavailable')
 return shutil.which(value)
def go_compiler():
 explicit=os.environ.get('FERRUGO_GO')
 candidates=[selected('FERRUGO_GO','')]if explicit is not None else[shutil.which('go'),str(ROOT/'.toolchain/go/bin/go')]
 for candidate in candidates:
  if not candidate:continue
  result=subprocess.run([candidate,'version'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  if result.returncode==0 and result.stdout.startswith('go version go'):return candidate
 raise SystemExit('FERRUGO_GO or the default Go compiler is unavailable')
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 rustc=selected('FERRUGO_RUSTC_NIGHTLY',ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc')
 if subprocess.check_output([rustc,'--version'],text=True).strip()!=VERSION:raise SystemExit('Original sorting SDK differs')
 sysroot=pathlib.Path(subprocess.check_output([rustc,'--print','sysroot'],text=True).strip())
 host='x86_64-unknown-linux-gnu'
 libraries={}
 for crate in ['std','core','alloc']:
  paths=list((sysroot/'lib/rustlib'/host/'lib').glob('lib'+crate+'-*.rlib'))
  if len(paths)!=1 or paths[0].is_symlink():raise SystemExit('Expected one original SDK '+crate+' library')
  libraries[paths[0].name]=sha(paths[0].read_bytes())
 raw_native=hashlib.sha256();records=[];cases=callbacks=panics=0;metadata=set()
 with tempfile.TemporaryDirectory(prefix='ferrugo-std-sort-')as temporary:
  executable=str(pathlib.Path(temporary)/'sort')
  subprocess.run([rustc,'--edition=2024','-C','opt-level=1',str(HERE/'oracle.rs'),'-o',executable],check=True)
  child=subprocess.Popen([executable],stdout=subprocess.PIPE)
  try:
   for line in child.stdout:
    raw_native.update(line)
    f=line.rstrip(b'\n').decode('ascii').split('\t')
    if len(f)!=16:raise SystemExit('Invalid original sort observation')
    metadata.add(tuple(f[0:1]+f[6:10]));callbacks+=int(f[11]);panics+=f[10]=='true'
    for index in [12,13,14]:
     data=bytes.fromhex(f[index]);f[index]=str(len(data))+':'+sha(data)
    records.append(('\t'.join(f)+'\n').encode());cases+=1
  finally:
   if child.poll()is None:child.stdout.close()
  if child.wait()!=0:raise SystemExit('Original std sort failed')
 raw=b''.join(records)
 if (cases,callbacks,panics,len(metadata))!=(98720,7978963,89414,10):raise SystemExit('Original sort coverage differs')
 compressed=gzip.compress(raw,mtime=0)
 fixture=ROOT/'port/support/rustsort/testdata/stable.tsv.gz';fixture.parent.mkdir(parents=True,exist_ok=True)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 stable_files=['stable.go','shared.go','merge.go','drift.go']
 manifest={'scope':'Scoped actual std1.101 normal stable-sort dependency, not standard-library source-file completion; hashes commit to every comparison, post-panic slot and destructor event','compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'target':host,'sdk_libraries':libraries,'source_audit_sha256':sha((HERE/'SOURCES.json').read_bytes()),'cases':cases,'callbacks':callbacks,'panics':panics,'types':[list(v)for v in sorted(metadata)],'native_raw_sha256':raw_native.hexdigest(),'fixture_sha256':sha(compressed),'fixture_raw_sha256':sha(raw),'oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'go_sources':{name:sha((ROOT/'port/support/rustsort'/name).read_bytes())for name in stable_files}}
 path=HERE/'manifest.json'
 if a.check:
  if json.loads(path.read_text())!=manifest or fixture.read_bytes()!=compressed:raise SystemExit('Original stable-sort proof differs')
 else:fixture.write_bytes(compressed);path.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:
  go=go_compiler()
  subprocess.run([go,'test','./port/support/rustsort','-run','^TestOriginalStableSort$','-count=1'],cwd=ROOT,check=True)
 print(f'{cases:,} actual stable-sort requests; {callbacks:,} comparisons; {panics:,} caught panics; event/state/drop commitments retained'+(' and Go replay matches'if not a.skip_go else''))
if __name__=='__main__':main()
