#!/usr/bin/env python3
"""Whole real cached-key operations with ordinary hooks/resume/field cleanup."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('cached_tools',HERE/'generate.py')
CACHED=importlib.util.module_from_spec(spec);spec.loader.exec_module(CACHED)
def sha(data):return hashlib.sha256(data).hexdigest()
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 rustc=CACHED.SORT.selected('FERRUGO_RUSTC_NIGHTLY',ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc')
 if subprocess.check_output([rustc,'--version'],text=True).strip()!=CACHED.SORT.VERSION:raise SystemExit('Cached hook SDK differs')
 sysroot=pathlib.Path(subprocess.check_output([rustc,'--print','sysroot'],text=True).strip())
 libraries={}
 for crate in ['std','core','alloc']:
  paths=list((sysroot/'lib/rustlib/x86_64-unknown-linux-gnu/lib').glob('lib'+crate+'-*.rlib'))
  if len(paths)!=1 or paths[0].is_symlink():raise SystemExit('Expected one original SDK '+crate+' library')
  libraries[paths[0].name]=sha(paths[0].read_bytes())
 rows=[];locations={};keys=['kind','chain','length','seed','key_fail','cmp_fail','drop_fail','fn_fail','token_fail','moved']
 with tempfile.TemporaryDirectory(prefix='ferrugo-cached-hooks-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle')
  subprocess.run([rustc,'--edition=2024','-C','opt-level=1',str((HERE/'hooks.rs').relative_to(ROOT)),'-o',executable],cwd=ROOT,check=True)
  for context in ['normal','unwind','processing']:
   for request in CACHED.child_specs():
    for resume in [False,True]:
     arguments=','.join(str(int(request[key]))for key in keys)
     result=subprocess.run([executable,context,arguments,str(resume).lower()],stdout=subprocess.PIPE,stderr=subprocess.PIPE,preexec_fn=no_core,timeout=10,env=dict(os.environ,RUST_BACKTRACE='0'))
     if result.returncode not in [0,-6]:raise SystemExit('Unexpected cached hook exit:'+str(result.returncode)+result.stderr.decode(errors='replace'))
     stdout=result.stdout.decode()
     for line in stdout.splitlines():
      if not line.startswith('HOOK:'):continue
      f=line.split(':')
      if len(f)!=9:raise SystemExit('Cached hook protocol differs')
      payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+context
      location={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
      if key in locations and locations[key]!=location:raise SystemExit('Cached hook operation locations vary:'+key)
      locations[key]=location
     rows.append({'context':context,'request':request,'resume':resume,'outcome':'completed'if result.returncode==0 else'abort','native_exit_code':result.returncode,'stdout':stdout})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows)
 aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=198 or aborts!=72:raise SystemExit('Cached hook coverage differs:'+str(aborts))
 fixture=ROOT/'port/support/rustsort/testdata/cached_hooks.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 files=['cached.go','key_function.go','unstable.go','network.go','stable.go','shared.go','panic_sites.go']
 manifest={'scope':'Actual std cached-key ownedFnMut ordinary-hook-once/resume/complete field and element cleanup; no standard-library file credit','compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'sdk_libraries':libraries,'cases':len(rows),'aborts':aborts,'locations':locations,'fixture_sha256':sha(compressed),'raw_sha256':sha(raw),'reference_sources':{name:sha((HERE/name).read_bytes())for name in ['hooks.rs','oracle.rs','generate.py','generate_hooks.py']},'sdk_selector_sha256':sha((HERE.parent/'sort/generate.py').read_bytes()),'go_sources':{name:sha((ROOT/'port/support/rustsort'/name).read_bytes())for name in files}}
 metadata=HERE/'hooks_manifest.json'
 if a.check:
  if json.loads(metadata.read_text())!=manifest or fixture.read_bytes()!=compressed:raise SystemExit('Cached hook proof differs')
 else:fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([CACHED.SORT.go_compiler(),'test','./port/support/rustsort','-run','^TestOriginalCachedHooks$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual cached-key hook processes/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
