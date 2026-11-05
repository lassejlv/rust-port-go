#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,hashlib,json,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_sparse_file.tsv'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args()
 if sys.platform!='linux':raise RuntimeError('retained sparse-file producer is the declared native Linux filesystem')
 profile,env,_=reference.load(rustc=args.rustc)
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-sparse-')as tmp:
  binary=Path(tmp)/'original';subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'sparse_file.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  data=subprocess.run([str(binary)],cwd=ROOT,env=env,stdout=subprocess.PIPE,check=True,timeout=15).stdout
 if len(data.splitlines())!=2:raise RuntimeError('missing actual oversized sparse-file records')
 if args.check:
  if DEST.read_bytes()!=data:raise RuntimeError('oversized sparse-file native drift')
 else:DEST.write_bytes(data)
 proof={'schema':1,'records':2,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_sha256':hashlib.sha256(reference.source_bytes('compiler/rustc_span/src/source_map.rs')).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'sparse_file.rs').read_bytes()).hexdigest(),'raw_sha256':hashlib.sha256(data).hexdigest(),'scope':'Actual complete original RealFileLoader with physical Linux sparse files of lengths u32::MAX and u32::MAX+1. Metadata rejection precedes any read or huge allocation. Only filesystem metadata is large; both files have zero data blocks and are removed.'}
 path=DEST.with_name('source_map_sparse_file_manifest.json');render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('sparse-file proof manifest drift')
 else:path.write_text(render)
 print('2 native oversized sparse-file records')
if __name__=='__main__':main()
