#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
HERE=Path(__file__).absolute().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/support/rusttracing/testdata/borrow_panic.tsv'
SITES={'set':'tracing_core::dispatcher::State::set_default.replace','close':'tracing_core::dispatcher::DefaultGuard::drop.replace','id':'tracing_core::span::Id::from_u64.nonzero'}
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();profile,env,_=reference.load(rustc=args.rustc);rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-tracing-borrow-')as tmp:
  binary=Path(tmp)/'original';command=[profile['rustc'],*profile['link_arguments'],'--remap-path-prefix='+str(ROOT)+'/=']
  for path in profile['package_artifacts']['tracing'][0]['filenames']:command+=['--extern','tracing='+path]
  subprocess.run([*command,str(HERE/'borrow.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  for mode in ['normal','inherited','processing']:
   for case in SITES:
    run=subprocess.run([str(binary),mode,case],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
    expected=-signal.SIGABRT if mode=='processing'else 0
    if run.returncode!=expected:raise RuntimeError(f'{mode}/{case}: unexpected native status{run.returncode}')
    for line in run.stdout.decode().splitlines():
     if line.startswith('hook:'):
      _,kind,payload,file,line,column,_=line.split(':');site='outer'if bytes.fromhex(payload)==b'outer'else SITES[case];value={'File':bytes.fromhex(file).decode(),'Line':int(line),'Column':int(column)}
      if site in locations and locations[site]!=value:raise RuntimeError('native static source location changed within profile')
      locations[site]=value
    rows.append(mode+'\t'+case+'\t'+('abort'if mode=='processing'else'ok')+'\t'+run.stdout.hex()+'\n')
 raw=''.join(rows).encode()
 if args.check:
  if DEST.read_bytes()!=raw:raise RuntimeError('tracing borrow/zero-ID native drift')
 else:DEST.write_bytes(raw)
 proof={'schema':1,'records':9,'actual_SIGABRT':3,'compiler':profile['rustc_version'],'source_revision':profile['source_revision'],'raw_sha256':hashlib.sha256(raw).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'borrow.rs').read_bytes()).hexdigest(),'locations':locations,'scope':'Actual public set_default and DefaultGuard Drop while get_default holds its source RefCell borrow; actual Id::from_u64(0). Normal, inherited-caught outer unwind and real panic-hook processing contexts. All hook payload/kind/verbatim source locations and frame events retained. Locations are explicit selected-producer profile cells; no Cargo filename inference. Native SIGABRT vs Go exit134 is the explicit host abort class.'}
 path=HERE/'BORROW_VALIDATION.json';render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('borrow/ID proof manifest drift')
 else:path.write_text(render)
 print('9 original borrow/zero-ID cases / 3 actual SIGABRT')
if __name__=='__main__':main()
