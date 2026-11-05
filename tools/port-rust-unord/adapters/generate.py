#!/usr/bin/env python3
"""Actual unchanged UnordItems default and overridden adapters, dispatch and ownership."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('verified_reference',ROOT/'tools/port-rust-span-lib/reference.py');REFERENCE=importlib.util.module_from_spec(spec);spec.loader.exec_module(REFERENCE)
spec=importlib.util.spec_from_file_location('index_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');p.add_argument('--native-only',action='store_true');a=p.parse_args()
 profile,environment=REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
 patterns=[dict(length=n)for n in [0,1,2,4,21]]
 patterns.extend([dict(length=4,next_fail=n)for n in [0,1]]+[dict(length=4,call_fail=n)for n in [0,1]]+[dict(length=4,item_fail=n)for n in [0,1,1000,100000]]+[dict(length=4,env_fail=True),dict(length=4,token_fail=True),dict(length=4,input_fail=True),dict(length=4,env_fail=True,token_fail=True),dict(length=4,call_fail=0,item_fail=0),dict(length=4,next_fail=0,input_fail=True),dict(length=4,inner=0),dict(length=4,inner=1),dict(length=4,keep=1),dict(length=4,item_fail=99),dict(length=4,keep=1,item_fail=99),dict(length=4,keep=1,item_fail=1099)])
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-adapters-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for family in range(11):
    for pattern in patterns:
     for resume in [False,True]:
      request=dict(length=4,inner=2,keep=0,next_fail=-1,call_fail=-1,item_fail=-1,env_fail=False,token_fail=False,input_fail=False,resume=resume);request.update(pattern)
      arguments=','.join(str(int(value))for value in [family,*[request[name]for name in ['length','inner','keep','next_fail','call_fail','item_fail','env_fail','token_fail','input_fail','resume']]])
      result=subprocess.run([executable,mode,arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(environment,RUST_BACKTRACE='0'))
      if result.returncode not in [0,-6]:raise SystemExit('Adapter native outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
      stdout=result.stdout.decode()
      for line in stdout.splitlines():
       if not line.startswith('HOOK:'):continue
       f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location=dict(File=bytes.fromhex(f[6]).decode(),Line=int(f[7]),Column=int(f[8]))
       if key in locations and locations[key]!=location:raise SystemExit('Adapter source location varies:'+key)
       locations[key]=location
      rows.append(dict(mode=mode,family=family,request=request,outcome='abort'if result.returncode==-6 else'completed',native_exit_code=result.returncode,stdout=stdout))
 bounds=[]
 for name,success in [('positive',True),('filter_not_fn',False),('filter_map_not_fn',False),('flat_map_not_fn',False),('chain_mismatch',False),('opaque_filter',False),('opaque_filter_map',False),('opaque_flat_map',False),('opaque_chain',False)]:
  with tempfile.TemporaryDirectory(prefix='ferrugo-adapter-bounds-')as temporary:
   result=subprocess.run([profile['rustc'],str((HERE/'bounds.rs').relative_to(ROOT)),*profile['link_arguments'],'--cfg',name,'-o',str(pathlib.Path(temporary)/'bound')],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
   if (result.returncode==0)!=success:raise SystemExit('Adapter bound differs:'+name+result.stderr.decode(errors='replace'))
   bounds.append(dict(configuration=name,compiles=success,diagnostic=result.stderr.decode()))
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=1650:raise SystemExit('Adapter coverage differs:'+str(len(rows)))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_adapter.jsonl.gz';compressed=REFERENCE.compressed_fixture(raw,fixture,16<<20)
 names=['tools/port-rust-unord/adapters/oracle.rs','tools/port-rust-unord/adapters/generate.py','tools/port-rust-unord/adapters/bounds.rs']
 if not a.native_only:names+=['port/rust/compiler/rustc_data_structures/src/unord_items_adapter_test.go','port/rust/compiler/rustc_data_structures/src/unord_function.go','port/support/rustvec/ownership.go','port/rust/compiler/rustc_data_structures/src/unord_items_collections.go','port/rust/compiler/rustc_data_structures/src/unord_collection_marker.go','port/rust/compiler/rustc_data_structures/src/unord_items_delegate.go','port/rust/compiler/rustc_data_structures/src/unord_items.go','port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go']
 metadata=dict(scope='Actual unchanged default and overridden UnordItems filter/filter_map/flat_map/chain, selected FnMut/shared/FnOnce, independent I fold/count and complete adapter ownership; independent native and Go bindings; no std/whole Unord credit',cases=len(rows),aborts=aborts,bounds=bounds,source_revision=profile['source_revision'],compiler=profile['rustc_version'],provenance_sha256=profile['provenance_sha256'],reference_inputs=REFERENCE.tool_inputs(),locations=locations,raw_sha256=sha(raw),fixture_sha256=sha(compressed),files={name:sha((ROOT/name).read_bytes())for name in names});destination=HERE/('native_manifest.json'if a.native_only else'manifest.json')
 if a.check:
  if destination.read_text()!=json.dumps(metadata,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Adapter proof differs')
 else:fixture.write_bytes(compressed);destination.write_text(json.dumps(metadata,indent=2)+'\n')
 if not a.native_only and not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordItemsAdapter$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual default-adapter cases/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
