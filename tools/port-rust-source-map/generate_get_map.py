#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_getter.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();rows=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-getter-')as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'get_map.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    data=subprocess.run([str(binary)],cwd=ROOT,env=env,stdout=subprocess.PIPE,check=True,timeout=10).stdout;rows.append(key+'\t'+data.hex()+'\n')
 raw=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=raw:raise RuntimeError('get_source_map native drift')
 elif not DEST.exists()or gzip.decompress(DEST.read_bytes())!=raw:DEST.write_bytes(gzip.compress(raw,mtime=0))
 proof={'schema':1,'profiles':len(rows),'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','wrapper_sha256':hashlib.sha256((HERE/'get_map.rs').read_bytes()).hexdigest(),'raw_sha256':hashlib.sha256(raw).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'scope':'Actual public get_source_map in genuine SessionGlobals None/Some scopes, returned Arc pointer identity/strong counts, same-container mutation visibility and final loader destruction. All original crate/source/library bodies unchanged.'}
 path=DEST.with_name('source_map_getter_manifest.json');render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('getter proof manifest drift')
 else:path.write_text(render)
 print(len(rows),'actual full-crate scoped getter/Arc scripts')
if __name__=='__main__':main()
