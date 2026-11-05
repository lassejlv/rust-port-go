#!/usr/bin/env python3
"""Actual std guard/hook/resume/cleanup observer. No expected values from Go."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('sort_tools',HERE.parent/'sort/generate.py')
SDK=importlib.util.module_from_spec(spec);spec.loader.exec_module(SDK)
def sha(data):return hashlib.sha256(data).hexdigest()
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
ACTIONS=['map_hash','map_eq','map_entry','map_capacity','map_hasher_double','map_hash_value_double','map_hash_key_double','vec_next','vec_capacity','vec_iter_drop','vec_iter_drop_double','sort_compare','sort_invalid']
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 rustc=SDK.selected('FERRUGO_RUSTC_NIGHTLY',ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc')
 if subprocess.check_output([rustc,'--version'],text=True).strip()!=SDK.VERSION:raise SystemExit('Guard proof SDK differs')
 sysroot=pathlib.Path(subprocess.check_output([rustc,'--print','sysroot'],text=True).strip())
 libraries={}
 for crate in ['std','core','alloc']:
  paths=list((sysroot/'lib/rustlib/x86_64-unknown-linux-gnu/lib').glob('lib'+crate+'-*.rlib'))
  if len(paths)!=1 or paths[0].is_symlink():raise SystemExit('Expected one original SDK '+crate+' library')
  libraries[paths[0].name]=sha(paths[0].read_bytes())
 rows=[];sites={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-guard-panic-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle')
  # Keep the authored source argument lexical/relative; locations must be
  # portable and verbatim, rather than normalized after observing them.
  source=str((HERE/'oracle.rs').relative_to(ROOT))
  subprocess.run([rustc,'--edition=2024','-C','opt-level=1',source,'-o',executable],cwd=ROOT,check=True)
  for mode in ['normal','unwind','processing']:
   for action in ACTIONS:
    for resume in [False,True]:
     for hook in ['return','panic','resume','drop_panic']:
      result=subprocess.run([executable,mode,action,str(resume).lower(),hook],stdout=subprocess.PIPE,stderr=subprocess.PIPE,preexec_fn=no_core,timeout=10,env=dict(os.environ,RUST_BACKTRACE='0'))
      if result.returncode not in [0,-6]:raise SystemExit(f'Unexpected original guard exit {result.returncode}: {result.stderr.decode(errors="replace")}')
      stdout=result.stdout.decode()
      for line in stdout.splitlines():
       if not line.startswith('HOOK:'):continue
       f=line.split(':')
       if len(f)!=9:raise SystemExit('Original hook protocol differs')
       payload=bytes.fromhex(f[2]).decode()
       key=payload
       if payload=='outer':key='outer.'+mode
       location={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
       if key in sites and sites[key]!=location:raise SystemExit('Selected source operation locations vary:'+key)
       sites[key]=location
      rows.append({'mode':mode,'action':action,'resume':resume,'hook':hook,'outcome':'completed'if result.returncode==0 else'abort','native_exit_code':result.returncode,'stdout':stdout})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows)
 aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=312 or aborts!=258:raise SystemExit('Guard proof coverage differs')
 fixture=ROOT/'port/support/rusthashmap/testdata/panic_context.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 production=['port/support/rustfmt/formatter.go','port/support/rustfmt/panic.go']
 for package in ['rusthashmap','rustvec','rustsort']:
  production+=['port/support/'+package+'/'+name for name in {'rusthashmap':['entry.go','extend.go','map.go','map_traits.go','once.go','ownership.go','raw.go','raw_clone.go','panic_sites.go'],'rustvec':['ownership.go','collect.go','for_each.go','panic_sites.go'],'rustsort':['stable.go','shared.go','panic_sites.go']}[package]]
 manifest={'scope':'Actual std callback hook-once/resume/cleanup-abort and intrinsic capacity/order violations across scoped Map/Vec/Stable-sort dependencies; no std source-file completion credit','compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'sdk_libraries':libraries,'cases':len(rows),'aborts':aborts,'locations':sites,'fixture_sha256':sha(compressed),'raw_sha256':sha(raw),'oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'sdk_helper_sha256':sha((HERE.parent/'sort/generate.py').read_bytes()),'go_sources':{name:sha((ROOT/name).read_bytes())for name in production}}
 path=HERE/'manifest.json'
 if a.check:
  if json.loads(path.read_text())!=manifest or fixture.read_bytes()!=compressed:raise SystemExit('Original guard panic proof differs')
 else:fixture.write_bytes(compressed);path.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([SDK.go_compiler(),'test','./port/support/rusthashmap','-run','^TestOriginalPanicGuards$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual guard/hook processes; {aborts} actual SIGABRT prefixes')
if __name__=='__main__':main()
