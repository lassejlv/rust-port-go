#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/analyze_source_file_panic.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();rows=[];aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-analyzer-panic-') as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;env['FERRUGO_ANALYZE_SOURCE']=str(Path(profile['original_root'])/'compiler/rustc_span/src/analyze_source_file.rs')
    subprocess.run([profile['rustc'],*profile['link_arguments'],'--remap-path-prefix='+profile['original_root']+'=analyzer-original',str(HERE/'panic.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for mode in ['normal','inherited_caught','inherited_escape','processing']:
     for case in ['assert_scan','overflow_line','overflow_pos','ok']:
      result=subprocess.run([str(binary),mode,case],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
      if result.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected analyzer native exit {key}/{mode}/{case}: {result.returncode}')
      kind='ok' if result.returncode==0 else 'abort';aborts+=kind=='abort';rows.append('\t'.join([key,mode,case,kind,result.stdout.hex()])+'\n')
 data=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('analyzer panic native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'records':len(rows),'actual_SIGABRT':aborts,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'panic.rs').read_bytes()).hexdigest(),'scope':'Entire unchanged analyzer include with actual full rustc_span Pos/MultiByteChar/operators. Native source include uses explicit compiler remap profile analyzer-original; no posthoc source-location normalization. Wrapper filename prefix alone removed. Assert-scan and primitive overflow payload/kind/location, partial vectors and normal/inherited/processing abort context.'}
 path=DEST.with_name('analyze_source_file_panic_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('analyzer panic manifest drift')
 else:path.write_text(render)
 print(len(rows),'analyzer native runtime cases,',aborts,'actual SIGABRT')
if __name__=='__main__':main()
