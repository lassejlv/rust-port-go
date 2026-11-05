#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
import producer
HERE=Path(__file__).absolute().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/support/rusttracing/testdata/registry_poison.tsv'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--output-dir',help='independent producer manifest and verbatim raw TSV directory');args=p.parse_args();profile,env,_=reference.load(rustc=args.rustc);rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-tracing-poison-')as tmp:
  binary=Path(tmp)/'original';command=[profile['rustc'],*profile['link_arguments'],'--remap-path-prefix='+str(ROOT)+'/=']
  for path in profile['package_artifacts']['tracing'][0]['filenames']:command+=['--extern','tracing='+path]
  subprocess.run([*command,str(HERE/'registry.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  for mode,case in [(mode,case)for mode in ['normal','inherited']for case in ['write0','write2','read2']]+[('processing_write','write2'),('processing_read','write2')]:
   run=subprocess.run([str(binary),mode,case],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
   abort=mode.startswith('processing')
   if run.returncode!=(-signal.SIGABRT if abort else 0):raise RuntimeError(f'{mode}/{case}: status{run.returncode}')
   for line in run.stdout.decode().splitlines():
    if not line.startswith('hook:'):continue
    _,kind,payload,file,line,column,_=line.split(':');payload=bytes.fromhex(payload).decode();site={'outer':'outer','register':'register','callsite':'callsite'}.get(payload)
    if site is None:site='tracing_core::callsite::Dispatchers::register_dispatch.write_unwrap'if int(line)==552 else'tracing_core::callsite::Dispatchers::rebuilder.read_unwrap'
    value={'File':bytes.fromhex(file).decode(),'Line':int(line),'Column':int(column)}
    if site in locations and locations[site]!=value:raise RuntimeError('source profile location drift')
    locations[site]=value
   rows.append(mode+'\t'+case+'\t'+('abort'if abort else'ok')+'\t'+run.stdout.hex()+'\n')
 raw=''.join(rows).encode()
 proof={'schema':1,'records':8,'actual_SIGABRT':2,'compiler':profile['rustc_version'],'source_revision':profile['source_revision'],'raw_sha256':hashlib.sha256(raw).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'registry.rs').read_bytes()).hexdigest(),'locations':locations,'scope':'Actual complete tracing registry after caught subscriber on_register_dispatch writer panic and register_callsite reader panic, with one-dispatch optimization and multiple live registrars. Subsequent Dispatch::new/rebuild, all subscriber Drop states, hooks and normal/inherited-caught/processing contexts retained. Only writers beginning outside panic poison. The standard poisoned read/write unwrap owns its guard until the source hook returns. Native SIGABRT vs Go exit134 is the explicit abort class.'}
 producer.persist('registry',raw,proof,args.output_dir,args.check)
 print('8 original registry poison scripts / 2 actual SIGABRT')
if __name__=='__main__':main()
