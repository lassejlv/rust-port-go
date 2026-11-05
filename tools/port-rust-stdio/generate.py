#!/usr/bin/env python3
"""Observe both original SDK stdio runtimes, retaining exact safe operations."""
import argparse,gzip,hashlib,io,json,os,re,resource,shlex,signal,subprocess,tempfile
from pathlib import Path
import prepare
ROOT=prepare.ROOT;HERE=prepare.HERE
DEST=ROOT/'port/support/ruststdio/testdata/cases.jsonl.gz'
LIMIT=16*1024*1024
def requests():
 modes=['plain','nested','preheld','preheld_nested','capture','capture_fatal','capture_nested','capture_preheld_nested','capture_fmt_error','capture_panic','capture_replace','capture_replace_panic','capture_poison_reuse','fmt_error','panic']
 result=[{'sdk':sdk,'mode':mode,'ambient':ambient,'script':''}for sdk in prepare.PROFILE for ambient in ['normal','unwinding','processing']for mode in modes]
 for sdk in prepare.PROFILE:
  for ambient in ['normal','unwinding','processing']:
   for script in ['e5','e9','e4,ok','ok,e4,ok','ok,n2,ok','ok,z','ok,ok,e5','e9,e9,e9,e9,e9','ok,ok,ok,e5']:
    result.append({'sdk':sdk,'mode':'release'if script=='e5'else'plain','ambient':ambient,'script':script})
   result.append({'sdk':sdk,'mode':'ignore_error','ambient':ambient,'script':'ok,ok,ok,e5'})
  for mode in ['concurrent','capture_concurrent']:result.append({'sdk':sdk,'mode':mode,'ambient':'normal','script':''})
  for capacity in [0,1,2,7,8,9,16,32,64,76,118,256]:result.append({'sdk':sdk,'mode':'capture','ambient':'normal','script':'','capacity':capacity})
 return result
