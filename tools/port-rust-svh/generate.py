#!/usr/bin/env python3
"""Generate observations from the unchanged full compiler crate, never Go."""
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

# Each pattern supplies real Rust Formatter flags, not a test-only constructor.
FORMATS=[
 ('{}',False,{}),('{:36}',False,{'HasWidth':True,'Width':36}),('{:>36}',False,{'HasWidth':True,'Width':36,'Align':2}),
 ('{:*^36}',False,{'HasWidth':True,'Width':36,'Align':3,'Fill':42}),('{:.0}',False,{'HasPrecision':True,'Precision':0}),
 ('{:.5}',False,{'HasPrecision':True,'Precision':5}),('{:+#036.5}',False,{'SignPlus':True,'Alternate':True,'ZeroPad':True,'HasWidth':True,'Width':36,'HasPrecision':True,'Precision':5}),
 ('{:🦀^36}',False,{'HasWidth':True,'Width':36,'Align':3,'Fill':129408}),('{:\0^36}',False,{'HasWidth':True,'Width':36,'Align':3,'Fill':0}),
 ('{:?}',True,{}),('{:#?}',True,{'Alternate':True}),('{:>36?}',True,{'HasWidth':True,'Width':36,'Align':2}),
 ('{:*^36?}',True,{'HasWidth':True,'Width':36,'Align':3,'Fill':42}),('{:+?}',True,{'SignPlus':True}),
 ('{:036?}',True,{'HasWidth':True,'Width':36,'ZeroPad':True}),('{:x?}',True,{'DebugLowerHex':True}),
 ('{:#x?}',True,{'DebugLowerHex':True,'Alternate':True}),('{:#X?}',True,{'DebugUpperHex':True,'Alternate':True}),
 ('{:+#036x?}',True,{'SignPlus':True,'Alternate':True,'ZeroPad':True,'HasWidth':True,'Width':36,'DebugLowerHex':True}),
 ('{:🦀^36?}',True,{'HasWidth':True,'Width':36,'Align':3,'Fill':129408}),('{:\0^36?}',True,{'HasWidth':True,'Width':36,'Align':3,'Fill':0}),
 ('{:.0?}',True,{'HasPrecision':True,'Precision':0}),('{:.5?}',True,{'HasPrecision':True,'Precision':5}),
]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,env,data=None):
 p=subprocess.run(command,env=env,input=data,capture_output=True,timeout=180)
 if p.returncode:raise SystemExit(f'command failed:{command[:2]}\n{p.stderr.decode(errors="replace")[:6000]}')
 return p.stdout
