#!/usr/bin/env python3
"""Original range/drain mutation/cleanup/clone/consumption comparisons."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,signal,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout[-2000:].decode(errors="replace")}\n{p.stderr[-2000:].decode(errors="replace")}')
 return p.stdout
def requests():
 result=[]
 for n in [0,1,7,16,29]:
  bounds=list(dict.fromkeys([(0,n),(n//3,2*n//3),(n,n),(0,0)]))
  for capacity in sorted(set([n,4*n+16])):
   for mask in [0,7]:
    for start,end in bounds:
     for op in ['inspect','close','forget','get','get_mut','next','back','nth','nth_back','count','last','truncate']:
      for inherited in [False,True]:
       for nth in ([0,2,(1<<64)-1] if op in ['nth','nth_back'] else [end]):
        result.append(dict(n=n,capacity=capacity,mask=mask,start=[1,start],end=[2,end],again_start=[1,start],again_end=[2,end],front=int(n>1 and op in ['next','count']),back=int(n>1 and op in ['back','last']),op=op,nth=nth,fail='',inherited=inherited))
 for n in [0,1,7]:
  for start,end in [([0,0],[0,0]),([2,n],[0,0]),([1,n+1],[0,0]),([0,0],[1,n]),([0,0],[2,n+1]),([1,n],[2,0])]:
   for op in ['get','get_mut','close']:
    for inherited in [False,True]:
     result.append(dict(n=n,capacity=n,mask=0,start=start,end=end,again_start=[1,100],again_end=[2,200],front=0,back=0,op=op,nth=0,fail='',inherited=inherited))
 return result
def encode(qs):return ''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode()
def compare(native,subject,qs,env,root):
 raw=run([native],env,root,encode(qs));actual=run([subject],env,root,encode(qs));a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('range/drain record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'range/drain mismatch case{n}\n{json.dumps(expected)}\n{json.dumps(got)}')
 return raw,a
def trace_events(data):
 events=[];infos={}
 for line in data.decode().splitlines():
  if line.startswith('E:'):events.append(json.loads(line[2:]))
  elif line.startswith('H:'):
   info=json.loads(line[2:]);events.append(info);infos[info['Payload']]=info
  elif line.startswith('{'):pass
  else:raise RuntimeError('unexpected trace stdout:'+line)
 return events,infos
def guard_cases(native,subject,rows,env,root):
 requests=[]
 for row in rows:
  q=row['request'];r=row['result']
  if q['op']=='close' and r['panic']is not None:
   requests.extend([{**q,'trace':True},{**q,'trace':True,'fail':'rdrop:true'}])
  if q['n']==7 and q['capacity']==7 and q['mask']==0 and q['start']==[1,0] and q['end']==[2,7] and q['op']in['nth_back','count','last','close'] and q['nth']in[2,7]:
   for event in list(dict.fromkeys(r['events'])):
    if event.startswith(('kdrop:','vdrop:')):requests.append({**q,'trace':True,'fail':event})
 for row in rows:
  q=row['request']
  if q['n']==7 and q['capacity']==7 and q['mask']==0 and q['op']=='close' and not q['inherited']:
   requests.append({**q,'trace':True,'processing':True,'fail':'start:0:true'})
   if q['start']==[1,0] and q['end']==[2,7]:requests.extend([{**q,'trace':True,'processing':True,'fail':'kdrop:0:true'},{**q,'trace':True,'processing':True,'fail':''}])
 cases=[]
 for q in requests:
  native_result=subprocess.run([native],cwd=root,env=env,input=encode([q]),capture_output=True,timeout=10)
  if native_result.returncode not in[0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native guard status {native_result.returncode}: {native_result.stderr[-2000:]}')
  events,infos=trace_events(native_result.stdout);q={**q,'Infos':infos}
  actual=subprocess.run([subject],cwd=root,env=env,input=encode([q]),capture_output=True,timeout=10)
  want_exit=134 if native_result.returncode else 0;got,_=trace_events(actual.stdout)
  if actual.returncode!=want_exit or got!=events:raise RuntimeError(f'guard mismatch {q}\nexit{actual.returncode}/{want_exit}\ngot{got}\nwant{events}\nstderr{actual.stderr[-1000:]}')
  cases.append(dict(Request=q,Exit=want_exit,NativeExit=native_result.returncode,Events=events))
 return cases
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('indexmap_range_drain',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'indexmap-range-drain';subject=cache/'indexmap-range-drain-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here/'drain_oracle.rs').relative_to(root)),'-L','dependency='+str(deps)]
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/drain-replay'],env,root)
 qs=requests();raw,rows=compare(native,subject,qs,env,root);failures=[{**row['request'],'fail':event}for row in rows if row['request']['capacity']==row['request']['n'] and row['request']['mask']==0 and row['request']['n']in[0,7,16] for event in dict.fromkeys(row['result']['events']) if not(row['result']['panic']is not None and event.startswith('rdrop:'))];failure_raw,observed=compare(native,subject,failures,env,root);raw+=failure_raw
 guards=guard_cases(native,subject,rows,env,root);guard_data=(json.dumps(guards,indent=2)+'\n').encode();guard_fixture=root/'port/support/rustindexmap/testdata/drain_guards.json'
 fixture=root/'port/support/rustindexmap/testdata/drain.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'drain_generate.py',here/'drain_oracle.rs',*sorted((here/'subject').glob('*.go')),*sorted((here/'drain-replay').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go')),root/'port/support/rustvec/replace.go',root/'port/support/rusthashmap/raw_iter_bucket.go',root/'port/support/rusthashmap/raw_iter_bucket_test.go',root/'port/support/rusthashmap/raw_unique.go']
 report={'status':'partial-supporting-IndexMap-range-drain','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'baseline_cases':len(qs),'one_shot_failures':len(failures),'guard_cases':len(guards),'actual_native_sigabrt':sum(row['Exit']!=0 for row in guards),'guard_fixture_sha256':sha(guard_data),'caught_panics':sum(row['result']['panic']is not None for row in observed),'request_sha256':sha(encode(qs+failures)),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Original RangeBounds callback and complete consumed state Drop timing; repeated bounds in reversed diagnostics; all three erase_indices strategies and colliding hashes; Drain front/back/nth/count/last/forget, prepared-result leaks, remaining cursors, truncate, map queries after borrow ends, complete nonCopy K/V Drop failure cleanup, forgotten-drain query failures, actual tracked panic metadata, normal/inherited contexts and immediate processing-panic aborts. Partial supporting API, not whole indexmap or std Vec.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'drain_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not guard_fixture.exists()or guard_fixture.read_bytes()!=guard_data or not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('range/drain evidence differs')
 else:fixture.write_bytes(compressed);guard_fixture.write_bytes(guard_data);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','-run','^TestOriginalIndexMap(RangesAndDrain|DrainGuards)$','-count=1'],env,root);print(f'IndexMap range/drain: {len(qs)} original baseline +{len(failures)} failure cases and {len(guards)} guard cases passed; no whole dependency credit')
if __name__=='__main__':main()
