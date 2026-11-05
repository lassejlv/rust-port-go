#!/usr/bin/env python3
"""Actual SDK cached-key/owned FnMut/temporary Drop proof, never Go expectations."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
SPEC=importlib.util.spec_from_file_location('sort_tools',HERE.parent/'sort/generate.py')
SORT=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(SORT)
def sha(data):return hashlib.sha256(data).hexdigest()
def child_specs():
 base={'kind':1,'chain':False,'length':9,'seed':1,'key_fail':-1,'cmp_fail':-1,'drop_fail':-1,'fn_fail':False,'token_fail':False,'moved':False}
 result=[]
 def add(**changes):result.append(dict(base,**changes))
 for changes in [{},{'fn_fail':True},{'token_fail':True},{'fn_fail':True,'token_fail':True}]:add(length=0,**changes)
 add(length=1);add(length=1,fn_fail=True)
 for changes in [{},{'moved':True},{'key_fail':0},{'key_fail':4},{'cmp_fail':0},{'cmp_fail':2},{'drop_fail':0},{'drop_fail':8},{'fn_fail':True},{'token_fail':True},{'moved':True,'key_fail':0},{'key_fail':0,'fn_fail':True},{'cmp_fail':0,'drop_fail':0},{'fn_fail':True,'token_fail':True},{'token_fail':True,'drop_fail':0}]:add(**changes)
 for changes in [{'cmp_fail':2},{'key_fail':4},{'drop_fail':0},{'fn_fail':True},{'moved':True}]:add(kind=2,length=33,**changes)
 for changes in [{'drop_fail':0},{},{'cmp_fail':0}]:add(kind=3,**changes)
 for changes in [{},{'cmp_fail':0},{'key_fail':2},{'fn_fail':True}]:add(kind=0,length=33,chain=True,**changes)
 assert len(result)==33
 return result
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 rustc=SORT.selected('FERRUGO_RUSTC_NIGHTLY',ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc')
 if subprocess.check_output([rustc,'--version'],text=True).strip()!=SORT.VERSION:raise SystemExit('Original cached-key SDK differs')
 sysroot=pathlib.Path(subprocess.check_output([rustc,'--print','sysroot'],text=True).strip())
 libraries={}
 for crate in ['std','core','alloc']:
  paths=list((sysroot/'lib/rustlib/x86_64-unknown-linux-gnu/lib').glob('lib'+crate+'-*.rlib'))
  if len(paths)!=1 or paths[0].is_symlink():raise SystemExit('Expected one original SDK '+crate+' library')
  libraries[paths[0].name]=sha(paths[0].read_bytes())
 with tempfile.TemporaryDirectory(prefix='ferrugo-cached-key-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle')
  subprocess.run([rustc,'--edition=2024','-C','opt-level=1',str(HERE/'oracle.rs'),'-o',executable],check=True)
  raw=subprocess.check_output([executable])
  child_rows=[]
  keys=['kind','chain','length','seed','key_fail','cmp_fail','drop_fail','fn_fail','token_fail','moved']
  for context in ['normal','unwind','processing']:
   for request in child_specs():
    arguments=','.join(str(int(request[key]))for key in keys)
    child=subprocess.run([executable,'--child',context,arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=no_core,env=dict(os.environ,RUST_BACKTRACE='0'))
    if child.returncode not in [0,-6]:raise SystemExit('Original cached-key child failed unexpectedly:'+str(child.returncode)+child.stderr.decode(errors='replace'))
    child_rows.append({'context':context,'request':request,'outcome':'completed'if child.returncode==0 else'abort','native_exit_code':child.returncode,'stdout':child.stdout.decode()})
 child_raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in child_rows)
 aborts=sum(row['outcome']=='abort'for row in child_rows)
 if len(child_rows)!=99 or aborts!=36:raise SystemExit('Original cached-key child coverage differs:'+str(aborts))
 counts={'cases':0,'compares':0,'key_drops':0,'function_calls':0,'function_drops':0,'token_drops':0,'moves':0,'partial_cmp':0,'chaining_less':0,'other_comparisons':0}
 for line in raw.splitlines():
  fields=line.decode().split('\t')
  if len(fields)!=15:raise SystemExit('Original cached-key protocol differs')
  counts['cases']+=1;counts['compares']+=int(fields[11]);counts['key_drops']+=int(fields[12])
  for event in bytes.fromhex(fields[13]).decode().splitlines():
   prefix=event.split(':')[0]
   target={'F':'function_calls','FD':'function_drops','TD':'token_drops','move':'moves','P':'partial_cmp','C':'chaining_less','EQ':'other_comparisons','LT':'other_comparisons','ORD':'other_comparisons'}.get(prefix)
   if target:counts[target]+=1
 expected={'cases':5768,'compares':121472,'key_drops':81600,'function_calls':121352,'function_drops':5768,'token_drops':5768,'moves':144,'partial_cmp':60736,'chaining_less':60736,'other_comparisons':0}
 if counts!=expected:raise SystemExit('Original cached-key coverage differs:'+str(counts))
 fixture=ROOT/'port/support/rustsort/testdata/cached.tsv.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 child_fixture=ROOT/'port/support/rustsort/testdata/cached_context.jsonl.gz';child_compressed=gzip.compress(child_raw,mtime=0)
 if child_fixture.exists()and gzip.decompress(child_fixture.read_bytes())==child_raw:child_compressed=child_fixture.read_bytes()
 names=['cached.go','key_function.go','unstable.go','network.go','stable.go','shared.go']
 manifest={'scope':'Actual std1.101 cached-key owned FnMut/tuple PartialOrd/temporary destruction/permutation; supporting dependency, no standard-library file completion credit','compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'sdk_libraries':libraries,'counts':counts,'fixture_sha256':sha(compressed),'raw_sha256':sha(raw),'children':{'cases':len(child_rows),'aborts':aborts,'fixture_sha256':sha(child_compressed),'raw_sha256':sha(child_raw)},'oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'shared_generator_sha256':sha((HERE.parent/'sort/generate.py').read_bytes()),'go_sources':{name:sha((ROOT/'port/support/rustsort'/name).read_bytes())for name in names},'source_audit_sha256':sha((HERE/'SOURCES.json').read_bytes())}
 path=HERE/'manifest.json'
 if a.check:
  if json.loads(path.read_text())!=manifest or fixture.read_bytes()!=compressed or child_fixture.read_bytes()!=child_compressed:raise SystemExit('Original cached-key proof differs')
 else:fixture.write_bytes(compressed);child_fixture.write_bytes(child_compressed);path.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([SORT.go_compiler(),'test','./port/support/rustsort','-run','^TestOriginalCached(Sort|Context)$','-count=1'],cwd=ROOT,check=True)
 print('Actual cached-key records: '+str(counts)+f'; {len(child_rows)} context children/{aborts} actual aborts')
if __name__=='__main__':main()
