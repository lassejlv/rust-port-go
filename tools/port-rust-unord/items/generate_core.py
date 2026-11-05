#!/usr/bin/env python3
"""Actual UnordItems primitive ownership/dispatch and nonfused next behavior."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('index_context_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
spec=importlib.util.spec_from_file_location('verified_span_reference',ROOT/'tools/port-rust-span-lib/reference.py');REFERENCE=importlib.util.module_from_spec(spec);spec.loader.exec_module(REFERENCE)
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args();profile,environment=REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
 if profile["source_revision"]!="2d20b42b883dcfc014f3c2c56081eeb6091c2179"or profile["rustc_version"]!="rustc 1.101.0-nightly (708c94ef0 2026-11-03)"or profile["upstream_tests_passed"]!=43:raise SystemExit("Core source/profile differs")
 if subprocess.check_output([profile["rustc"],"--version"],text=True).strip()!=profile["rustc_version"]or not profile["debug_assertions"]or not profile["overflow_checks"]:raise SystemExit("Core compiled profile differs")
 patterns=[(0,0,-1,-1,False),(1,0,-1,-1,False),(2,0,-1,-1,False),(4,0,-1,-1,False),(0,1,-1,-1,False),(4,0,0,-1,False),(4,0,1,-1,False),(4,0,-1,0,False),(4,0,-1,1,False),(4,0,-1,1,True),(4,0,1,0,False),(4,0,1,-1,True),(0,0,-1,-1,True)]
 requests=[(0,pattern,resume)for pattern in patterns for resume in [False,True]]+[(1,p,resume)for p in [(4,0,-1,-1,False),(4,0,-1,-1,True),(0,0,-1,-1,False)]for resume in [False,True]]+[(2,(4,0,-1,-1,False),False),(3,(0,0,-1,-1,False),False)]+[(4,p,resume)for p in [(4,0,-1,-1,False),(4,0,-1,-1,True),(4,0,0,-1,False),(4,0,0,-1,True)]for resume in [False,True]]
 rows=[];locations={};bounds=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-core-')as temporary:
  for option in [None,'no_item_clone','opaque_map_clone','opaque_filter_clone']:
   command=[profile['rustc'],str((HERE/'clone_bounds.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',str(pathlib.Path(temporary)/'bounds')]
   if option is not None:command+=['--cfg',option]
   proof=subprocess.run(command,cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
   if option is None:
    if proof.returncode:raise SystemExit(proof.stderr.decode(errors='replace'))
    subprocess.run([str(pathlib.Path(temporary)/'bounds')],check=True)
   elif proof.returncode!=1 or b'error[E0599]'not in proof.stderr or b'Clone'not in proof.stderr:raise SystemExit('Original Clone bound differs:'+str(option))
   bounds.append({'configuration':option or'positive','compiles':proof.returncode==0,'stderr_sha256':sha(proof.stderr)})
  executable=str(pathlib.Path(temporary)/'oracle');r=subprocess.run([profile['rustc'],str((HERE/'core_oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if r.returncode:raise SystemExit(r.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for action,(length,pattern,next_fail,item_fail,iterator_fail),resume in requests:
    arguments=','.join(str(int(v))for v in [length,pattern,next_fail,item_fail,iterator_fail,resume]);r=subprocess.run([executable,mode,str(action),arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(environment,RUST_BACKTRACE='0'))
    if r.returncode not in [0,-6]:raise SystemExit('Core native outcome differs:'+str(r.returncode)+r.stderr.decode(errors='replace'))
    stdout=r.stdout.decode()
    for line in stdout.splitlines():
     if not line.startswith('HOOK:'):continue
     f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
     if key in locations and locations[key]!=location:raise SystemExit('Core location varies:'+key)
     locations[key]=location
    rows.append({'mode':mode,'action':action,'length':length,'pattern':pattern,'next_fail':next_fail,'item_fail':item_fail,'iterator_fail':iterator_fail,'resume':resume,'outcome':'abort'if r.returncode==-6 else'completed','native_exit_code':r.returncode,'stdout':stdout})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=126:raise SystemExit('Core coverage differs')
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_items_core.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 names=['port/rust/compiler/rustc_data_structures/src/unord_items_core_test.go','port/rust/compiler/rustc_data_structures/src/unord_items_boolean.go','port/rust/compiler/rustc_data_structures/src/unord_items.go','port/rust/compiler/rustc_data_structures/src/unord_bound_primaries.go','tools/port-rust-unord/items/clone_bounds.rs','tools/port-rust-unord/items/core_oracle.rs','tools/port-rust-unord/items/generate_core.py','port/rust/compiler/rustc_data_structures/src/unord_items_context_test.go','tools/port-rust-unord/items/README.md','tools/port-rust-unord/items/test_tools.py']
 manifest={'scope':'Original UnordItems new/empty/Clone/count/get_only ownership primitives, selected count override, nonfused next and failures; generic algebra still pending','cases':len(rows),'aborts':aborts,'clone_bounds':bounds,'compiler':profile['rustc_version'],'source_revision':profile['source_revision'],'provenance_sha256':profile['provenance_sha256'],'reference_inputs':REFERENCE.tool_inputs(),'locations':locations,'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{p:sha((ROOT/p).read_bytes())for p in names}};metadata=HERE/'core_manifest.json'
 if a.check:
  if metadata.read_text()!=json.dumps(manifest,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Core proof differs')
 else:fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordItemsCore$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual Items primitive processes/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
