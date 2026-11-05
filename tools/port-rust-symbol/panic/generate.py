#!/usr/bin/env python3
"""Actual whole-crate Symbol panic/owned-context observations, four profiles."""
import argparse,concurrent.futures,gzip,hashlib,io,json,os,pathlib,resource,signal,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(HERE.parent));import native_profile
import symbol_reference
BASE=ROOT/'port/rust/compiler/rustc_span/src'

def sha(data):return hashlib.sha256(data).hexdigest()
def run(cmd,env,timeout=180,**kw):
 p=subprocess.run([str(x)for x in cmd],cwd=ROOT,env=env,capture_output=True,timeout=timeout,**kw)
 if p.returncode:raise RuntimeError('command failed: '+str(cmd)+'\n'+p.stderr.decode(errors='replace')[-4000:])
 return p.stdout

def requests():
 cases=[]
 def add(op,values=[0],fails=[0],auxes=[0]):
  for value in values:
   for fail in fails:
    for aux in auxes:
     for mode in range(4):cases.append('\t'.join(map(str,[op,mode,value,fail,aux])))
 add('new-symbol',[0,0xffffff00,0xffffff01,0xffffffff]);add('new-byte',[0,0xffffff00,0xffffff01,0xffffffff])
 add('get-symbol',[0,2420,2421,0xffffff00]);add('get-byte',[0,2420,2421,0xffffff00])
 add('clamp-symbol');add('clamp-byte');add('prefill',[0,1]);add('lifetime',[0,1],auxes=[0,1,2]);add('debug-get',[0,2421],auxes=[0,1])
 add('substring',[0,(1<<64)-2,(1<<64)-1]);add('no-session');add('ident',fails=[0,1]);add('format-error');add('context',fails=range(4))
 return cases

def observe(binary,request,env):
 child=env.copy();child['FERRUGO_SYMBOL_PANIC_TRACE']='1'
 p=subprocess.run([str(binary)],input=(request+'\n').encode(),env=child,cwd=ROOT,capture_output=True,timeout=10)
 if len(p.stdout)>1<<20 or len(p.stderr)>1<<20:raise RuntimeError('native output exceeds1MiB')
 lines=p.stdout.decode().splitlines()
 if p.returncode in[-signal.SIGABRT,134]:return {'abort':True,'trace':lines}
 if p.returncode:raise RuntimeError('unexpected native exit '+str(p.returncode)+' '+p.stderr.decode(errors='replace')[-1000:])
 fields=lines[-1].split('\t')
 if len(fields)!=2:raise RuntimeError('native schema differs')
 trace=bytes.fromhex(fields[1]).decode().split('|')if fields[1]else[]
 if trace!=lines[:-1]:raise RuntimeError('native streamed trace differs')
 return {'abort':False,'result':bytes.fromhex(fields[0]).decode(),'trace':trace}

def semantic(info,req):
 payload=info['payload'];mode=int(req.split('\t')[1])
 if payload=='assertion failed: value <= 0xFFFF_FF00':return 'rustc_span.symbol.index_from_u32'
 if payload=='assertion failed: min <= max':return 'rustc_span.symbol.clamp'
 if payload.startswith('index out of bounds:'):return 'rustc_span.symbol.interner_index'
 if payload.startswith('duplicate symbols in the rustc symbol list'):return 'rustc_span.symbol.prefill_duplicate'
 if payload=='only lifetime idents should be passed with RawLifetime mode':return 'rustc_span.symbol.raw_lifetime_expect'
 if payload=='attempt to add with overflow':return 'rustc_span.edit_distance.limit_add'
 if payload=='cannot access a scoped thread local variable without calling `set` first':return 'rustc_span.symbol.session_unset'
 if payload=='a Display implementation returned an error unexpectedly: Error':return 'rustc_span.symbol.to_string_error'
 if payload in['called `Ident::new` with empty symbol','ExplicitBug']:return 'symbol.client.ident'
 if payload=='outer':return 'symbol.client.outer3'if mode==3 else'symbol.client.outer12'
 if payload=='context-drop':return 'symbol.client.context_drop'
 if payload=='field-drop':return 'symbol.client.field_drop'
 if payload=='panic in a destructor during cleanup':return 'symbol.runtime.cleanup'
 raise RuntimeError('unclassified source hook payload '+payload)

def hook_info(event):
 p=event.split(':')
 return {'kind':p[1],'payload':bytes.fromhex(p[2]).decode(),'can_unwind':p[3]=='true','force_no_backtrace':p[4]=='true','file':bytes.fromhex(p[5]).decode(),'line':int(p[6]),'column':int(p[7])}

def load_profiles(configs):
 # Each requested profile is prepared even on a cold cache. --prepared never
 # changes this boundary, and cached executable/linkage commands are ignored.
 return {name:symbol_reference.load(debug,checked)for name,debug,checked in configs}

