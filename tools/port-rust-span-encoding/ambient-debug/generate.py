#!/usr/bin/env python3
"""Separate immediate-abort Debug proof; main span evidence stays frozen."""
import argparse,gzip,hashlib,json,pathlib,platform,resource,signal,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
sys.path.insert(0,str(HERE.parent))
import prepare
import generate as parent_driver
ROOT=prepare.ROOT
FIXTURE=ROOT/'port/rust/compiler/rustc_span/src/span_testdata/ambient-debug/cases.jsonl.gz'
INPUTS=[HERE/'oracle.rs',HERE/'generate.py',HERE/'PORT.md',HERE/'test_probe.py',HERE.parent/'prepare.py',HERE.parent/'generate.py',HERE.parent/'runtime.rs',HERE.parent/'adapters.rs',HERE.parent/'Cargo.toml',HERE.parent/'Cargo.lock',HERE.parent/'upstream/rustc_index_macros/Cargo.toml',HERE.parent/'manifest.json']
def cases():
 result=[]
 for debug in [False,True]:
  for overflow in [False,True]:
   for ambient in ['normal','unwinding','processing_panic']:
    for kind in ['span','ord','data']:
     shapes=[('inline',False),('parent',False),('needs_interner',False),('needs_interner',True)]if kind=='data'else[('inline',False),('parent',False),('bad_index',False),('bad_index',True),('overflow',False)]
     for value,session in shapes:
      for pretty in [False,True]:
       for chars in [False,True]:
        for fail in [-1,0]:result.append(dict(id=f'ambient-debug-{len(result):04d}',debug_assertions=debug,overflow_checks=overflow,ambient=ambient,kind=kind,value=value,session=session,pretty=pretty,chars=chars,fail=fail))
 return result

def main():
 parser=argparse.ArgumentParser();parser.add_argument('-check','--check',action='store_true');args=parser.parse_args()
 if platform.system()!='Linux':raise RuntimeError('SIGABRT lock audit requires the recorded Linux profile')
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));env=prepare.environment();cargo=prepare.selected('FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo');go=prepare.selected('FERRUGO_GO',pathlib.Path('.toolchain/go/bin/go'),'go')
 inputs={str(p.relative_to(ROOT)):prepare.sha(p.read_bytes())for p in INPUTS}
 if prepare.execute([env['RUSTC'],'--version'],env).decode().strip()!=prepare.RUST:raise RuntimeError('native compiler differs')
 cfg=prepare.execute([env['RUSTC'],'--print','cfg'],env).splitlines()
 if b'target_pointer_width="64"'not in cfg or b'target_os="linux"'not in cfg:raise RuntimeError('native signal audit requires the recorded 64-bit Linux target')
 destination=(ROOT/'.toolchain/oracles/span-ambient-debug-reference').resolve();provenance,original=prepare.prepare(destination,env);prepare.write(destination/'oracle.rs',(HERE/'oracle.rs').read_bytes());staged=parent_driver.reference_inputs(destination)
 lock=(destination/'Cargo.lock').read_bytes();prepare.verify_lock(lock,original);metadata=json.loads(prepare.execute([cargo,'metadata','--locked','--offline','--format-version=1'],env,destination));prepare.verify_packages(metadata,destination);deps=prepare.verify_registry(metadata,lock,pathlib.Path(env['CARGO_HOME']))
 requests=cases();observations=[];profiles=[]
 for debug in [False,True]:
  for overflow in [False,True]:
   binary,profile=parent_driver.build(destination,debug,overflow,env,cargo);profiles.append(profile)
   for request in requests:
    if(request['debug_assertions'],request['overflow_checks'])!=(debug,overflow):continue
    run=subprocess.run([str(binary)],input=parent_driver.encoded(request),env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=15)
    audit=None
    if run.returncode:
     if run.returncode!=-signal.SIGABRT:raise RuntimeError(f'Unexpected native failure:{run.returncode}\n{run.stderr.decode(errors="replace")}')
     events=[]
     for line in run.stderr.splitlines():
      if line.startswith(b'PROBE_EVENT\t'):events.append(json.loads(line.split(b'\t',1)[1]))
      elif line.startswith(b'PROBE_LOCK\t'):audit=line.split(b'\t',1)[1].decode()
     if audit not in ['held','free']:raise RuntimeError('native abort omitted actual lock observation')
     response={'abort':'abort','events':events}
    else:
     response=json.loads(run.stdout)
     if response['panic']and response['panic'].startswith('unclassified:'):raise RuntimeError(response['panic'])
    observations.append({'request':request,'response':response,'reference_abort_lock':audit})
   print(f'Native ambient Debug {profile["name"]} complete.',flush=True)
 observations.sort(key=lambda v:v['request']['id']);raw=b''.join(parent_driver.encoded(v)for v in observations)
 if staged!=parent_driver.reference_inputs(destination):raise RuntimeError('staged original inputs changed')
 if deps!=prepare.verify_registry(metadata,lock,pathlib.Path(env['CARGO_HOME'])):raise RuntimeError('registry sources changed')
 if inputs!={str(p.relative_to(ROOT)):prepare.sha(p.read_bytes())for p in INPUTS}:raise RuntimeError('native helper inputs changed during execution')
 info={'format':1,'revision':prepare.PIN,'compiler':prepare.RUST,'records':len(observations),'aborts':sum('abort'in v['response']for v in observations),'lock_held_at_abort':sum(v['reference_abort_lock']=='held'for v in observations),'source_provenance_sha256':prepare.sha((destination/'source-provenance.json').read_bytes()),'compiled_reference_sha256':staged,'registry_dependencies':deps,'tool_sha256':inputs,'profiles':profiles,'raw_sha256':prepare.sha(raw),'scope':'Separate unchanged-source primary Debug ambient/processing-panic and actual native lock-at-abort proof; no main span corpus changes.'}
 manifest=HERE/'manifest.json'
 if args.check:
  compressed=FIXTURE.read_bytes()
  if gzip.decompress(compressed)!=raw:raise RuntimeError('native ambient Debug observations changed')
  info['fixture_sha256']=prepare.sha(compressed)
  if json.loads(manifest.read_text())!=info:raise RuntimeError('native ambient Debug manifest changed')
 else:
  compressed=FIXTURE.read_bytes()if FIXTURE.exists()and gzip.decompress(FIXTURE.read_bytes())==raw else gzip.compress(raw,compresslevel=9,mtime=0)
  prepare.write(FIXTURE,compressed);info['fixture_sha256']=prepare.sha(compressed);prepare.write(manifest,(json.dumps(info,indent=2)+'\n').encode())
 replay=prepare.execute([go,'test','./port/rust/compiler/rustc_span/src','-run','^TestOriginalSpanAmbientDebug$','-count=1','-v'],env)
 if b'--- PASS: TestOriginalSpanAmbientDebug'not in replay:raise RuntimeError('ambient Go replayer did not run')
 print(f'Native ambient Debug: {info["records"]} cases/{info["aborts"]} actual aborts/{info["lock_held_at_abort"]} held-lock observations.')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
