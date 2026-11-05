#!/usr/bin/env python3
"""Direct original StableCrateIdMap alias comparison, without source credit."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,random,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 rng=random.Random(0xdef1da1);qs=[];ops=['insert','get','entry','or_insert','with','modify','insert_entry','swap_remove','shift_remove','swap_index','shift_index','reverse','clear','pop','reserve','reserve_exact','clone','clone_from','hash_one','debug','truncate','drain','drain_count','get_range'];weights=[20,4,3,5,4,3,5,3,3,1,1,1,1,1,1,1,2,2,3,2,1,1,1,2]
 for capacity in [0,1,2,3,7,8,15,16,40,63,64,100]:
  for high in [False,True]:
   rows=[]
   for _ in range(400):
    key=rng.randrange(100)
    if high:key=(key<<57)|((1<<53)+1)
    rows.append(dict(op=rng.choices(ops,weights)[0],key=key,value=rng.randrange(1000),n=rng.randrange(40)))
   qs.append(dict(capacity=capacity,ops=rows))
 return qs
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-def-id/indexmap-alias';builder=here.parent/'reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('defid_alias_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'def-id-indexmap-alias';subject=cache/'def-id-indexmap-alias-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here.parent/'indexmap_alias_oracle.rs').relative_to(root)),*profile['link_arguments']]
 for name in ['rustc_hashes','indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-def-id/indexmap-alias/replay'],env,root)
 qs=requests();data=''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode();raw=run([native],env,root,data);actual=run([subject],env,root,data);a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('alias record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'alias mismatch case{n}')
 fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_indexmap_alias.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'generate.py',here.parent/'indexmap_alias_oracle.rs',*sorted((here/'subject').glob('*.go')),*sorted((here/'replay').glob('*.go')),root/'port/rust/compiler/rustc_span/src/def_id_indexmap.go',root/'port/rust/compiler/rustc_span/src/def_id_indexmap_alias_oracle_test.go',*sorted((root/'port/support/rustindexmap').glob('*.go'))]
 report={'status':'partial-genuine-StableCrateIdMap-binding','source_file_complete':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'cases':len(qs),'operations':sum(len(q['ops'])for q in qs),'request_sha256':sha(data),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Actual fixed alias with BuildHasherDefault<Unhasher>, concrete live hasher state, supported ordered core/Entry/range/drain/truncate operations, default hash_one, capacities, Clone/CloneFrom/equality/Debug. Remaining IndexMap and Unord layers still pending.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('alias evidence differs')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/rust/compiler/rustc_span/src','-run','^TestDefIdOriginalIndexMapAlias$','-count=1'],env,root);print(f'StableCrateIdMap genuine partial alias: {len(qs)} native cases /{report["operations"]} operations passed')
if __name__=='__main__':main()
