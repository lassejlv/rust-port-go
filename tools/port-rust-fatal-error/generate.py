#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,os,resource,signal,subprocess,tempfile
import reference
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/fatal_error_unwind.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();profile,env,_=reference.load(rustc=args.rustc);rows=[];aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-fatal-') as tmp:
  binary=Path(tmp)/'original';subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'oracle.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  for mode in ['normal','inherited_caught','inherited_escape','processing']:
   for case in ['ok','fatal','ordinary','env_ordinary','env_fatal','body_ordinary','fatal_env_ordinary','ordinary_env_ordinary','fatal_body_ordinary']:
    result=subprocess.run([str(binary),mode,case],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
    if result.returncode not in (0,-signal.SIGABRT):raise RuntimeError(f'unexpected native termination {mode}/{case}: exit {result.returncode}; expected success or SIGABRT')
    kind='ok' if result.returncode==0 else 'abort';aborts+=kind=='abort';rows.append(f'{mode}\t{case}\t{kind}\t{result.stdout.hex()}\n')
  format_binary=Path(tmp)/'format';subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'format.rs'),'-o',str(format_binary)],cwd=ROOT,env=env,check=True,timeout=60)
  format_data=subprocess.run([str(format_binary)],stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=10).stdout
 format_path=DEST.with_name('fatal_error_format.tsv.gz')
 if args.check:
  if gzip.decompress(format_path.read_bytes())!=format_data:raise SystemExit('fatal formatter native trace drift')
 elif not format_path.exists() or gzip.decompress(format_path.read_bytes())!=format_data:format_path.write_bytes(gzip.compress(format_data,mtime=0))
 data=''.join(rows).encode();DEST.parent.mkdir(parents=True,exist_ok=True)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise SystemExit('fatal native trace drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 source=reference.source_bytes('compiler/rustc_span/src/fatal_error.rs');manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_sha256':hashlib.sha256(source).hexdigest(),'records':len(rows),'isolated_aborts':aborts,'rustc_version':profile['rustc_version'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'oracle.rs').read_bytes()).hexdigest(),'format_records':len(format_data.splitlines()),'format_raw_sha256':hashlib.sha256(format_data).hexdigest(),'format_gzip_sha256':hashlib.sha256(format_path.read_bytes()).hexdigest(),'format_wrapper_sha256':hashlib.sha256((HERE/'format.rs').read_bytes()).hexdigest(),'abort_representation':'Native SIGABRT vs Go exit134 normalized to abort. Complete callback witnesses dispatch ordinary source panic hook and compiler cleanup-abort diagnostic; Raise/resume never dispatch one.'}
 path=DEST.with_name('fatal_error_unwind_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise SystemExit('fatal manifest drift')
 else:path.write_text(render)
 print(len(rows),'native records,',aborts,'isolated aborts')
if __name__=='__main__':main()