def output_paths(value):
 if value is None:return HERE/'manifest.json',BASE/'testdata/symbol_panic.jsonl.gz',None
 if not str(value):raise ValueError('Symbol producer output directory is explicitly empty')
 directory=pathlib.Path(value);directory=directory if directory.is_absolute()else ROOT/directory
 if directory.is_symlink()or directory.exists()and not directory.is_dir():raise ValueError('invalid Symbol producer output directory')
 paths=[directory/name for name in ['manifest.json','symbol_panic.jsonl.gz','symbol_panic.jsonl']]
 for path in paths:
  if path.is_symlink()or path.exists()and not path.is_file():raise ValueError('invalid Symbol producer output file')
 return tuple(paths)

def preserve_historical_producer(raw,fixture,explicit_output):
 # Default generation may refresh real source bindings, but must never replace
 # historical observations merely because a new host spells its cache path
 # differently. A distinct native producer belongs in an explicit directory.
 if explicit_output:return
 if fixture.is_symlink()or not fixture.is_file()or fixture.stat().st_size>8<<20:
  raise RuntimeError('historical Symbol producer is missing or invalid; use --output-dir for new evidence')
 try:
  with gzip.GzipFile(fileobj=io.BytesIO(fixture.read_bytes()))as stream:previous=stream.read((16<<20)+1)
 except(OSError,EOFError)as error:
  raise RuntimeError('historical Symbol producer cannot be verified; use --output-dir for new evidence')from error
 if len(previous)>16<<20 or previous!=raw:
  raise RuntimeError('native Symbol producer differs from historical raw observations; use --output-dir to preserve verbatim new evidence')

