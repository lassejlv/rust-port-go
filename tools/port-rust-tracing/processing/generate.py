#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Keep actual nested panic-hook subscriber failures as isolated abort proofs."""
import argparse,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
HERE=Path(__file__).absolute().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/support/rusttracing/testdata/processing.tsv'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();profile,env,_=reference.load(rustc=args.rustc)
 with tempfile.TemporaryDirectory(prefix='ferrugo-tracing-processing-')as tmp:
  binary=Path(tmp)/'original';command=[profile['rustc'],*profile['link_arguments']]
  for path in profile['package_artifacts']['tracing'][0]['filenames']:command+=['--extern','tracing='+path]
  subprocess.run([*command,str(HERE/'oracle.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  rows=[]
  for case in ['event','span','dispatch']:
   run=subprocess.run([str(binary),case],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
   if run.returncode!=-signal.SIGABRT:raise RuntimeError(f'{case}: expected real SIGABRT, got{run.returncode}')
   rows.append(case+'\tabort\t'+run.stdout.hex()+'\n')
 raw=''.join(rows).encode()
 if args.check:
  if DEST.read_bytes()!=raw:raise RuntimeError('processing subscriber native drift')
 else:DEST.write_bytes(raw)
 proof={'schema':1,'records':3,'actual_SIGABRT':3,'compiler':profile['rustc_version'],'source_revision':profile['source_revision'],'raw_sha256':hashlib.sha256(raw).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'oracle.rs').read_bytes()).hexdigest(),'scope':'Actual tracing event!/span!/Dispatch::new invoked inside a real Rust panic hook; subscriber callback raises a new panic. All stdout callback/owned field/subscriber Drop events retained. Native SIGABRT vs Go exit134 is the explicit host abort representation; terminal runtime stderr is not compared. Raw Go callback local defer timing is outside the translation contract; component-owned field/subscriber cleanup is suppressed at the processing boundary.'}
 path=HERE/'VALIDATION.json';render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('processing proof manifest drift')
 else:path.write_text(render)
 print('3 original tracing processing-panic cases / 3 actual SIGABRT')
if __name__=='__main__':main()
