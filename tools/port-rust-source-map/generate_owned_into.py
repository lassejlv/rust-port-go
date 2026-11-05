#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_owned_into.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();rows=[];aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-into-') as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'owned_into.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for mode in ['normal','inherited_caught','inherited_escape','processing']:
     for action in ['normal','body','env','both']:
      for path in range(4):
       for owned in [0,1]:
        r=subprocess.run([str(binary),mode,action,str(path),str(owned)],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
        if r.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected Into native exit {key}/{mode}/{action}/{path}/{owned}: {r.returncode}')
        kind='ok' if r.returncode==0 else 'abort';aborts+=kind=='abort';rows.append('\t'.join([key,mode,action,str(path),str(owned),kind,r.stdout.hex()])+'\n')
 data=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('owned Into native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(rows),'actual_SIGABRT':aborts,'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'owned_into.rs').read_bytes()).hexdigest(),'scope':'Actual public FilePathMapping.to_real_filename with consumed custom Into<Cow<Path>> self, exact borrowed/owned/taken-empty ownership, actual Call/Drop cells, return-place/drop/body panic behavior, normal/inherited/processing+compilerabort hooks. Only wrapper filename prefix removed.'}
 path=DEST.with_name('source_map_owned_into_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('owned Into manifest drift')
 else:path.write_text(render)
 print(len(rows),'actual public Into ownership records,',aborts,'actual SIGABRT')
if __name__=='__main__':main()
