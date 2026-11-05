#!/usr/bin/env python3
"""Retain full-crate Span wrapper observations over genuine expansion graphs."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
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
 result=subprocess.run(command,env=env,input=data,capture_output=True,timeout=180)
 if result.returncode:raise SystemExit(f'Span wrapper command failed:{command[:2]}\n{result.stderr.decode(errors="replace")[:5000]}')
 return result.stdout
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--prepared',action='store_true');args=parser.parse_args()
 rustc,env,compiler=native_profile.compiler_profile()
 inputs=[f'{n}\t{mode}\t{edition}'for n in [0,1,2,3,5,8]for mode in range(16)for edition in range(5)]
 requests=('\n'.join(inputs)+'\n').encode();records=[];profiles=[]
 cache=ROOT/'.toolchain/oracles/span-hygiene-wrappers';cache.mkdir(parents=True,exist_ok=True)
 for debug in [False,True]:
  for overflow in [False,True]:
   name=f'd{int(debug)}o{int(overflow)}';profile,env=reference.load_profile(ROOT/f'.toolchain/oracles/rustc-span-reference/build-profile-{name}.json');rustc=profile['rustc']
   if profile['source_revision']!=native_profile.REVISION or profile['debug_assertions']!=debug or profile['overflow_checks']!=overflow:raise SystemExit('Span wrapper profile differs')
   binary=cache/('oracle-'+name);run([rustc,str(HERE/'native_span_hygiene.rs'),*profile['link_arguments'],'-o',str(binary)],env)
   output=run([str(binary)],env,requests).decode().splitlines()
   if len(output)!=len(inputs):raise SystemExit('Span wrapper native record count differs')
   steps=sum(row.count('|')+1 for row in output)
   records.extend(f'{name}\t{request}\t|\t{response}\n'for request,response in zip(inputs,output))
   profiles.append({'name':name,'debug_assertions':debug,'overflow_checks':overflow,'provenance_sha256':profile['provenance_sha256'],'records':len(inputs),'steps':steps,'caught_panics':sum(row.count('=panic@')for row in output)})
 raw=''.join(records).encode()
 if len(raw)>128<<20:raise SystemExit('Span wrapper fixture exceeds128MiB')
 base=ROOT/'port/rust/compiler/rustc_span/src';fixture=base/'testdata/lib_span_hygiene.tsv.gz';compressed=gzip.compress(raw,compresslevel=9,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 files=sorted(set(span_wrapper_evidence.inputs()+[HERE/'generate_span_hygiene.py',HERE/'native_span_hygiene.rs']))
 manifest={'status':'partial-lib-validation','source_revision':native_profile.REVISION,**compiler,'scope':'Actual complete-crate Span wrappers over valid local expansion graphs: source/parent/callee, editions/features, suggestions, ancestor selection, normalization, marks/adjustments, backtrace iteration, exact SPAN_TRACK sequence and Arc reference counts. SourceMap-dependent methods and hook/owned runtime proofs remain separate.','profiles':profiles,'records':len(records),'steps':sum(p['steps']for p in profiles),'request_sha256':sha(requests),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':[{'path':str(p.relative_to(ROOT)),'sha256':sha(p.read_bytes())}for p in files]}
 metadata=HERE/'span-hygiene-manifest.json';encoded=(json.dumps(manifest,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or metadata.read_bytes()!=encoded:raise SystemExit('Span wrapper evidence differs')
 else:fixture.write_bytes(compressed);metadata.write_bytes(encoded)
 print(f'Span hygiene wrappers:{len(records)} graphs/{manifest["steps"]} operation steps across4profiles')
if __name__=='__main__':main()
