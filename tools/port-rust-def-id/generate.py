#!/usr/bin/env python3
"""Regenerate exact native def_id source observations and replay Go source bodies."""
import argparse,gzip,hashlib,json,os,pathlib,random,resource,shutil,signal,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def digest(b):return hashlib.sha256(b).hexdigest()
def run(args,env,cwd,input=None):
 p=subprocess.run([str(a)for a in args],cwd=cwd,env=env,input=input,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}: exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def executable(root,key,local,fallback):
 name=os.environ.get(key,str(root/local)if(root/local).is_file()else fallback)
 if not name:raise RuntimeError(key+' explicitly empty')
 if '/'in name and not pathlib.Path(name).is_absolute():name=str(root/name)
 p=shutil.which(name)
 if not p:raise RuntimeError(key+' unavailable:'+name)
 return os.path.abspath(p)
def basic(debug,overflow):
 rng=random.Random(0xdef1d2026);qs=[];maximum=0xffffff00
 for a in [0,1,2,65535,maximum]:
  for b in [0,1,2,65535,maximum]:qs.append({'op':'core','crate':a,'index':b,'first':rng.getrandbits(64),'second':rng.getrandbits(64)})
 for _ in range(256):qs.append({'op':'core','crate':rng.randrange(maximum+1),'index':rng.randrange(maximum+1),'first':rng.getrandbits(64),'second':rng.getrandbits(64)})
 edges=[0,1,2,maximum-1,maximum,maximum+1,(1<<32)-1,(1<<64)-1]
 amounts=[0,1,2,255,maximum,maximum+1,(1<<64)-2,(1<<64)-1]
 for kind in ['crate','index']:
  for a in edges:
   for b in edges:qs.append({'op':'step','kind':kind,'method':'between','a':a,'b':b})
   for n in amounts:
    for method in ['new','plus','increment','add','assign','forward_checked','backward_checked','forward_overflowing','backward_overflowing','forward','backward']:qs.append({'op':'step','kind':kind,'method':method,'a':a,'n':n})
 for q in qs:q.update(debug=debug,overflow=overflow)
 qs.extend(full_hashes(debug,overflow))
 return qs
def full_hashes(debug,overflow):
 qs=[]
 for kind in ['crate','index','id','local','mod','localmod','path','stable','controls']:
  for name in ['raw','typed','unhash']:
   for method in ['single','slice','empty']:
    for c,i,a,b in [(0,0,0,0),(1,2,10,26),(0xffffff00,0xffffff00,(1<<64)-1,1)]:
     for fail in ([-1,0,1]if name!='unhash'else[-1]):
      for seed in ([0,7]if name=='unhash'else[0]):
       for override in ([False,True]if name=='unhash'and kind=='path'else[False]):
        qs.append({'op':'full_hash','kind':kind,'name':name,'method':method,'crate':c,'index':i,'first':a,'second':b,'fail':fail,'a':seed,'override':override,'debug':debug,'overflow':overflow,'exe':bool(c&1)})
 return qs
def formats():
 qs=[]
 for kind in ['crate','crate_display','index','id','local','mod','localmod','path','stable','stable_hex','controls']:
  for a,b,c,d in [(0,0,0,0),(1,2,10,26),(0xffffff00,0xffffff00,(1<<64)-1,1)]:
   for mode in range(16):
    for chars in [False,True]:qs.append({'op':'format','kind':kind,'crate':a,'index':b,'first':c,'second':d,'mode':mode,'chars':chars,'fail':-1,'exe':bool(a&1),'debug':True,'overflow':True})
 return qs
def constructors():
 rng=random.Random(0x57ab1e);qs=[]
 for a in range(1,256):
  for b in range(1,256):qs.append({'op':'new_crate','name':'crate','exe':False,'metadata':[],'cfg':'cfg','version_hex':bytes([a,b]).hex()})
 strings=['','crate','a','ab','a\0b','🦀','é','界','metadata','lib','exe']
 for _ in range(2048):
  q={'op':'new_crate','name':rng.choice(strings),'exe':bool(rng.randrange(2)),'metadata':[rng.choice(strings)for _ in range(rng.randrange(9))],'cfg':rng.choice(strings)}
  if rng.randrange(2):q['version_hex']=bytes(rng.randrange(1,256)for _ in range(rng.randrange(16))).hex()
  qs.append(q)
 return qs