def profile(config,selected,cases,work):
 name,debug,checked=config;p,env,_=selected;rustc=p['rustc']
 if p['debug_assertions']!=debug or p['overflow_checks']!=checked or p['cfg_release']!='1.101.0':raise RuntimeError('wrong whole-crate build profile')
 provenance=json.loads(pathlib.Path(p['provenance_path']).read_text())
 binary=work/('oracle-'+name)
 cmd=[rustc,'tools/port-rust-symbol/panic/oracle.rs',*p['link_arguments'],'-C','opt-level=0','-C','debug-assertions='+str(debug).lower(),'-C','overflow-checks='+str(checked).lower(),'-o',str(binary)]
 run(cmd,env);rows=[];sites={}
 for req in cases:
  out=observe(binary,req,env)
  for event in out['trace']:
   if event.startswith('hook:'):
    info=hook_info(event);site=semantic(info,req)
    # Runtime-varying payload values share one exact source location/kind.
    meta={k:v for k,v in info.items()if k not in['payload','kind']}
    if site in sites:
     previous={k:v for k,v in sites[site].items()if k!='kinds'}
     if previous!=meta:raise RuntimeError('source site metadata varies: '+site)
     sites[site]['kinds']=sorted(set(sites[site]['kinds']+[info['kind']]))
    else:sites[site]={**meta,'kinds':[info['kind']]}
  rows.append({'request':req,'want':out})
 no_sync=env.copy();no_sync['FERRUGO_SYMBOL_NO_SYNC']='1'
 no_sync_rows=[{'request':'reenter-index-hook\t0\t2421\t0\t0','want':observe(binary,'reenter-index-hook\t0\t2421\t0\t0',no_sync)}]
 if not no_sync_rows[0]['want']['abort']:raise RuntimeError('real no-sync interner hook reentry did not abort')
 for event in no_sync_rows[0]['want']['trace']:
  if event.startswith('hook:'):
   info=hook_info(event);site=semantic(info,no_sync_rows[0]['request']);meta={k:v for k,v in info.items()if k not in['payload','kind']}
   if site not in sites or {k:v for k,v in sites[site].items()if k!='kinds'}!=meta or info['kind']not in sites[site]['kinds']:raise RuntimeError('no-sync source hook metadata differs')
 print(f'Symbol panic {name}: {len(rows)} observations,{sum(r["want"]["abort"]for r in rows)} aborts',flush=True)
 return rows,{'name':name,'debug_assertions':debug,'overflow_checks':checked,'opt_level':0,'cfg_release':'1.101.0','source_provenance_sha256':sha(pathlib.Path(p['provenance_path']).read_bytes()),'source_graph_sha256':provenance['source_graph_sha256'],'selected_lock_sha256':provenance['selected_lock_sha256'],'original_span_tests':p['upstream_tests_passed'],'locations':sites,'records':len(rows),'aborts':sum(r['want']['abort']for r in rows),'client_source_argument':'tools/port-rust-symbol/panic/oracle.rs','dynamic_thread_safe':True},no_sync_rows

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--prepared',action='store_true');p.add_argument('--offline',action='store_true');p.add_argument('--jobs',type=int,default=2);p.add_argument('--output-dir',help='write an environment-specific producer manifest, gzip and raw JSONL here');args=p.parse_args();resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 if args.jobs not in range(1,5):raise RuntimeError('jobs must be1..4')
 manifest,fixture,raw_artifact=output_paths(args.output_dir)
 work=ROOT/'.toolchain/oracles/symbol-panic';work.mkdir(parents=True,exist_ok=True)
 configs=[(f'd{int(d)}o{int(o)}',d,o)for d in[False,True]for o in[False,True]]
 selected=load_profiles(configs);compiler=selected['d1o1'][2]
 cases=requests()
 with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs)as pool:results=list(pool.map(lambda c:profile(c,selected[c[0]],cases,work),configs))
 grouped={}
 for rows,meta,no_sync_rows in results:
  for row in rows:
   key=json.dumps(row,separators=(',',':'))
   if key not in grouped:grouped[key]={**row,'profiles':[]}
   grouped[key]['profiles'].append(meta['name'])
  for row in no_sync_rows:
   key=json.dumps(row,separators=(',',':'))
   if key not in grouped:grouped[key]={**row,'profiles':[]}
   grouped[key]['profiles'].append(meta['name']+'-no-sync')
 profiles=[]
 for rows,meta,no_sync_rows in results:
  profiles.append(meta)
  profiles.append({**meta,'name':meta['name']+'-no-sync','records':1,'aborts':1,'dynamic_thread_safe':False})
 raw=b''.join((json.dumps(r,separators=(',',':'))+'\n').encode()for r in grouped.values())
 if len(raw)>16<<20:raise RuntimeError('fixture raw bound exceeded')
 preserve_historical_producer(raw,fixture,args.output_dir is not None)
 compressed=symbol_reference.compressed_fixture(raw,fixture)
 files=[HERE/'oracle.rs',pathlib.Path(__file__),HERE.parent/'native_profile.py',BASE/'symbol_panic.go',BASE/'symbol_panic_contract_test.go',BASE/'symbol_panic_evidence_test.go',HERE.parent/'symbol_reference.py',HERE.parent/'test_reference.py',HERE/'test_portability.py',HERE/'VALIDATION.md',HERE.parent/'PORT.md',BASE/'symbol.go',BASE/'symbol_ident.go',BASE/'lib_bug_runtime.go',BASE/'lib_bug_runtime_cells.go',BASE/'symbol_context_ord.go',BASE/'symbol_format.go',BASE/'edit_distance.go',ROOT/'port/support/rustvec/ownership.go',ROOT/'port/support/rustfmt/panic.go']
 source={path:sha(native_profile.source_bytes(path))for path in ['compiler/rustc_span/src/symbol.rs','compiler/rustc_span/src/edit_distance.rs','compiler/rustc_index_macros/src/newtype.rs']}
 metadata={'schema':1,'rust_revision':native_profile.REVISION,**compiler,'scope':'Actual unchanged complete rustc_span Symbol and edit-distance public operations, their generated SymbolIndex, ordinary source panic metadata/processing/cleanup and owned ContextOrd field glue; generic ToString error is a separately identified actual std client. No inferred filenames or default locations, no new whole dependency credit.','profiles':profiles,'native_observations':sum(len(rows)+len(extra)for rows,_,extra in results),'retained_rows':len(grouped),'raw_bytes':len(raw),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'source_sha256':source,'reference_builder_sha256':sha((ROOT/'tools/port-rust-span-reference/prepare.py').read_bytes()),'registry_dependencies':json.loads(pathlib.Path(selected['d1o1'][0]['provenance_path']).read_text())['registry_dependencies'],'reference_inputs':symbol_reference.inputs(),'producer_profile':{'environment_specific':True,'filenames':'verbatim native producer; Cargo cache paths are not portable across builds','raw_artifact':'symbol_panic.jsonl'if raw_artifact is not None else None},'bindings':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in files},'bounds':{'native_case_seconds':10,'stdout_bytes':1<<20,'stderr_bytes':1<<20,'raw_fixture_bytes':16<<20,'core_dumps':False},'abort_comparison':'SIGABRT/exit134 class, all component event/hook traces; native runtime abort diagnostic/backtrace text is outside this scoped comparison.'}
 data=(json.dumps(metadata,indent=2)+'\n').encode()
 if args.check:
  if not fixture.is_file()or not manifest.is_file()or fixture.read_bytes()!=compressed or manifest.read_bytes()!=data or raw_artifact is not None and(not raw_artifact.is_file()or raw_artifact.read_bytes()!=raw):raise RuntimeError('Symbol panic producer proof/bindings differ; native filenames are environment-specific')
 else:
  fixture.parent.mkdir(parents=True,exist_ok=True);manifest.parent.mkdir(parents=True,exist_ok=True)
  fixture.write_bytes(compressed);manifest.write_bytes(data)
  if raw_artifact is not None:raw_artifact.write_bytes(raw)
 print(f'Symbol panic: {metadata["native_observations"]} observations,{len(grouped)} retained rows,all4 whole-crate profiles')
if __name__=='__main__':
 try:main()
 except Exception as e:print(e,file=sys.stderr);sys.exit(1)
