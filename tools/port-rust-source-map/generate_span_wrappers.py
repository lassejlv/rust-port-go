#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_span_wrappers.tsv.gz'
def requests():return [(n,m,k,e) for n in [0,1,3] for m in [0,1,2,3,4,8,16,17,24,31] for k in range(10) for e in [0,3]]
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();req=requests();input=''.join('\t'.join(map(str,r))+'\n' for r in req).encode();rows=[];steps=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-wrappers-') as tmp:
  for d in [False,True]:
   for o in [False,True]:
    profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/f'd{int(d)}o{int(o)}';subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'span_wrappers.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    out=subprocess.run([str(binary)],input=input,stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=60).stdout.splitlines()
    if len(out)!=len(req):raise RuntimeError('missing wrapper records')
    for request,result in zip(req,out):rows.append(f'd{int(d)}o{int(o)}\t'+'\t'.join(map(str,request))+'\t'+result.hex()+'\n');steps+=result.count(b'|')+1
 data=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('SourceMap Span wrapper native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(rows),'steps':steps,'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'span_wrappers.rs').read_bytes()).hexdigest(),'scope':'Complete original Span is_visible/in_external_macro/find_ancestor_not_from_extern_macro/within_macro over real SourceMap local/imported metadata and actual local/registered foreign expansion graphs; metavar insertion/read/freeze/substitution, diagnostic_opaque, lazy external source present/absent, SPAN_TRACK and Arc counts.'}
 path=DEST.with_name('source_map_span_wrappers_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('wrapper manifest drift')
 else:path.write_text(render)
 print(len(rows),'whole-native Span/SourceMap wrapper records,',steps,'steps')
if __name__=='__main__':main()