def codecs():
 qs=[]
 for kind in ['id','index','crate','local','mod','localmod','path','stable']:
  for c,i,a,b in [(0,0,0,0),(1,2,10,26),(0xffffff00,0xffffff00,(1<<64)-1,(1<<64)-2)]:
   for override in [False,True]:
    for fail in [-1,0,1]:qs.append({'op':'codec','kind':kind,'method':'encode','crate':c,'index':i,'first':a,'second':b,'override':override,'fail':fail})
   for fail in [-1,0,1]:
    for n in ([0,1,7,8,9,15,16,17,32]if kind in ['path','stable']else[0]):qs.append({'op':'codec','kind':kind,'method':'decode','crate':c,'index':i,'raw_hex':bytes(range(n)).hex(),'fail':fail})
 return qs
def context_cases():
 qs=[]
 for context in ['normal','inherited','processing']:
  for kind in ['raw','def','local','crate']:
   base={'kind':kind,'context':context}
   cases=[{}, {'get':True},{'default':True},{'drop':True},{'field_drop':True},{'drop':True,'field_drop':True},{'panic_call':0},{'panic_call':1}]
   if kind=='raw':cases +=[{'forget':True},{'panic_call':2},{'panic_call':0,'drop':True},{'panic_call':0,'field_drop':True}]
   else:cases +=[{'once':True},{'once':True,'drop':True},{'once':True,'field_drop':True},{'once':True,'panic_call':0},{'once':True,'panic_call':1},{'once':True,'panic_call':0,'drop':True}]
   qs.extend(dict(base,**q)for q in cases)
 return qs
def panic_cases():
 return [{'op':op,'debug':debug,'local':local,'context':context}for context in ['normal','inherited','processing']for op in ['expect','format']for local in [False,True]for debug in ['default','normal','error','partial','panic','swap']]
def collection_cases():
 qs=[]
 for local in [False,True]:
  for capacity in [0,7,20]:
   for constructor in ['capacity','iterator']:
    qs.append({'kind':'set','local':local,'capacity':capacity,'constructor':constructor})
    for action in ['clone','clone_from']:
     for failclone,faildrop in [(-1,-1),(10,-1),(42,-1),(1011,-1),(-1,100),(-1,101),(-1,102)]:qs.append({'kind':'map','local':local,'capacity':capacity,'constructor':constructor,'action':action,'clone':failclone,'drop':faildrop})
 return qs
