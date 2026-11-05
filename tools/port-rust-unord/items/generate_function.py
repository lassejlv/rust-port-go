#!/usr/bin/env python3
"""Actual Fn trait dispatch, parameter destruction and prepared return places."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('sort_tools',HERE.parent/'sort/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(b):return hashlib.sha256(b).hexdigest()
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args();rustc=TOOLS.selected('FERRUGO_RUSTC_NIGHTLY',ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc')
 if subprocess.check_output([rustc,'--version'],text=True).strip()!=TOOLS.VERSION:raise SystemExit('Fn SDK differs')
 sysroot=pathlib.Path(subprocess.check_output([rustc,'--print','sysroot'],text=True).strip());libraries={}
 for crate in ['core','alloc','std']:
  paths=list((sysroot/'lib/rustlib/x86_64-unknown-linux-gnu/lib').glob('lib'+crate+'-*.rlib'))
  if len(paths)!=1 or paths[0].is_symlink():raise SystemExit('Expected one exact SDK library')
  libraries[paths[0].name]=sha(paths[0].read_bytes())
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-fn-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([rustc,'--edition=2024',str((HERE/'function_oracle.rs').relative_to(ROOT)),'-o',executable],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for kind in range(4):
    for flags in (range(8)if kind<3 else[0]):
     for resume in ([False,True]if kind<3 else[False]):
      result=subprocess.run([executable,mode,str(kind),str(flags),str(resume).lower()],stdout=subprocess.PIPE,stderr=subprocess.PIPE,preexec_fn=no_core,timeout=10,env=dict(os.environ,RUST_BACKTRACE='0'))
      if result.returncode not in [0,-6]:raise SystemExit('Fn native outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
      stdout=result.stdout.decode()
      for line in stdout.splitlines():
       if not line.startswith('HOOK:'):continue
       f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
       if key in locations and locations[key]!=location:raise SystemExit('Fn location varies:'+key)
       locations[key]=location
      rows.append({'mode':mode,'kind':kind,'flags':flags,'resume':resume,'outcome':'abort'if result.returncode==-6 else'completed','native_exit_code':result.returncode,'stdout':stdout})
 aborts=sum(row['outcome']=='abort'for row in rows);raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows)
 if len(rows)!=147:raise SystemExit('Fn coverage differs')
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_function.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 files=['port/rust/compiler/rustc_data_structures/src/unord_function.go','port/rust/compiler/rustc_data_structures/src/unord_function_test.go','tools/port-rust-unord/items/function_oracle.rs','tools/port-rust-unord/items/generate_function.py','port/rust/compiler/rustc_data_structures/src/unord_items_context_test.go','tools/port-rust-unord/items/README.md','tools/port-rust-unord/items/test_tools.py']
 manifest={'scope':'Actual three Fn traits, actual env/argument cells, complete parameter and field destruction, prepared-result leaks and contexts; supporting machinery only','cases':len(rows),'aborts':aborts,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'sdk_libraries':libraries,'locations':locations,'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{p:sha((ROOT/p).read_bytes())for p in files}}
 path=HERE/'function_manifest.json'
 if a.check:
  if path.read_text()!=json.dumps(manifest,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Fn proof differs')
 else:fixture.write_bytes(compressed);path.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordFunction$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual Fn trait processes/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