def invoke(binary,env,requests):
 if not requests:return []
 output=run([str(binary)],env,('\n'.join(requests)+'\n').encode()).decode().splitlines()
 if len(output)!=len(requests):raise SystemExit('native count differs')
 return output
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--prepared',action='store_true');args=parser.parse_args()
 rustc,env,compiler=native_profile.compiler_profile()
 profile_file=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json'
 if not args.prepared:run([sys.executable,str(ROOT/'tools/port-rust-span-reference/prepare.py')],env)
 profile=json.loads(profile_file.read_text())
 if profile['source_revision']!=native_profile.REVISION or not profile['debug_assertions'] or not profile['overflow_checks']:raise SystemExit('reference profile differs')
 work=ROOT/'.toolchain/oracles/svh';work.mkdir(parents=True,exist_ok=True)
 arms='\n'.join(f'{index} => fmt::write(sink, format_args!({json.dumps(pattern,ensure_ascii=False).replace(chr(92)+"u0000",chr(92)+"0")}, observed)),'for index,(pattern,_,_)in enumerate(FORMATS))
 body=(HERE/'oracle.rs').read_text().replace('// FORMAT_ARMS',arms);(work/'oracle.rs').write_text(body)
 binary=work/'oracle';run([rustc,str(work/'oracle.rs'),*profile['link_arguments'],'-o',str(binary)],env)
 maximum=(1<<64)-1
 pairs=[(0,0),(1,0),(0,1),(1,2),(0xf,0x10),(maximum,0),(maximum,maximum),(1<<63,1<<63)]
 rng=random.Random(0x5A42026);pairs += [(rng.getrandbits(64),rng.getrandbits(64))for _ in range(128)]
 ordinary=[]
 for lo,hi in pairs:
  ordinary.append(f'V\t{lo:016x}\t{hi:016x}')
  for kind in ['raw','typed']:
   for count in [0,1,2]:ordinary.append(f'H\t{lo:016x}\t{hi:016x}\t{kind}\t{count}')
 baselines=[]
 for lo,hi in pairs[:8]:
  for index,(_,debug,_)in enumerate(FORMATS):
   for kind in (['S','F']if debug else['S']):
    for chars in [0,1]:
     for reuse in [0,1]:baselines.append(f'F\t{lo:016x}\t{hi:016x}\t{kind}\t{index}\t-1\t{chars}\t{reuse}')
 success=invoke(binary,env,baselines)
 failures=[]
 for request,response in zip(baselines,success):
  events=response.split('\t')[2]
  for fail in range(len(events.split('|'))if events else 0):
   parts=request.split('\t');parts[5]=str(fail);failures.append('\t'.join(parts))
 requests=ordinary+baselines+failures
 responses=invoke(binary,env,ordinary)+success+invoke(binary,env,failures)
 assertion_profiles=[]
 for debug in [False,True]:
  if not debug and not args.prepared:run([sys.executable,str(ROOT/'tools/port-rust-span-reference/prepare.py'),'--debug-assertions','false'],env)
  selected=json.loads((ROOT/f'.toolchain/oracles/rustc-span-reference/build-profile-d{int(debug)}o1.json').read_text())
  if selected['debug_assertions']!=debug or not selected['overflow_checks']:raise SystemExit('assertion profile differs')
  extra_binary=work/('unhash-'+str(int(debug)))
  run([rustc,str(work/'oracle.rs'),*selected['link_arguments'],'-o',str(extra_binary)],env)
  extra=[]
  for lo,hi in pairs:
   for kind in ['direct','reference','wrapped']:
    for seed in [0,7]:
     for count in [0,1,2]:extra.append(f'U\t{lo:016x}\t{hi:016x}\t{kind}\t{seed}\t{count}\t{int(debug)}')
  requests.extend(extra);responses.extend(invoke(extra_binary,env,extra))
  assertion_profiles.append({'debug_assertions':debug,'overflow_checks':True,'provenance_sha256':selected['provenance_sha256'],'cases':len(extra)})
 raw=''.join(request+'\t|\t'+response+'\n'for request,response in zip(requests,responses)).encode()
 if len(raw)>64<<20:raise SystemExit('native fixture exceeds64MiB')
 base=ROOT/'port/rust/compiler/rustc_data_structures/src'
 fixture=base/'testdata/svh.tsv.gz';compressed=gzip.compress(raw,compresslevel=9,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 source=native_profile.source_bytes('compiler/rustc_data_structures/src/svh.rs')
 paths=[HERE/'generate.py',HERE/'oracle.rs',ROOT/'tools/port-rust-symbol/native_profile.py',base/'svh.go',base/'fingerprint_format_primary.go',base/'svh_oracle_test.go']
 metadata={'complete':True,'scope':'Complete unchanged Svh source: construction/projection/copy/equality, raw+typed Hash/hash_slice, exact/ref/wrapped Unhasher in both assertion profiles, actual codec/stable hash and full mutable formatter. Fingerprint trait callback protocols are composed without another owned argument or cleanup boundary. No surrounding compiler/std/dependency completion credit.','rust_revision':native_profile.REVISION,**compiler,'source_git_blob':hashlib.sha1(f'blob {len(source)}\0'.encode()+source).hexdigest(),'source_sha256':sha(source),'provenance_sha256':profile['provenance_sha256'],'assertion_profiles':assertion_profiles,'records':len(requests),'formatter_baselines':len(baselines),'writer_failure_cuts':len(failures),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'formats':[{'pattern':p,'debug':d,'options':{'Fill':32,**o}}for p,d,o in FORMATS],'files':[{'path':str(p.relative_to(ROOT)),'sha256':sha(p.read_bytes())}for p in paths]}
 manifest=HERE/'manifest.json';encoded=(json.dumps(metadata,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or manifest.read_bytes()!=encoded:raise SystemExit('Svh native evidence differs')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'Svh:{len(requests)} native records,{len(failures)} exact write-failure cuts')
if __name__=='__main__':main()