def compressed(raw):
 if len(raw)>LIMIT:raise RuntimeError('stdio corpus exceeds bound')
 if DEST.is_symlink():raise RuntimeError('stdio fixture is a symlink')
 if DEST.exists()and DEST.stat().st_size<=LIMIT:
  previous=DEST.read_bytes()
  try:
   with gzip.GzipFile(fileobj=io.BytesIO(previous))as stream:old=stream.read(LIMIT+1)
   if old==raw:return previous
  except(OSError,EOFError):pass
 return gzip.compress(raw,mtime=0)
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--check',action='store_true');p.add_argument('--offline',action='store_true');p.add_argument('--cache',type=Path);p.add_argument('--rustc-199');p.add_argument('--rustc-1101');p.add_argument('--cc');args=p.parse_args()
 if not __import__('sys').platform.startswith('linux'):raise RuntimeError('native stderr syscall observer is explicitly Linux-only')
 sources=prepare.verified(args.cache,args.offline)
 cc=args.cc if args.cc is not None else os.environ.get('CC','cc')
 if not cc:raise RuntimeError('selected CC is explicitly empty')
 records=[];versions={};locations={};aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-stdio-')as tmp:
  shared=Path(tmp)/'observer.so';subprocess.run([*shlex.split(cc),'-std=c11','-shared','-fPIC',str(HERE/'write_observer.c'),'-o',str(shared)],cwd=ROOT,check=True,timeout=30)
  binaries={}
  for sdk in prepare.PROFILE:
   compiler,versions[sdk]=prepare.selected(sdk,getattr(args,'rustc_'+sdk.replace('.','')))
   binary=Path(tmp)/('native-'+sdk);subprocess.run([compiler,'--edition=2024',str(HERE.relative_to(ROOT)/'oracle.rs'),'-o',str(binary)],cwd=ROOT,env=dict(os.environ,RUSTC_BOOTSTRAP='1'),check=True,timeout=60);binaries[sdk]=binary
  for request in requests():
   env=dict(os.environ,LD_PRELOAD=str(shared),FERRUGO_STDIO_SCRIPT=request['script'])
   if request['mode']not in ['concurrent','capture_concurrent']:env['FERRUGO_STDIO_TRACE']='1'
   else:env.pop('FERRUGO_STDIO_TRACE',None)
   process=subprocess.run([str(binaries[request['sdk']]),request['mode'],request['ambient'],str(request.get('capacity',0))],cwd=ROOT,env=env,capture_output=True,timeout=15,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
   if process.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native exit {request}:{process.returncode}:{process.stderr!r}')
   aborted=process.returncode==-signal.SIGABRT;aborts+=aborted
   output=process.stdout.decode()
   if aborted:
    # The observer also sees libstd's terminal panic diagnostic writes. Those
    # are not the source printing operation and Go's explicit abort does not
    # claim their platform-specific text. Keep every operation event before
    # the positively identified selected SDK diagnostic, without inventing a
    # post-abort state. The fixed driver statements never print this prefix.
    lines=output.splitlines();marker='write:'+b'panicked at '.hex()
    if marker in lines:
     cut=lines.index(marker)
     if not all(line.startswith('write:')for line in lines[cut:]):raise RuntimeError('unexpected events after abort diagnostic')
     output='\n'.join(lines[:cut])+'\n'
   if request['mode']in ['concurrent','capture_concurrent']:
    captured=request['mode']=='capture_concurrent'
    if captured:
     if process.stderr or not output.startswith('complete\ncapture:'):raise RuntimeError('original concurrent capture unexpectedly escaped')
     text=bytes.fromhex(output.removeprefix('complete\ncapture:').strip()).decode()
    else:text=process.stderr.decode()
    lines=text.splitlines()
    if len(lines)!=64 or not all(re.fullmatch(r'BEGIN([0-7]):\1{512}END\1',line)for line in lines):raise RuntimeError('original stderr statements interleaved')
    record={'request':request,'concurrency':{'records':64,'chunks_per_record':512,'whole':True}}
   else:
    record={'request':request,'abort':aborted,'stdout':output,'stderr':''if aborted else process.stderr.hex()}
    for line in output.splitlines():
     if line.startswith('hook:'):
      fields=line.split(':');kind,payload,file,row,col=fields[1:6];text=bytes.fromhex(payload).decode();site='outer'if text=='outer'else'body'if text=='body-panic'else'format'if text.startswith('a formatting trait')else'io'
      location={'File':bytes.fromhex(file).decode(),'Line':int(row),'Column':int(col)};profile=locations.setdefault(request['sdk'],{})
      if site in profile and profile[site]!=location:raise RuntimeError('native source site unexpectedly differs')
      profile[site]=location
   records.append(record)
 raw=b''.join((json.dumps(record,sort_keys=True,separators=(',',':'))+'\n').encode()for record in records);gz=compressed(raw)
 files=[*sorted((ROOT/'port/support/ruststdio').glob('*.go')),*sorted(HERE.glob('*.py')),HERE/'oracle.rs',HERE/'write_observer.c']
 manifest={'schema':1,'status':'scoped-used-stdio-contract-no-source-file-credit','sdk_versions':versions,'sdk_sources':sources,'records':len(records),'actual_native_SIGABRT':aborts,'locations':locations,'request_sha256':prepare.sha(json.dumps(requests(),sort_keys=True).encode()),'raw_sha256':prepare.sha(raw),'fixture_sha256':prepare.sha(gz),'inputs_sha256':{str(path.relative_to(ROOT)):prepare.sha(path.read_bytes())for path in files},'source_only_branch_evidence':{'reentrant_lock_count':{'source':'library/std/src/sync/reentrant_lock.rs','width':'u32','increment':'checked_add(1), assignment only on Some','overflow':'Option.expect, OwnedString lock count overflow in reentrant mutex','source_callsite':[291,45],'scope':'Exact source verified both archives; Go private-limit simulation only, no native 2^32 public-guard construction claimed.'}},'native_scope':'Actual selected SDK std::io _eprint, StderrLock and output capture; safe Rust public APIs and child-only libc observation/injection. Concurrency observes64 complete512-yield records, no scheduler-order commitment. Abort stderr and positively identified terminal panic diagnostic write observations are omitted; all preceding source-operation events retained, no postabort state.'}
 rendered=json.dumps(manifest,indent=2)+'\n';path=HERE/'manifest.json'
 if args.check:
  if not DEST.is_file()or DEST.read_bytes()!=gz:raise RuntimeError('stdio native fixture differs')
  if not path.is_file()or path.read_text()!=rendered:raise RuntimeError('stdio manifest differs')
 else:
  DEST.parent.mkdir(parents=True,exist_ok=True);DEST.write_bytes(gz);path.write_text(rendered)
 print(len(records),'native stdio cases;',aborts,'actual SIGABRT; SDK1.99+1.101')
if __name__=='__main__':main()
