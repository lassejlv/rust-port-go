#!/usr/bin/env python3
"""Actual ordinary UnordBag Extend with owned IntoIterator Self and Vec strategies."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('verified_reference',ROOT/'tools/port-rust-span-lib/reference.py');REFERENCE=importlib.util.module_from_spec(spec);spec.loader.exec_module(REFERENCE)
spec=importlib.util.spec_from_file_location('index_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');p.add_argument('--native-only',action='store_true');a=p.parse_args()
 profile,environment=REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
 patterns=[dict(length=n)for n in [0,1,2,4,21]]
 patterns.extend([dict(length=4,next_fail=n)for n in [0,1]]+[dict(length=4,item_fail=n)for n in [90,100,101]]+[dict(length=4,input_fail=True),dict(length=4,producer_fail=True),dict(length=4,into_fail=True),dict(length=4,into_fail=True,input_fail=True),dict(length=4,into_fail=True,producer_fail=True),dict(length=4,producer_fail=True,item_fail=100)])
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-bag-iterator-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for family in range(2):
    for variant in range(4):
     for pattern in patterns:
      for resume in [False,True]:
       request=dict(length=4,next_fail=-1,call_fail=False,item_fail=-1,input_fail=False,hash_fail=-1,resume=resume,nested=False,producer_fail=False,into_fail=False,forget=bool(variant&1),initial=bool(variant&2));request.update(pattern)
       arguments=','.join(str(int(value))for value in [family,*[request[name]for name in ['length','next_fail','call_fail','item_fail','input_fail','hash_fail','resume','nested','producer_fail','into_fail','forget','initial']]])
       result=subprocess.run([executable,mode,arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(environment,RUST_BACKTRACE='0'))
       if result.returncode not in [0,-6]:raise SystemExit('Bag-iterator native outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
       stdout=result.stdout.decode()
       for line in stdout.splitlines():
        if not line.startswith('HOOK:'):continue
        f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location=dict(File=bytes.fromhex(f[6]).decode(),Line=int(f[7]),Column=int(f[8]))
        if key in locations and locations[key]!=location:raise SystemExit('Bag-iterator source location varies:'+key)
        locations[key]=location
       rows.append(dict(mode=mode,family=family,request=request,outcome='abort'if result.returncode==-6 else'completed',native_exit_code=result.returncode,stdout=stdout))
 allocations=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-bag-iterator-allocation-')as temporary:
  executable=str(pathlib.Path(temporary)/'allocation');result=subprocess.run([profile['rustc'],str((HERE/'allocation.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for family in range(2):
   for length in [0,1,2,4,21]:
    for initial in [False,True]:
     result=subprocess.run([executable,str(family),str(length),str(int(initial))],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,check=True)
     row=json.loads(result.stdout);row['native_stdout']=result.stdout.decode();allocations.append(row)
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=768:raise SystemExit('Bag-iterator coverage differs:'+str(len(rows)))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_bag_iterator.jsonl.gz';compressed=REFERENCE.compressed_fixture(raw,fixture,16<<20)
 names=['tools/port-rust-unord/bag-iterator/allocation.rs','tools/port-rust-unord/bag-iterator/oracle.rs','tools/port-rust-unord/bag-iterator/generate.py']
 if not a.native_only:names+=['port/support/rustvec/ownership.go','port/rust/compiler/rustc_data_structures/src/unord_bag_iterator_owned.go','port/rust/compiler/rustc_data_structures/src/unord_bag_iterator_test.go','port/rust/compiler/rustc_data_structures/src/unord_items_conversion.go','port/rust/compiler/rustc_data_structures/src/unord_bag.go','port/rust/compiler/rustc_index/src/vec.go','port/rust/compiler/rustc_index/src/vec_context.go','port/rust/compiler/rustc_index/src/slice.go']
 metadata=dict(scope='Actual unchanged ordinary Bag Extend, explicitly owned source IntoIterator Self take/forget and prepared iterator leaks, generic/actual VecIntoIter strategies; no whole Unord/std credit',cases=len(rows),aborts=aborts,allocations=allocations,source_revision=profile['source_revision'],compiler=profile['rustc_version'],provenance_sha256=profile['provenance_sha256'],reference_inputs=REFERENCE.tool_inputs(),locations=locations,raw_sha256=sha(raw),fixture_sha256=sha(compressed),files={name:sha((ROOT/name).read_bytes())for name in names});destination=HERE/('native_manifest.json'if a.native_only else'manifest.json')
 if a.check:
  if destination.read_text()!=json.dumps(metadata,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Bag-iterator proof differs')
 else:fixture.write_bytes(compressed);destination.write_text(json.dumps(metadata,indent=2)+'\n')
 if not a.native_only and not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordBagIterator$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual owned Bag-iterator cases/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