def execute(binary,queries,env,root):
 raw=(''.join(json.dumps(q,separators=(',',':'))+'\n'for q in queries)).encode();out=run([binary],env,root,raw);rows=list(map(json.loads,out.splitlines()))
 if len(rows)!=len(queries):raise RuntimeError('native record count differs')
 return out,rows
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-def-id';env=os.environ.copy();rustc=executable(root,'FERRUGO_RUSTC',pathlib.Path('.toolchain/rust/bin/rustc'),'rustc');cargo=executable(root,'FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo');go=executable(root,'FERRUGO_GO',pathlib.Path('.toolchain/go/bin/go'),'go');env.update(FERRUGO_RUSTC=rustc,FERRUGO_CARGO=cargo,FERRUGO_GO=go,RUSTC_BOOTSTRAP='1');env.setdefault('GOCACHE','/tmp/ferrugo-go-cache')
 if run([rustc,'--version'],env,root).decode().strip()!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('native formatter reference requires pinned rustc1.99.1')
 reference=root/'.toolchain/oracles/def-id-reference';parts=[];profiles=[];record_count=0;format_count=0
 for debug,overflow in [(True,True),(True,False),(False,True),(False,False)]:
  print(f'original def_id profile debug={debug} overflow={overflow}',flush=True)
  run([sys.executable,here/'reference-build/prepare.py','--root',root,*(['--offline']if args.offline else[]),'--debug',str(debug).lower(),'--overflow',str(overflow).lower()],env,root)
  profile=json.loads((reference/'build-profile.json').read_text());profiles.append(profile);deps=pathlib.Path(profile['library']).parent/'deps';binary=reference/f'oracle-d{int(debug)}o{int(overflow)}'
  argv=[rustc,'--edition=2024','-Cdebug-assertions='+str(debug).lower(),'-Coverflow-checks='+str(overflow).lower(),str((here/'oracle.rs').relative_to(root)),*profile['link_arguments']]
  for name in ['serde_json','rustc_hashes','rustc_serialize']:
   found=list(deps.glob('lib'+name+'-*.rlib'))
   if len(found)!=1:raise RuntimeError('ambiguous native dependency:'+name)
   argv+=['--extern',name+'='+str(found[0])]
  run([*argv,'-o',binary],env,root)
  raw,rows=execute(binary,basic(debug,overflow),env,root);parts.append(raw);record_count+=len(rows)
  if debug and overflow:
   raw,newrows=execute(binary,constructors(),env,root);parts.append(raw);record_count+=len(newrows)
   raw,codecrows=execute(binary,codecs(),env,root);parts.append(raw);record_count+=len(codecrows)
   raw,rows=execute(binary,formats(),env,root);parts.append(raw);record_count+=len(rows);format_count+=len(rows);failing=[]
   for row in rows:
    if 'calls'not in row['result']:raise RuntimeError('format success record missing calls:'+str(row))
    for fail in range(len(row['result']['calls'])+1):failing.append(dict(row['request'],fail=fail))
   raw,failrows=execute(binary,failing,env,root);parts.append(raw);record_count+=len(failrows);format_count+=len(failrows)
 # Custom Fn/FnMut/FnOnce, consuming capture and full destructor observations.
 profile=profiles[-1];deps=pathlib.Path(profile['library']).parent/'deps';context_binary=reference/'context-oracle'
 argv=[rustc,'--edition=2024',str((here/'context_oracle.rs').relative_to(root)),*profile['link_arguments']]
 for name in ['serde_json','rustc_hashes','rustc_serialize']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native context dependency:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',context_binary],env,root)
 context_rows=[]
 for q in context_cases():
  outcome=subprocess.run([str(context_binary),'--live'],cwd=root,env=env,input=(json.dumps(q)+'\n').encode(),capture_output=True,timeout=10)
  events=[]
  for line in outcome.stdout.decode().splitlines():
   if line.startswith('EVENT\t'):events.append(line[len('EVENT\t'):])
  code=outcome.returncode
  if code not in [0,-signal.SIGABRT]:raise RuntimeError('unexpected native context status:'+str(outcome.returncode))
  code=134 if code else 0
  context_rows.append({'request':q,'code':code,'events':events})
 context_data=(''.join(json.dumps(q,separators=(',',':'))+'\n'for q in context_rows)).encode();context_fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_context.jsonl'
 if args.check:
  if context_fixture.read_bytes()!=context_data:raise RuntimeError('context native records differ')
 else:context_fixture.write_bytes(context_data)
 panic_binary=reference/'panic-oracle'
 argv=[rustc,'--edition=2024',str((here/'panic_oracle.rs').relative_to(root)),*profile['link_arguments']]
 found=list(deps.glob('libserde_json-*.rlib'))
 if len(found)!=1:raise RuntimeError('ambiguous panic native serde')
 run([*argv,'--extern','serde_json='+str(found[0]),'-o',panic_binary],env,root)
 panic_rows=[]
 for q in panic_cases():
  outcome=subprocess.run([str(panic_binary),'--live'],cwd=root,env=env,input=(json.dumps(q)+'\n').encode(),capture_output=True,timeout=10)
  events=[line[len('EVENT\t'):]for line in outcome.stdout.decode().splitlines()if line.startswith('EVENT\t')]
  if outcome.returncode not in [0,-signal.SIGABRT]:raise RuntimeError('unexpected panic native status:'+str(outcome.returncode))
  code=134 if outcome.returncode else 0
  panic_rows.append({'request':q,'code':code,'events':events})
 panic_data=(''.join(json.dumps(q,separators=(',',':'))+'\n'for q in panic_rows)).encode();panic_fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_panic.jsonl'
 if args.check:
  if panic_fixture.read_bytes()!=panic_data:raise RuntimeError('panic native records differ')
 else:panic_fixture.write_bytes(panic_data)
 collection_profile=profiles[0];collection_deps=pathlib.Path(collection_profile['library']).parent/'deps';collection_binary=reference/'collections-oracle'
 found=list(collection_deps.glob('libserde_json-*.rlib'))
 if len(found)!=1:raise RuntimeError('ambiguous collection native serde')
 run([rustc,'--edition=2024',str((here/'collections_oracle.rs').relative_to(root)),*collection_profile['link_arguments'],'--extern','serde_json='+str(found[0]),'-o',collection_binary],env,root)
 collection_data,collection_rows=execute(collection_binary,collection_cases(),env,root)
 collection_fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_collections.jsonl.gz';collection_compressed=None
 if collection_fixture.exists()and gzip.decompress(collection_fixture.read_bytes())==collection_data:collection_compressed=collection_fixture.read_bytes()
 if collection_compressed is None:collection_compressed=gzip.compress(collection_data,mtime=0,compresslevel=9)
 if args.check:
  if not collection_fixture.exists()or collection_fixture.read_bytes()!=collection_compressed:raise RuntimeError('collection native records differ')
 else:collection_fixture.write_bytes(collection_compressed)
 raw=b''.join(parts);fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_oracle.jsonl.gz';retained=None
 if fixture.exists():
  try:
   if gzip.decompress(fixture.read_bytes())==raw:retained=fixture.read_bytes()
  except(OSError,EOFError):pass
 if args.check and retained is None:raise RuntimeError('native def_id records differ')
 if retained is None:retained=gzip.compress(raw,mtime=0,compresslevel=9)
 if not args.check:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(retained)
 files=sorted((root/'port/rust/compiler/rustc_span/src').glob('def_id*.go'))
 test_env=dict(env,FERRUGO_DEF_ID_FIXTURE=str(fixture),FERRUGO_DEF_ID_CONTEXT_FIXTURE=str(context_fixture),FERRUGO_DEF_ID_PANIC_FIXTURE=str(panic_fixture))
 target='./port/rust/compiler/rustc_span/src'
 out=run([go,'test','-count=1',target,'-run','^TestDefId(Original(Oracle|OwnedContext|LazyPanic|Collections)|FnRuntimeContext|ComparatorRuntimeContext)$'],test_env,root);print(out.decode(),end='')
 metadata={'revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','status':'partial-pending-aliases-and-span-integration','native_profiles':[{k:v for k,v in p.items()if k not in ['library','link_arguments']}for p in profiles],'records':record_count,'format_writer_records':format_count,'full_hasher_records':sum(len(full_hashes(d,o))for d,o in [(True,True),(True,False),(False,True),(False,False)]),'context_cases':len(context_rows),'lazy_panic_cases':len(panic_rows),'lazy_panic_abort_cases':sum(row['code']==134 for row in panic_rows),'context_abort_cases':sum(row['code']==134 for row in context_rows),'raw_sha256':digest(raw),'fixture_sha256':digest(retained),'production_rust_fallback':False,'isolated_go':False,'scope':'Whole unchanged def_id source and genuine reference dependencies. Collection alias completion, final coherent integration and review remain separate pending obligations.','go_sources':[{'path':str(p.relative_to(root)),'sha256':digest(p.read_bytes())}for p in files],'helpers':[{'path':str(p.relative_to(root)),'sha256':digest(p.read_bytes())}for p in [here/'generate.py',here/'oracle.rs',here/'context_oracle.rs',here/'panic_oracle.rs',*sorted((here/'reference-build').glob('*'))]if p.is_file()]}
 metadata['collection_records']=len(collection_rows);metadata['collection_sha256']=digest(collection_data);metadata['collection_go_paths']='actual DefId/LocalDefId alias map/set/entry wrappers, non-Copy value traits, source Vec iterator constructors, Clone/whole-value CloneFrom, post-panic state and destruction';metadata['helpers'].append({'path':str((here/'collections_oracle.rs').relative_to(root)),'sha256':digest((here/'collections_oracle.rs').read_bytes())})
 encoded=(json.dumps(metadata,indent=2)+'\n').encode();manifest=here/'manifest.json'
 if args.check:
  if manifest.read_bytes()!=encoded:raise RuntimeError('def_id manifest differs')
 else:manifest.write_bytes(encoded)
 print(f'def_id: {record_count} original records; {format_count} fallible formatter records; four arithmetic profiles')
if __name__=='__main__':main()
