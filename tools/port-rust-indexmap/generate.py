#!/usr/bin/env python3
"""Stateful comparison of supporting operations with the unchanged locked crate."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,random,resource,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def digest(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}: exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 rng=random.Random(0x1de2ab);qs=[]
 names=['entry','entry_or_insert','entry_or_default','entry_with','entry_with_key','entry_modify','entry_insert','occupied_replace','entry_swap_remove','entry_shift_remove','vacant_insert','insert','get','equivalent','swap_remove','shift_remove','swap_index','shift_index','index','reserve','reserve_exact','reverse','pop','clear','clone','equal_clone','clone_from','debug'];weights=[3,6,2,3,3,3,6,3,3,3,3,35,8,8,5,5,2,2,2,2,2,2,3,1,3,2,3,1]
 for capacity in [0,1,2,3,4,7,8,15,16,40]:
  for mask in [0,15,(1<<64)-1]:
   for seed in [0,7]:
    ops=[]
    for n in range(300):
     op=rng.choices(names,weights)[0];key=rng.randrange(80)
     if n%7==0:key|=1<<63
     ops.append(dict(op=op,key=key,value=rng.randrange(-10000,10000),n=rng.randrange(30)))
    qs.append(dict(capacity=capacity,mask=mask,seed=seed,ops=ops))
 return qs
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--check',action='store_true');ap.add_argument('--offline',action='store_true');args=ap.parse_args();root=args.root.resolve();env=os.environ.copy();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build'
 spec=importlib.util.spec_from_file_location('indexmap_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1'
 run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps'
 native=cache/'indexmap-rust';subject=cache/'indexmap-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here/'oracle.rs').relative_to(root)),'-L','dependency='+str(deps)]
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/replay'],env,root)
 qs=requests();data=(''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs)).encode();raw=run([native],env,root,data);go=run([subject],env,root,data)
 rustrows=list(map(json.loads,raw.splitlines()));gorows=list(map(json.loads,go.splitlines()))
 if len(rustrows)!=len(qs)or len(gorows)!=len(qs):raise RuntimeError('record count differs')
 for n,(rust,actual)in enumerate(zip(rustrows,gorows)):
  if rust!=actual:raise RuntimeError('IndexMap mismatch case'+str(n))
 fixture=root/'port/support/rustindexmap/testdata/core.jsonl.gz';compressed=None
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 if compressed is None:compressed=gzip.compress(raw,mtime=0,compresslevel=9)
 dependencies=[p for p in sorted(builder.glob('*'))if p.is_file()]+[root/'tools/port-rust-span-encoding/prepare.py']
 owned=[here/'generate.py',here/'oracle.rs',here/'source-manifest.json',*sorted((here/'subject').glob('*.go')),*sorted((here/'replay').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go')),root/'port/support/rusthashmap/hasher_borrow.go',root/'port/support/rusthashmap/hasher_borrow_test.go',root/'port/support/rusthashmap/raw_unique.go']
 metadata={'status':'partial-supporting-IndexMap-core','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'cases':len(qs),'operations':sum(len(q['ops'])for q in qs),'request_sha256':digest(data),'raw_sha256':digest(raw),'fixture_sha256':digest(compressed),'scope':'Stored-hash dense vector plus genuine SwissTable indices; default hash_one callbacks and owned hasher cleanup, Equivalent callbacks, capacities, mutation, Clone/CloneFrom and Debug. Implemented Entry transitions are included; Entry formatting and additional inherited APIs are outside this corpus. Nontrivial ownership proof is retained separately.','files':[{'path':str(p.relative_to(root)),'sha256':digest(p.read_bytes())}for p in [*owned,*dependencies]]}
 encoded=(json.dumps(metadata,indent=2)+'\n').encode();manifest=here/'validation.json'
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('IndexMap core evidence differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','./port/support/rusthashmap','-run','^(TestOriginalIndexMapCore|TestOwnedHasherBorrowUsesActualStateAndRejectsConsumedOwner)$','-count=1'],env,root)
 print(f'IndexMap supporting core: {len(qs)} original cases / {metadata["operations"]} stateful operations passed; no whole dependency credit')
if __name__=='__main__':main()
