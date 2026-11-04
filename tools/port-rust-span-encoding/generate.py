#!/usr/bin/env python3
"""Independent native span-encoding source oracle and retained evidence driver."""
import argparse, collections, gzip, hashlib, json, os, pathlib, resource, signal, subprocess, sys, tempfile
import prepare
HERE=prepare.HERE;ROOT=prepare.ROOT
PACKAGE='./port/rust/compiler/rustc_span/src'
FIXTURE=ROOT/'port/rust/compiler/rustc_span/src/span_testdata/cases.jsonl.gz'
MANIFEST=HERE/'manifest.json'
BUILDER=ROOT/'port/rust/compiler/rustc_span/src/span_requests_test.go'
TOOL_FILES=['prepare.py','generate.py','oracle.rs','runtime.rs','adapters.rs','Cargo.toml','Cargo.lock','upstream/rustc_index_macros/Cargo.toml','SCHEMA.md','test_integrity.py','PORT.md']
def encoded(value):return json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()+b'\n'
def reference_inputs(destination):
 result={}
 for path in destination.rglob('*'):
  if any(part.startswith('target-')for part in path.relative_to(destination).parts):continue
  if path.is_symlink():raise RuntimeError(f'reference source symlink:{path}')
  if not path.is_file()or not(path.suffix=='.rs'or path.name in ['Cargo.toml','Cargo.lock']):continue
  result[path.relative_to(destination).as_posix()]=prepare.sha(path.read_bytes())
 return dict(sorted(result.items()))
def fatal_result(result):
 completed=[];events=[];start=None
 for line in result.stderr.splitlines():
  if line.startswith(b'PROBE_START\t'):start=json.loads(line.split(b'\t',1)[1]);events=[]
  elif line.startswith(b'PROBE_STEP\t'):completed.append(json.loads(line.split(b'\t',1)[1]))
  elif line.startswith(b'PROBE_EVENT\t'):events.append(json.loads(line.split(b'\t',1)[1]))
 if start is None:raise RuntimeError('native abort lacked its actual pre-operation snapshot')
 if result.returncode not in (-signal.SIGABRT,134):raise RuntimeError(f'unexpected native termination:{result.returncode}\n{result.stderr.decode(errors="replace")}')
 return {'steps':completed,'abort':{'op':start['op'],'class':'abort','events':events,'last_observed':{'interner':start['interner'],'slots':start['slots']}}}
