#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_predicate_borrow.tsv.gz'
def requests():
 rows=[]
 for src in ['','abc','αβγ','a𐀀b\u200dγ','\ufeffα\r\nβ','\0\n\t…']:
  for start in [0,1]:
   for end in [0,1,len(src.encode()),len(src.encode())+1]:
    for limit in [0,1,2,10]:rows.append([src.encode().hex(),str(start),str(end),str(limit)])
 return rows
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();req=requests();data_in=''.join(str(i)+'\t'+'\t'.join(r)+'\n' for i,r in enumerate(req)).encode();rows=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-borrow-') as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'predicate_borrow.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60);out=subprocess.run([str(binary)],input=data_in,stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=30).stdout.decode().splitlines()
    if len(out)!=len(req):raise RuntimeError('missing predicate records')
    for i,line in enumerate(out):ident,value=line.split('\t');assert ident==str(i);rows.append(key+'\t'+'\t'.join(req[i])+'\t'+value+'\n')
 data=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('predicate borrowed-cell native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(rows),'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'predicate_borrow.rs').read_bytes()).hexdigest(),'scope':'Actual whole SourceMap span_take_while HRTB FnMut(&char) actual temporary/environment cell alias comparisons, unused/used owned environment Drop order, count/UTF8 boundaries/normalization/early stop/error return. Within-operation equality only; no Rust/Go ABI address equality.'}
 path=DEST.with_name('source_map_predicate_borrow_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('predicate manifest drift')
 else:path.write_text(render)
 print(len(rows),'whole-source borrowed predicate cases')
if __name__=='__main__':main()
