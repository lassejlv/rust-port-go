#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_owned_extract.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--predicates',action='store_true');args=p.parse_args();rows=[];aborts=0
 global DEST
 if args.predicates:DEST=DEST.with_name('source_map_owned_predicates.tsv.gz')
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-extract-') as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'owned_extract.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for operation in (['while','prev','take'] if args.predicates else ['extract']):
     for mode in ['normal','inherited_caught','inherited_escape','processing']:
      for input in ['local','external','bad','distinct']:
       for action in ['normal','body','env','both']:
        result=subprocess.run([str(binary),mode,input,action,operation],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
        if result.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native exit {key}/{mode}/{input}/{action}: {result.returncode}')
        kind='ok' if result.returncode==0 else 'abort';aborts+=kind=='abort';rows.append('\t'.join(([key,operation,mode,input,action,kind,result.stdout.hex()] if args.predicates else [key,mode,input,action,kind,result.stdout.hex()]))+'\n')
 data=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('owned extraction native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(rows),'actual_SIGABRT':aborts,'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'owned_extract.rs').read_bytes()).hexdigest(),'operation_scope':('while/prev/take owned predicates' if args.predicates else 'generic extraction'),'scope':'Whole-original generic span_to_source FnMut/value destruction, real local/imported/external SourceFile cells, source-string aliases, Arc counts, external guard release before Fn Drop, unused captures, normal/inherited/processing/body/drop panic and cleanup-abort hooks. Only native wrapper filename prefix removed; SDK/compiler source hook spellings retained.'}
 path=DEST.with_name('source_map_owned_predicates_manifest.json' if args.predicates else 'source_map_owned_extract_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('owned extraction manifest drift')
 else:path.write_text(render)
 print(len(rows),'whole-source owned extraction records,',aborts,'actual SIGABRT')
if __name__=='__main__':main()
