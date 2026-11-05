#!/usr/bin/env python3
"""Retain original-crate owned Entry callback, cleanup and partial-state cases."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}: exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 ops=['clone','clone_from','close','or_insert','with','with_key','modify','insert_entry','occupied_insert','swap_remove','shift_remove','into_key','vacant_insert']
 qs=[dict(n=n,capacity=cap,key=key,mask=mask,op=op,fail='',inherited=ctx)for n in [0,1,3,7,14,28]for cap in [0,40]for key in [0,99]for mask in [0,7,(1<<64)-1]for op in ops for ctx in [False,True]]
 return qs+[{**q,'from_override':True}for q in qs if q['op']=='clone_from']
def encode(qs):return ''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode()
def compare(native,subject,qs,env,root):
 data=encode(qs);raw=run([native],env,root,data);actual=run([subject],env,root,data);a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('owned record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'owned case {n} mismatch\n{json.dumps(expected)}\n{json.dumps(got)}')
 return raw,a
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();env=os.environ.copy();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build'
 spec=importlib.util.spec_from_file_location('indexmap_owned_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1'
 run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'indexmap-owned-rust';subject=cache/'indexmap-owned-go'
 argv=[env['FERRUGO_RUSTC'],'--edition=2024',str(here.relative_to(root)/'owned_oracle.rs'),'-L','dependency='+str(deps)]
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/owned-replay'],env,root)
 qs=requests();raw,baseline=compare(native,subject,qs,env,root)
 failures=[{**row['request'],'fail':event}for row in baseline for event in dict.fromkeys(row['result']['events'])]
 failure_raw,observations=compare(native,subject,failures,env,root);raw+=failure_raw
 fixture=root/'port/support/rustindexmap/testdata/owned.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'owned_generate.py',here/'owned_oracle.rs',*sorted((here/'owned-replay').glob('*.go')),*sorted((here/'subject').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go')),root/'port/support/rusthashmap/hasher_borrow.go',root/'port/support/rusthashmap/raw_unique.go']
 report={'status':'partial-supporting-IndexMap-owned-Entry','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'baseline_cases':len(qs),'one_shot_failures':len(failures),'caught_panics':sum(r['result']['panic']is not None for r in observations),'request_sha256':sha(encode(qs+failures)),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Actual non-Copy keys/values, independent CloneFrom overrides and complete custom FnOnce state; borrowed hashing/equality callbacks, default BuildHasher::hash_one, Entry parameter cleanup, vacant/occupied transitions, key/value/Fn Drop, initialized return-place leaks, insertion before old-value destructor panic, whole Core/Map Clone/CloneFrom cleanup and map partial states under normal/inherited unwinding. Not a complete IndexMap API/dependency or allocator/runtime proof.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]}
 manifest=here/'owned_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('owned IndexMap evidence differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','-run','^TestOriginalIndexMapOwned$','-count=1'],env,root)
 print(f'IndexMap owned Entry: {len(qs)} original baseline +{len(failures)} one-shot failures passed; no whole dependency credit')
if __name__=='__main__':main()