def observe(binary,requests,env):
 responses=[None]*len(requests)
 regular=[(i,r)for i,r in enumerate(requests)if not r.get('abort',False)]
 # Bound subprocess capture sizes; all expected semantic output is still from
 # actual Rust, without any Go execution or expected-value synthesis here.
 for offset in range(0,len(regular),512):
  batch=regular[offset:offset+512];input=b''.join(encoded(r)for _,r in batch)
  result=subprocess.run([str(binary)],input=input,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
  if result.returncode:raise RuntimeError(f'ordinary native batch failed:{result.returncode}\n{result.stderr.decode(errors="replace")}')
  output=result.stdout.splitlines()
  if len(output)!=len(batch):raise RuntimeError('native response count differs from request count')
  for(i,_),line in zip(batch,output):responses[i]=json.loads(line)
 for i,request in enumerate(requests):
  if not request.get('abort',False):continue
  result=subprocess.run([str(binary)],input=encoded(request),env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
  if result.returncode:responses[i]=fatal_result(result)
  else:responses[i]=json.loads(result.stdout)
 return responses

def build(destination,debug,overflow,env,cargo):
 profile=f'd{int(debug)}o{int(overflow)}';profile_env=env.copy()
 profile_env.update({'CARGO_TARGET_DIR':str(destination/('target-'+profile)),'CARGO_PROFILE_DEV_DEBUG_ASSERTIONS':str(debug).lower(),'CARGO_PROFILE_DEV_OVERFLOW_CHECKS':str(overflow).lower()})
 if overflow:profile_env['RUSTFLAGS']='--cfg probe_overflow_checks'
 else:profile_env.pop('RUSTFLAGS',None)
 output=prepare.execute([cargo,'build','--locked','--offline','--bin','oracle','--message-format=json'],profile_env,destination)
 binary=None
 for line in output.splitlines():
  record=json.loads(line)
  if record.get('reason')=='compiler-artifact'and record['target']['name']=='oracle'and record.get('executable'):
   if pathlib.Path(record['target']['src_path']).resolve()!=destination/'oracle.rs':raise RuntimeError('reference executable source path differs')
   binary=pathlib.Path(record['executable'])
 if binary is None:raise RuntimeError('Cargo did not report actual native oracle executable')
 return binary,{'name':profile,'debug_assertions':debug,'overflow_checks':overflow,'rustflags':['--cfg','probe_overflow_checks']if overflow else[],'rustc_bootstrap':True}

def main():
 parser=argparse.ArgumentParser();parser.add_argument('-check','--check',action='store_true');parser.add_argument('--destination',type=pathlib.Path,default=ROOT/'.toolchain/oracles/span-encoding-reference');args=parser.parse_args()
 resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 env=prepare.environment();destination=args.destination.resolve();cargo=prepare.selected('FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo');go=prepare.selected('FERRUGO_GO',pathlib.Path('.toolchain/go/bin/go'),'go')
 tool_hashes={p:prepare.sha((HERE/p).read_bytes())for p in TOOL_FILES};builder_hash=prepare.sha(BUILDER.read_bytes())
 compiler=prepare.execute([env['RUSTC'],'--version'],env).decode().strip()
 if compiler!=prepare.RUST:raise RuntimeError('native formatter reference compiler differs')
 if b'target_pointer_width="64"'not in prepare.execute([env['RUSTC'],'--print','cfg'],env).splitlines():raise RuntimeError('native reference requires a 64-bit host')
 provenance,original_lock=prepare.prepare(destination,env)
 compiled_inputs=reference_inputs(destination)
 lock=(destination/'Cargo.lock').read_bytes();prepare.verify_lock(lock,original_lock)
 metadata=json.loads(prepare.execute([cargo,'metadata','--locked','--offline','--format-version=1'],env,destination));prepare.verify_packages(metadata,destination);deps=prepare.verify_registry(metadata,lock,pathlib.Path(env['CARGO_HOME']))
 with tempfile.TemporaryDirectory(prefix='ferrugo-span-requests-')as temporary:
  request_path=pathlib.Path(temporary)/'requests.jsonl';dump_env=env.copy();dump_env['FERRUGO_SPAN_DUMP_REQUESTS']=str(request_path)
  prepare.execute([go,'test',PACKAGE,'-run','^TestDumpSpanRequests$','-count=1'],dump_env)
  request_bytes=request_path.read_bytes()
 requests=[json.loads(line)for line in request_bytes.splitlines()];responses=[None]*len(requests);profiles=[]
 for debug in [False,True]:
  for overflow in [False,True]:
   binary,profile=build(destination,debug,overflow,env,cargo)
   selected=[(i,r)for i,r in enumerate(requests)if(r['debug_assertions'],r['overflow_checks'])==(debug,overflow)]
   output=observe(binary,[r for _,r in selected],env)
   for(i,_),response in zip(selected,output):responses[i]=response
   profile['requests']=len(selected);profiles.append(profile)
   print(f'Original span {profile["name"]}: {len(selected)} requests.',flush=True)
 if any(r is None for r in responses):raise RuntimeError('a request was not run in an actual native profile')
 counts=collections.Counter();panics=collections.Counter();aborts=0
 for request,response in zip(requests,responses):
  for op in request['ops']:counts[op['op']]+=1
  for step in response['steps']:
   if step['panic']is not None:
    if step['panic'].startswith('unclassified:'):raise RuntimeError(f'Unclassified original panic for {request["id"]}: {step["panic"]}')
    panics[step['panic']]+=1
  if 'abort'in response:aborts+=1
 if deps!=prepare.verify_registry(metadata,lock,pathlib.Path(env['CARGO_HOME'])):raise RuntimeError('compiled registry dependencies changed during native execution')
 if compiled_inputs!=reference_inputs(destination):raise RuntimeError('reference source inputs changed during native execution')
 if builder_hash!=prepare.sha(BUILDER.read_bytes()):raise RuntimeError('Go request builder changed during native execution')
 if tool_hashes!={p:prepare.sha((HERE/p).read_bytes())for p in TOOL_FILES}:raise RuntimeError('native tool inputs changed during execution')
 raw=b''.join(encoded({'request':r,'response':s})for r,s in zip(requests,responses))
 response_bytes=b''.join(encoded(s)for s in responses)
 info={'format':1,'revision':prepare.PIN,'source_version':provenance['source_version'],'compiler':compiler,'pointer_bits':64,'scope':'Complete unchanged span_encoding.rs plus the actual used dependency algorithms and recorded exact foundational fragments. Surrounding span/session/source-map runtime remains partial; no surrounding-file source credit.','sources':provenance['sources'],'exact_fragments':provenance['exact_fragments'],'source_provenance_sha256':prepare.sha((destination/'source-provenance.json').read_bytes()),'compiled_reference_sha256':compiled_inputs,'registry_dependencies':deps,'tool_sha256':tool_hashes,'request_builder_sha256':builder_hash,'requests_sha256':prepare.sha(request_bytes),'responses_sha256':prepare.sha(response_bytes),'requests':len(requests),'steps':sum(len(r['steps'])for r in responses),'aborts':aborts,'operations':dict(sorted(counts.items())),'panics':dict(sorted(panics.items())),'profiles':profiles,'raw_sha256':prepare.sha(raw)}
 if args.check:
  compressed=FIXTURE.read_bytes()
  if gzip.decompress(compressed)!=raw:raise RuntimeError('native span observations changed')
  info['fixture_sha256']=prepare.sha(compressed)
  if json.loads(MANIFEST.read_text())!=info:raise RuntimeError('native span evidence manifest changed')
 else:
  if FIXTURE.exists()and gzip.decompress(FIXTURE.read_bytes())==raw:compressed=FIXTURE.read_bytes()
  else:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
  prepare.write(FIXTURE,compressed);info['fixture_sha256']=prepare.sha(compressed);prepare.write(MANIFEST,(json.dumps(info,indent=2)+'\n').encode())
 replay=prepare.execute([go,'test',PACKAGE,'-run','^TestOriginalSpanEncoding$','-count=1','-v'],env)
 if b'--- PASS: TestOriginalSpanEncoding'not in replay:raise RuntimeError('the actual Go span replayer did not run')
 print(f'Native span encoding: {len(requests)} requests/{info["steps"]} steps/{aborts} native aborts; {"checked"if args.check else"retained"}.')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
