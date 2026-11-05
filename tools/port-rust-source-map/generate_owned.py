#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_owned.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--format-error',action='store_true');args=p.parse_args();rows=[];aborts=0
 global DEST
 if args.format_error:DEST=DEST.with_name('source_map_owned_format_error.tsv.gz')
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-owned-') as tmp:
  for d in (False,True):
   for o in (False,True):
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key
    subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'owned_runtime.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for mode in ['normal','inherited_caught','inherited_escape','processing']:
     for case in (['format'] if args.format_error else ['normal','debug','error','loader','error_loader','debug_error_loader']):
      result=subprocess.run([str(binary),mode,case],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
      if result.returncode not in (0,-signal.SIGABRT):raise RuntimeError(f'unexpected native termination {key}/{mode}/{case}: {result.returncode}')
      kind='ok' if result.returncode==0 else 'abort';aborts+=kind=='abort';rows.append(f'{key}\t{mode}\t{case}\t{kind}\t{result.stdout.hex()}\n')
 data=''.join(rows).encode();DEST.parent.mkdir(parents=True,exist_ok=True)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise SystemExit('SourceMap owned native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_sha256':hashlib.sha256(reference.source_bytes('compiler/rustc_span/src/source_map.rs')).hexdigest(),'records':len(rows),'isolated_aborts':aborts,'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'owned_runtime.rs').read_bytes()).hexdigest(),'normalization':'Only the owned_runtime.rs wrapper file prefix is removed. The original/compiler/... file spelling compiled into the actual source graph is retained verbatim and supplied by an explicit Go panic profile. Every hook payload/kind/location, Debug/Drop event and cell identity observation is retained. Native SIGABRT vs Go exit134 is the explicit host abort representation.','complete':False,'remaining':'SourceMap other owned callback/result/external-source failure states and host I/O remain under validation.'}
 if args.format_error:manifest['scope']='Actual full SourceMap expect with custom io::Error.Debug returning fmt::Error after partial output, processing context/local Drop before hook, complete error/loader cleanup across normal/inherited/processing modes. Partial panic payload retained; this is not a Debug panic.'
 path=DEST.with_name('source_map_owned_format_error_manifest.json' if args.format_error else 'source_map_owned_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise SystemExit('SourceMap owned manifest drift')
 else:path.write_text(render)
 print(len(rows),'native owned records,',aborts,'actual SIGABRT cases')
if __name__=='__main__':main()
