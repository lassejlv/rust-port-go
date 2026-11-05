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
 patterns=[dict(length=n)for n in [0,1,2,4,21]]
 for key in ['next_fail','item_fail','hash_fail']:
  for cut in ([0,1]if key!='item_fail'else[100,101]):patterns.append(dict(length=4,**{key:cut}))
 patterns.extend([dict(length=4,call_fail=True),dict(length=4,input_fail=True),dict(length=4,next_fail=0,item_fail=100),dict(length=4,call_fail=True,input_fail=True),dict(length=4,hash_fail=0,item_fail=100),dict(length=0,input_fail=True)])
 patterns.extend([dict(length=4,nested=True),dict(length=4,nested=True,next_fail=1),dict(length=4,nested=True,hash_fail=0),dict(length=4,nested=True,input_fail=True),dict(length=0,nested=True),dict(length=4,nested=True,item_fail=100),dict(length=4,nested=True,call_fail=True)])
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-items-sort-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'conversion_oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for family in range(9):
    for pattern in patterns:
     for resume in [False,True]:
      request=dict(length=4,next_fail=-1,call_fail=False,item_fail=-1,input_fail=False,hash_fail=-1,resume=resume,nested=False);request.update(pattern)
      arguments=','.join(str(int(value))for value in [family,*[request[name]for name in ['length','next_fail','call_fail','item_fail','input_fail','hash_fail','resume','nested']]])
      result=subprocess.run([executable,mode,arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(environment,RUST_BACKTRACE='0'))
      if result.returncode not in [0,-6]:raise SystemExit('Conversion native outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
      stdout=result.stdout.decode()
      for line in stdout.splitlines():
       if not line.startswith('HOOK:'):continue
       f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location=dict(File=bytes.fromhex(f[6]).decode(),Line=int(f[7]),Column=int(f[8]))
       if key in locations and locations[key]!=location:raise SystemExit('Conversion source location varies:'+key)
       locations[key]=location
      rows.append(dict(mode=mode,family=family,request=request,outcome='abort'if result.returncode==-6 else'completed',native_exit_code=result.returncode,stdout=stdout))
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=1296:raise SystemExit('Conversion coverage differs:'+str(len(rows)))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_conversion.jsonl.gz';compressed=REFERENCE.compressed_fixture(raw,fixture,16<<20)
 names=['tools/port-rust-unord/items/conversion_oracle.rs','tools/port-rust-unord/items/generate_conversion.py']
 if not a.native_only:names+=['port/support/rusthashmap/extend_scoped.go','port/support/rustvec/scoped_consumer.go','port/support/rustvec/collect.go','port/rust/compiler/rustc_data_structures/src/unord_items_conversion.go','port/rust/compiler/rustc_data_structures/src/unord_items_conversion_test.go','port/rust/compiler/rustc_data_structures/src/unord_items_collections.go','port/rust/compiler/rustc_data_structures/src/unord_collection_marker.go','port/rust/compiler/rustc_data_structures/src/unord_items.go','port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go','port/rust/compiler/rustc_data_structures/src/unord_function.go']
 metadata=dict(scope='Actual unchanged Map/Set/Bag FromUnordItems and ExtendUnord, FromFxSet allocation transfer, generic and VecIntoIter strategies; no whole Unord/std credit',cases=len(rows),aborts=aborts,source_revision=profile['source_revision'],compiler=profile['rustc_version'],provenance_sha256=profile['provenance_sha256'],reference_inputs=REFERENCE.tool_inputs(),locations=locations,raw_sha256=sha(raw),fixture_sha256=sha(compressed),files={name:sha((ROOT/name).read_bytes())for name in names});destination=HERE/('conversion_native_manifest.json'if a.native_only else'conversion_manifest.json')
 if a.check:
  if destination.read_text()!=json.dumps(metadata,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Conversion proof differs')
 else:fixture.write_bytes(compressed);destination.write_text(json.dumps(metadata,indent=2)+'\n')
 if not a.native_only and not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordItemsConversion$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual collection-conversion cases/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
