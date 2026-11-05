#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_formats_locked.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();out=bytearray()
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-locked-') as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'format_locked.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    data=subprocess.run([str(binary)],stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=30).stdout
    for line in data.splitlines():out.extend(key.encode()+b'\t'+line+b'\n')
 data=bytes(out)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('locked SourceMap format native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(data.splitlines()),'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'format_locked.rs').read_bytes()).hexdigest(),'scope':'Genuine public safe metadata-only import overflow while original files write lock is held; actual hook formats same Arc<SourceMap>. All writer C/S chunks/errors, options, same-Formatter reuse, hook metadata and postpanic files length retained. Only loader-slot pointer digits normalized. Actual native source filename spelling retained; Go profile explicit.'}
 path=DEST.with_name('source_map_formats_locked_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('locked source format manifest drift')
 else:path.write_text(render)
 print(len(data.splitlines()),'whole-source public-trigger locked Debug scripts')
if __name__=='__main__':main()
