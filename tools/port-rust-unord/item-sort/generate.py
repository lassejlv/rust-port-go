#!/usr/bin/env python3
"""Actual whole-original generic UnordItems collection/sorting ownership."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('verified_reference',ROOT/'tools/port-rust-span-lib/reference.py');REFERENCE=importlib.util.module_from_spec(spec);spec.loader.exec_module(REFERENCE)
spec=importlib.util.spec_from_file_location('index_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');p.add_argument('--native-only',action='store_true');a=p.parse_args()
 profile,environment=REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
 patterns=[]
 for length in [0,1,2,4,21]:patterns.append(dict(length=length))
 for key in ['key_fail','cmp_fail','key_drop_fail','item_drop_fail']:
  for cut in [0,1]:patterns.append(dict(length=4,**{key:cut}))
 for key in ['input_drop_fail','borrow_fail','collection_drop_fail','project_drop_fail','token_fail']:patterns.append(dict(length=4,**{key:True}))
 patterns.extend([dict(length=4,borrow_fail=True,collection_drop_fail=True),dict(length=4,cmp_fail=0,item_drop_fail=0),dict(length=4,key_fail=0,item_drop_fail=0),dict(length=4,project_drop_fail=True,token_fail=True),dict(length=0,borrow_fail=True),dict(length=0,project_drop_fail=True)])
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-items-sort-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for unstable in [False,True]:
    for action in [0,1,2]:
     for pattern in patterns:
      for resume in [False,True]:
       request=dict(length=4,action=action,key_fail=-1,cmp_fail=-1,key_drop_fail=-1,item_drop_fail=-1,input_drop_fail=False,borrow_fail=False,collection_drop_fail=False,project_drop_fail=False,token_fail=False,resume=resume);request.update(pattern)
       arguments=','.join(str(int(request[name]))for name in ['length','action','key_fail','cmp_fail','key_drop_fail','item_drop_fail','input_drop_fail','borrow_fail','collection_drop_fail','project_drop_fail','token_fail','resume'])
       result=subprocess.run([executable,mode,str(unstable).lower(),arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(environment,RUST_BACKTRACE='0'))
       if result.returncode not in [0,-6]:raise SystemExit('Item sort native outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
       stdout=result.stdout.decode()
       for line in stdout.splitlines():
        if not line.startswith('HOOK:'):continue
        f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location=dict(File=bytes.fromhex(f[6]).decode(),Line=int(f[7]),Column=int(f[8]))
        if key in locations and locations[key]!=location:raise SystemExit('Item sort source location varies:'+key)
        locations[key]=location
       rows.append(dict(mode=mode,unstable=unstable,request=request,outcome='abort'if result.returncode==-6 else'completed',native_exit_code=result.returncode,stdout=stdout))
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=864:raise SystemExit('Item sort coverage differs:'+str(len(rows)))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_sort.jsonl.gz';compressed=REFERENCE.compressed_fixture(raw,fixture,16<<20)
 names=['tools/port-rust-unord/item-sort/oracle.rs','tools/port-rust-unord/item-sort/generate.py']
 if not a.native_only:names+=['port/rust/compiler/rustc_data_structures/src/unord_items_sort.go','port/rust/compiler/rustc_data_structures/src/unord_items_sort_test.go','port/rust/compiler/rustc_data_structures/src/unord_items.go','port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go','port/rust/compiler/rustc_data_structures/src/unord_function.go','port/rust/compiler/rustc_data_structures/src/unord_items_boolean.go','port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go']
 metadata=dict(scope='Actual unchanged generic UnordItems collect_sorted/collect_stable_ord_by_key with real C BorrowMut/Drop, I.collect override and owned projection; no full Unord/Vec/std credit yet',cases=len(rows),aborts=aborts,source_revision=profile['source_revision'],compiler=profile['rustc_version'],provenance_sha256=profile['provenance_sha256'],reference_inputs=REFERENCE.tool_inputs(),locations=locations,raw_sha256=sha(raw),fixture_sha256=sha(compressed),files={name:sha((ROOT/name).read_bytes())for name in names});destination=HERE/('native_manifest.json'if a.native_only else'manifest.json')
 if a.check:
  if destination.read_text()!=json.dumps(metadata,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Item sort proof differs')
 else:fixture.write_bytes(compressed);destination.write_text(json.dumps(metadata,indent=2)+'\n')
 if not a.native_only and not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordItemsSort$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual generic item-sort cases/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
