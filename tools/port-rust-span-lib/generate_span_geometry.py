#!/usr/bin/env python3
"""Exercise unchanged public Span geometry/combine APIs, including tracking."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import random
import subprocess
import sys

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
sys.path.insert(0,str(ROOT/'tools/port-rust-symbol'))
import native_profile
import reference
import span_wrapper_evidence

def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,env,data=None):
 p=subprocess.run(command,env=env,input=data,capture_output=True,timeout=180)
 if p.returncode:raise SystemExit(f'geometry command failed:{command[:2]}\n{p.stderr.decode(errors="replace")[:4000]}')
 return p.stdout
def requests():
 maximum=(1<<32)-1;limit=0xffffff00
 positions=[(0,0),(0,1),(1,1),(3,5),(5,3),(10,30),(100,102),(0,32766),(0,32767),(0,32768),(0,65535),(65535,65536),(0,maximum),(maximum-1,maximum),(maximum,maximum)]
 # The already-constructed Span input domain has a 29-bit context field;
 # larger raw context constructor failures belong to span_encoding's proof.
 contexts=[0,1,2,65535,65536,(1<<29)-1];parents=['-','0','7','65535','65536',str(limit)]
 pool=[(*pos,ctxt,parent)for pos in positions for ctxt in contexts for parent in parents]
 result=[];seen=set()
 def add(op,a,b,x=13,y=37,mode=0):
  line='\t'.join(map(str,[op,*a,*b,x,y,mode]))
  if line not in seen:seen.add(line);result.append(line)
 anchor=(10,30,0,'-')
 for a in pool:
  for op in ['lo','hi','lohi','shlo','shhi','empty','withlo','withhi','withctxt','root']:add(op,a,anchor)
  length=abs(a[1]-a[0])
  for x in [0,1,length,min(maximum,length+1),maximum]:add('split',a,anchor,x,0)
  for x,y in [(0,0),(1,2),(maximum,maximum),((1<<32)+1,(1<<32)+2),((1<<64)-1,0)]:add('inner',a,anchor,x,y)
 rng=random.Random(0x5A4A20261104)
 pairs=[(rng.choice(pool),rng.choice(pool))for _ in range(600)]
 for a,b in pairs:
  for op in ['subdummy','contains','overlap','adjacent','trimstart','trimend','neighbor','to','between','until']:add(op,a,b)
 for a,b in pairs[:150]:
  for mode in [1,2,3,7,11,15,19,23,31]:
   for op in ['neighbor','to','between','until']:add(op,a,b,111,177,mode)
 return result
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--prepared',action='store_true');args=p.parse_args()
 rustc,env,compiler=native_profile.compiler_profile();cache=ROOT/'.toolchain/oracles/span-geometry';cache.mkdir(parents=True,exist_ok=True)
 inputs=requests();data=('\n'.join(inputs)+'\n').encode();records=[];profiles=[]
 for debug in [False,True]:
  for overflow in [False,True]:
   name=f'd{int(debug)}o{int(overflow)}';profile,env=reference.load_profile(ROOT/f'.toolchain/oracles/rustc-span-reference/build-profile-{name}.json');rustc=profile['rustc']
   if profile['source_revision']!=native_profile.REVISION or profile['debug_assertions']!=debug or profile['overflow_checks']!=overflow:raise SystemExit('geometry profile differs')
   binary=cache/('oracle-'+name);run([rustc,str(HERE/'native_span_geometry.rs'),*profile['link_arguments'],'-o',str(binary)],env)
   output=run([str(binary)],env,data).decode().splitlines()
   if len(output)!=len(inputs):raise SystemExit('geometry native record count differs')
   records.extend(f'{name}\t{request}\t|\t{response}\n'for request,response in zip(inputs,output))
   profiles.append({'name':name,'debug_assertions':debug,'overflow_checks':overflow,'provenance_sha256':profile['provenance_sha256'],'records':len(inputs),'panics':sum(row.startswith('panic\t')for row in output)})
 raw=''.join(records).encode()
 if len(raw)>64<<20:raise SystemExit('geometry fixture exceeds64MiB')
 base=ROOT/'port/rust/compiler/rustc_span/src';fixture=base/'testdata/lib_span_geometry.tsv.gz';compressed=gzip.compress(raw,compresslevel=9,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 files=sorted(set(span_wrapper_evidence.inputs()+[HERE/'generate_span_geometry.py',HERE/'native_span_geometry.rs']))
 manifest={'status':'partial-lib-validation','source_revision':native_profile.REVISION,**compiler,'scope':'Actual full-crate public closed Span geometry/combine, packing boundaries, parent tracking and metavar read/freeze; hygiene wrapper/runtime-hook proof remains separate.','profiles':profiles,'records':len(records),'requests_per_profile':len(inputs),'operations':sorted({line.split('\t')[0]for line in inputs}),'request_sha256':sha(data),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':[{'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())}for path in files]}
 metadata=HERE/'span-geometry-manifest.json';encoded=(json.dumps(manifest,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or metadata.read_bytes()!=encoded:raise SystemExit('Span geometry evidence differs')
 else:fixture.write_bytes(compressed);metadata.write_bytes(encoded)
 print(f'Span geometry:{len(records)} actual public-API records across4profiles; {len(manifest["operations"])} operation families')
if __name__=='__main__':main()
