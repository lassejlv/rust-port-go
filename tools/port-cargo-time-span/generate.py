#!/usr/bin/env python3
"""Full unchanged Cargo time_span.rs, original tests and exact-anyhow producer."""
import argparse,gzip,hashlib,json,os,pathlib,random,resource,signal,subprocess,tempfile
import prepare
ROOT=prepare.ROOT;HERE=prepare.HERE;LIMIT=32*1024*1024

def requests():
 units=['second','seconds','minute','minutes','hour','hours','day','days','week','weeks','month','months']
 words=['','0','1','second','+2 seconds','-1 days','1.5 days','1 dayz','always','never','1 day ',' 1 day','1  second','0seconds','0001 second','0 months','18446744073709551615 seconds','18446744073709551616 seconds','9'*1024,'0'*1024+'1 seconds','1\tsecond','1\nsecond','1\u00a0second','١second','１second','\0','é','Σ','\u2028','\u200b','\U0001cfaf','\\\"\t\r\n\0']
 for unit in units:
  factor=1 if unit.startswith('second')else 60 if unit.startswith('minute')else 3600 if unit.startswith('hour')else 86400 if unit.startswith('day')else 604800 if unit.startswith('week')else 2629746
  for value in [0,1,2,23,(1<<64)//factor-1,((1<<64)-1)//factor,((1<<64)-1)//factor+1,(1<<64)-1,1<<64]:
   for gap in ['',' ','  ','\t']:
    words.append(str(value)+gap+unit)
 rng=random.Random(0x24de4721)
 alphabet=['a','b','0','9',' ','_','\0','\n','\t','é','Σ','\u200d','\u0301','\U0001cfaf','🦀']
 for _ in range(1200):words.append(''.join(rng.choice(alphabet)for _ in range(rng.randrange(32))))
 words=list(dict.fromkeys(words));rows=[]
 for word in words:
  for op in ['M','P']:rows.append({'op':op,'span':word})
 bad=['','abc','\0','a\"\\\n\t','é\u200d\U0001cfaf']
 for span in bad:
  for pattern in range(12):
   for fail in [0,1,2,3,4,5,8,24]:
    for chars in [False,True]:rows.append({'op':'F','span':span,'pattern':pattern,'fail':fail,'chars':chars})
 return rows

def wire(q):
 fields=[q['op'],q['span'].encode().hex()]
 if q['op']=='F':fields.extend([str(q['pattern']),str(q['fail']),str(q['chars']).lower()])
 return '\t'.join(fields)+'\n'
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def output_paths(value):
 if value is None:return(HERE/'manifest.json',ROOT/'port/cargo/src/util/testdata/time-span.jsonl.gz',None)
 if not value:raise RuntimeError('output directory explicitly empty')
 directory=pathlib.Path(value).absolute()
 if directory.is_symlink()or directory.exists()and not directory.is_dir():raise RuntimeError('invalid producer directory')
 paths=(directory/'manifest.json',directory/'time-span.jsonl.gz',directory/'time-span.jsonl')
 if any(p.resolve()==h.resolve()for p in paths for h in output_paths(None)[:2]):raise RuntimeError('producer overlaps historical evidence')
 for p in paths:
  if p.is_symlink()or p.exists()and not p.is_file():raise RuntimeError('nonregular producer output')
 return paths

def bindings():
 paths=[HERE/name for name in ['Cargo.toml','Cargo.lock','oracle.rs','prepare.py','generate.py']]
 paths+=[ROOT/'port/cargo/src/util'/name for name in ['queue_duration.go','time_span.go','time_span_test.go','time_span_oracle_test.go']]
 paths+=sorted((ROOT/'port/support/rustanyhow').glob('*.go'))
 paths+=sorted((ROOT/'port/support/rustfmt').glob('*.go'))
 paths+=sorted((ROOT/'port/support/ruststr').glob('*.go'))
 paths+=[HERE/'sdk199.json']
 return {str(p.relative_to(ROOT)):prepare.sha(p.read_bytes())for p in paths}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--fetch',action='store_true');p.add_argument('--check',action='store_true');p.add_argument('--output-dir');a=p.parse_args();manifest,destination,rawpath=output_paths(a.output_dir)
 profile,base,cargo=prepare.prepare(a.fetch);rows=[];request_rows=requests();raw_requests=''.join(map(wire,request_rows)).encode();profiles=[]
 for debug in [False,True]:
  for overflow in [False,True]:
   key=f'd{int(debug)}o{int(overflow)}';target=prepare.CACHE/('target-'+key);env=dict(base,CARGO_HOME=str(ROOT/'.toolchain/port-cargo-home'),CARGO_TARGET_DIR=str(target),RUSTC_BOOTSTRAP='1',RUSTFLAGS='--remap-path-prefix='+str(prepare.CACHE/'original')+'=cargo-original')
   for kind in ['DEV','TEST']:
    env['CARGO_PROFILE_'+kind+'_DEBUG_ASSERTIONS']=str(debug).lower();env['CARGO_PROFILE_'+kind+'_OVERFLOW_CHECKS']=str(overflow).lower()
   command=[cargo,'test','--offline','--locked','--manifest-path',str(HERE/'Cargo.toml'),'--bin','oracle'];subprocess.run(command,cwd=ROOT,env=env,check=True,timeout=120)
   subprocess.run([cargo,'build',*command[2:]],cwd=ROOT,env=env,check=True,timeout=120)
   binary=target/'debug/oracle';out=subprocess.run([str(binary)],input=raw_requests,cwd=ROOT,env=env,check=True,stdout=subprocess.PIPE,timeout=30).stdout
   lines=out.decode().splitlines()
   if len(lines)!=len(request_rows):raise RuntimeError('native parser/format record count differs')
   for request,line in zip(request_rows,lines):rows.append({'profile':key,'request':request,'want':line})
   # Captured stacks are REAL producer-specific input. Fresh producers retain
   # exact addresses/filenames and the formatter output from that SAME error;
   # Go uses an explicit record/capture operation, never a synthetic Rust stack.
   captured=[{'op':'F','span':'\0bad','pattern':pattern,'fail':fail,'chars':chars,'captured':True}for pattern in [0,4,5,10]for fail in [0,1,2,3,8]for chars in [False,True]]
   captured_env=dict(env,RUST_LIB_BACKTRACE='1')
   out=subprocess.run([str(binary)],input=''.join(map(wire,captured)).encode(),cwd=ROOT,env=captured_env,check=True,stdout=subprocess.PIPE,timeout=30).stdout.decode().splitlines()
   if len(out)!=len(captured)or any(line.split('\t')[-1]!='true'for line in out):raise RuntimeError('actual native backtrace was not captured')
   for request,line in zip(captured,out):rows.append({'profile':key,'request':request,'want':line})
   ambient_inputs=['18446744073709551615 months','1 month','not a duration']
   ambient_cases=[(mode,span,op)for mode in ['normal','unwinding','processing']for span in ambient_inputs for op in ['M','P']]
   ambient_cases += [(mode,'not a duration','R')for mode in ['normal','unwinding','processing']]
   for mode,span,op in ambient_cases:
    child=subprocess.run([str(binary),mode,op,span.encode().hex()],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=no_core)
    abort=mode=='processing'and (op=='R' or overflow and span==ambient_inputs[0])
    if child.returncode!=(-signal.SIGABRT if abort else 0):raise RuntimeError('unexpected native ambient status:'+str(child.returncode))
    events=[line[6:]for line in child.stdout.decode().splitlines()if line.startswith('EVENT:')]
    rows.append({'profile':key,'request':{'op':'A','inner':op,'mode':mode,'span':span},'aborted':abort,'events':events})
   profiles.append({'name':key,'debug_assertions':debug,'overflow_checks':overflow,'original_tests':2,'original_assertions':21})
 raw=b''.join((json.dumps(row,ensure_ascii=True,separators=(',',':'))+'\n').encode()for row in rows)
 if len(raw)>LIMIT:raise RuntimeError('native fixture exceeds bound')
 compressed=gzip.compress(raw,mtime=0)
 if destination.is_file():
  old=destination.read_bytes()
  try:
   with gzip.GzipFile(fileobj=__import__('io').BytesIO(old))as g:previous=g.read(LIMIT+1)
  except(OSError,EOFError):previous=None
  if previous==raw:compressed=old
  elif a.output_dir is None:raise RuntimeError('native captured-stack producer differs; use --output-dir')
 proof={'schema':1,'artifact_kind':'cargo-time-span-producer-v1','reference':profile,'profiles':profiles,'records':len(rows),'actual_aborts':sum(row.get('aborted',False)for row in rows),'source_locations':'Actual compiler remap of the copied source root to cargo-original; no observer normalization. Caller/tool/SDK locations otherwise verbatim.','request_sha256':prepare.sha(raw_requests),'raw_sha256':prepare.sha(raw),'fixture_sha256':prepare.sha(compressed),'files':bindings(),'captured_backtrace_contract':'Actual native record from the same returned error; explicit current-context capture operation in Go; no Go-created Rust stack, std backtrace file credit or cross-language allocator ABI.'}
 encoded=(json.dumps(proof,sort_keys=True,indent=2)+'\n').encode()
 if a.check:
  if manifest.read_bytes()!=encoded or destination.read_bytes()!=compressed or rawpath and rawpath.read_bytes()!=raw:raise RuntimeError('producer differs; captured records require fresh --output-dir')
 else:
  destination.parent.mkdir(parents=True,exist_ok=True);manifest.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
  if rawpath:rawpath.write_bytes(raw)
 print(len(rows),'actual original time-span records;',proof['actual_aborts'],'native aborts')
if __name__=='__main__':main()
