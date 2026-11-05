#!/usr/bin/env python3
"""Compare Symbol/Ident APIs with the complete unchanged original span crate."""
import argparse
from collections import Counter
import gzip
import hashlib
import json
import os
from pathlib import Path
import random
import itertools
import resource
import subprocess
import sys

import native_profile
import symbol_reference
import generate_predefined

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
FORMATS=[
 ('{}',False,{}),('{:20}',False,{'HasWidth':True,'Width':20}),('{:>20}',False,{'HasWidth':True,'Width':20,'Align':2}),
 ('{:*^20}',False,{'HasWidth':True,'Width':20,'Align':3,'Fill':42}),('{:.0}',False,{'HasPrecision':True,'Precision':0}),
 ('{:.3}',False,{'HasPrecision':True,'Precision':3}),('{:+#020.2}',False,{'SignPlus':True,'Alternate':True,'ZeroPad':True,'HasWidth':True,'Width':20,'HasPrecision':True,'Precision':2}),
 ('{:🦀^20}',False,{'HasWidth':True,'Width':20,'Align':3,'Fill':129408}),('{:\0^20}',False,{'HasWidth':True,'Width':20,'Align':3,'Fill':0}),
 ('{:?}',True,{}),('{:#?}',True,{'Alternate':True}),('{:>20?}',True,{'HasWidth':True,'Width':20,'Align':2}),
 ('{:<20?}',True,{'HasWidth':True,'Width':20,'Align':1}),('{:*^20?}',True,{'HasWidth':True,'Width':20,'Align':3,'Fill':42}),
 ('{:+?}',True,{'SignPlus':True}),('{:020?}',True,{'HasWidth':True,'Width':20,'ZeroPad':True}),
 ('{:x?}',True,{'DebugLowerHex':True}),('{:#x?}',True,{'DebugLowerHex':True,'Alternate':True}),
 ('{:X?}',True,{'DebugUpperHex':True}),('{:#X?}',True,{'DebugUpperHex':True,'Alternate':True}),
 ('{:+#020x?}',True,{'SignPlus':True,'Alternate':True,'ZeroPad':True,'HasWidth':True,'Width':20,'DebugLowerHex':True}),
 ('{:🦀^20?}',True,{'HasWidth':True,'Width':20,'Align':3,'Fill':129408}),('{:\0^20?}',True,{'HasWidth':True,'Width':20,'Align':3,'Fill':0}),
 ('{:.0?}',True,{'HasPrecision':True,'Precision':0}),('{:.3?}',True,{'HasPrecision':True,'Precision':3}),
]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,**kwargs):
 result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180,**kwargs)
 if result.returncode:raise SystemExit(f'command failed: {command}\n{result.stderr.decode(errors="replace")[:4096]}')
 return result.stdout
def hx(value):return value.encode().hex()
def syms(values):return ','.join(map(hx,values)) if values else '-'
def requests():
 result=[]
 for e in range(5):
  for index in range(2421):result.append(f'K\t{e}\t{index}')
 for sequence in ['0','1','2','3','4','0,1,2','4,0,3','0,0,4','4,4,0']:
  result.append(f'U\t0\t{sequence}')
 result.append('I\t0\t'+','.join(['s'+hx('dog'),'b'+hx('dog'),'b'+hx('cat'),'s'+hx('cat'),'s'+hx('dog'),'m'+hx('mutate'),'b','s','bff','b00','s00','s'+hx('🦀')]))
 result.append('I\t0\t'+','.join('m'+bytes([v]).hex() for v in range(256)))
 result.append('I\t0\t'+','.join('b'+v.to_bytes(2,'big').hex() for v in range(65536)))
 for extras in [[],['symbol_driver_extra_a'],['symbol_driver_extra_a','symbol_driver_extra_b'],['Σdriver\0'],['dog','cat'],['symbol_driver_extra_a','symbol_driver_extra_a'],['async'],['🦀'],['1.101.0'],['x','x']]:
  operations=','.join('s'+hx(value)for value in ['symbol_driver_extra_a','dog','cat','symbol_driver_extra_b','Σdriver\0'])
  result.append(f'Z\t0\t{syms(extras)}\t{operations}')
 words=['','a','aaab','aaabc','aaaa','AAAA','foo_bar','bar_foo','forced_capture','force_capture','capture','is_empty','empty','shrunk','rchunks','Σ','ΟΣ','ος','οσ','İ','i\u0307','ß','SS','ﬃ','FFI','ǅ','Ǆ','ǆ','🦀','\0','a\0b','a-b','a_b','ab','ba','abde','bcaed','A\u0301.\u200dΣ','AΣ\u0301.\u200dB']
 rng=random.Random(0x5A4B0A26)
 for lookup in words:
  for candidates in [words,words[::-1],[],[lookup],[lookup,lookup],['a_b','b_a','_ab','ab_']]:
   for dist in ['-','0','1','3','20']:
    for sub in ['0','1']:result.append(f'M\t0\t{syms(candidates)}\t{hx(lookup)}\t{dist}\t{sub}')
  for candidates in [[],words,words[::-1],[lookup.lower()]]:result.append(f'R\t0\t{syms(candidates)}\t{hx(lookup)}')
 for _ in range(2048):
  values=[''.join(rng.choice('abcde_Σσςéδß🦀\0') for _ in range(rng.randrange(20))) for _ in range(rng.randrange(7))]
  lookup=''.join(rng.choice('abcde_Σσςéδß🦀\0') for _ in range(rng.randrange(20)))
  result.append(f'M\t0\t{syms(values)}\t{hx(lookup)}\t{rng.choice(["-","0","1","3","20"])}\t{rng.randrange(2)}')
  result.append(f'R\t0\t{syms(values)}\t{hx(lookup)}')
 for _ in range(512):
  candidates=rng.sample(words,rng.randrange(10));lookups=rng.sample(words,rng.randrange(8))
  result.append(f'N\t0\t{syms(candidates)}\t{syms(lookups)}\t{rng.choice(["-","0","2","20"])}')
 for a in words:
  for b in words:result.append(f'V\t0\t{hx(a)}\t{hx(b)}\t{rng.randbytes(rng.randrange(24)).hex()}')
 for _ in range(512):result.append(f'B\t0\t{rng.randbytes(rng.randrange(96)).hex()}\t{rng.randbytes(rng.randrange(24)).hex()}')
 format_words=['','x','async','dyn','gen','try','crate','$crate',"'break","'gen","'static","'_","'",'ab\0c\n','Σé🦀','\u0378','\U0001e6c0','A\u0301.\u200dΣ',"\"'\\\n\t\r\0"]
 for e in [0,1,3,4]:
  for word in format_words:
   for index,(_,debug,_) in enumerate(FORMATS):
    for kind in ['S','I','N','B']:
     if kind=='B' and not debug:continue
     for fail in [0,1,2,3,8]:result.append(f'F\t{e}\t{kind}\t{hx(word)}\t{index}\t{int(debug)}\t{fail}\t0\t')
 for word in format_words:
  for print_mode in range(4):
   for index,(_,debug,_) in enumerate(FORMATS):
    if debug:continue
    for fail in [0,1,2,3,8]:result.append(f'F\t3\tP\t{hx(word)}\t{index}\t0\t{fail}\t{print_mode}\t')
 for name in ['crate','other_crate','', '$crate','{{root}}','self','Self','super','🦀']:
  for print_mode in range(4):
   for index,(_,debug,_) in enumerate(FORMATS):
    if not debug:result.append(f'F\t3\tA\t{hx("$crate")}\t{index}\t0\t0\t{print_mode}\t{hx(name)}')
 for kind in ['S','B','I']:
  for value in [b'',b'a',b'\0',b'ab',bytes(range(128)),b'x'*127,b'x'*128,b'x'*129,'Σé🦀'.encode()]:
   for lo,hi in [(0,0),(1,2),(2,1),(65535,131072),(0,(1<<32)-1)]:result.append(f'C\t0\t{kind}\t{value.hex()}\t{lo}\t{hi}')
 result.append(f'C\t0\tB\t{bytes(range(256)).hex()}\t0\t0')
 for value in [0,1,2419,2420,2421,0xffffff00,0xffffff01,0xffffffff]:result.append(f'J\t0\t{value}')
 for word in format_words:result.append(f'E\t3\t{hx(word)}')
 for c in [*range(128),0x3a3,0x1f980,0x10ffff]:result.append(f'L\t0\t{c}')
 for kind,values in [('i128',[-(1<<127),-100,-10,-1,0,1,9,10,100,(1<<127)-1]),('u128',[0,1,9,10,100,(1<<128)-1]),('isize',[-(1<<63),-1,0,9,10,(1<<63)-1]),('usize',[0,1,9,10,(1<<64)-1])]:
  for value in values:result.append(f'G\t0\t{kind}\t{value}')
 for mode in range(4):
  for fail in [0,1]:
   if not(fail and mode in [2,3]):result.append(f'O\t0\t{mode}\t{fail}')
 for root_edition in range(5):
  for marks in ['','0','1','2',*(''.join(values)for values in itertools.product('012',repeat=3)),'210120']:
   for name in ['async','dyn','gen','try','x',"'gen"]:result.append(f'H\t{root_edition}\t{marks}\t{hx(name)}')
 for value in ['','a','async','ab\0c','Σé🦀']:
  for lo,hi in [(0,0),(1,2),(2,1),(65535,131072),(0,(1<<32)-1)]:result.append(f'W\t0\t{hx(value)}\t{lo}\t{hi}\t{rng.randbytes(rng.randrange(24)).hex()}')
 for kind in ['new0','new1','dummy','from','from_span','spanpos','quote','macro20','rules','normalizer']:
  for emit in [0,1]:result.append(f'X\t0\t{kind}\t{emit}')
 for a,b,c in itertools.product([0,1,2500,0xffffff00],repeat=3):result.append(f'P\t0\t{a}\t{b}\t{c}')
 return result
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--offline',action='store_true');parser.add_argument('--prepared',action='store_true');args=parser.parse_args()
 profile,env,compiler=symbol_reference.load();rustc=profile['rustc'];resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 if not profile['debug_assertions'] or not profile['overflow_checks'] or profile['cfg_release']!='1.101.0':raise SystemExit('whole-span reference profile must be d1o1/CFG_RELEASE1.101.0')
 provenance=json.loads(Path(profile['provenance_path']).read_text())
 cache=ROOT/'.toolchain/oracles/symbol-runtime';cache.mkdir(parents=True,exist_ok=True)
 wrapper=(HERE/'oracle.rs').read_text()
 for debug,marker in [(False,'DISPLAY_ARMS'),(True,'DEBUG_ARMS')]:
  arms='\n'.join(f'{i}=>fmt::write(sink,format_args!({json.dumps(pattern,ensure_ascii=False).replace(chr(92)+"u0000",chr(92)+"0")},value)),' for i,(pattern,is_debug,_) in enumerate(FORMATS) if is_debug==debug)
  wrapper=wrapper.replace('// '+marker,arms)
 (cache/'oracle.rs').write_text(wrapper)
 run([rustc,str(cache/'oracle.rs'),*profile['link_arguments'],'-C','opt-level=1','-C','debug-assertions=true','-C','overflow-checks=true','-o',str(cache/'oracle')],env=env)
 cases=requests();request=('\n'.join(cases)+'\n').encode()
 output=run([str(cache/'oracle')],input=request,env=env).decode().splitlines()
 if len(output)!=len(cases):raise SystemExit('native Symbol record count mismatch')
 raw=''.join(case+'\t|\t'+value+'\n' for case,value in zip(cases,output)).encode()
 if len(raw)>64<<20:raise SystemExit('native Symbol fixture exceeds64MiB bound')
 aborts=[]
 for mode in [2,3]:
  abort_env=env.copy();abort_env['FERRUGO_SYMBOL_ABORT_TRACE']='1'
  result=subprocess.run([str(cache/'oracle')],input=f'O\t0\t{mode}\t1\n'.encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=abort_env,timeout=10)
  if result.returncode not in [-6,134]:raise SystemExit('native ContextOrd destructor did not abort')
  aborts.append({'mode':mode,'native_exit':result.returncode,'trace':result.stdout.decode().splitlines()})
 base=ROOT/'port/rust/compiler/rustc_span/src';fixture=base/'testdata/symbol_oracle.tsv.gz';manifest=base/'symbol_oracle_manifest.json';compressed=None
 if fixture.exists():
  current=fixture.read_bytes()
  try:
   if gzip.decompress(current)==raw:compressed=current
  except (OSError,EOFError):pass
 if args.check and compressed is None:raise SystemExit('Symbol oracle differs, missing or corrupt')
 if compressed is None:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
 direct_locations={};internal_locations={}
 internal_sites={'dummy':'with_dummy_span','from':'with_dummy_span','from_span':'from_str_and_span','spanpos':'with_span_pos','quote':'without_first_quote','macro20':'normalize_to_macros_2_0','rules':'normalize_to_macro_rules','normalizer':'normalize_to_macro_rules'}
 for case,value in zip(cases,output):
  fields=case.split('\t');parts=value.split('\t')
  if fields[0]=='X'and fields[2]in['new0','new1']:
   direct_locations[fields[2]]={'File':bytes.fromhex(parts[5]).decode(),'Line':int(parts[6]),'Column':int(parts[7]),'Host':False}
  if fields[0]=='X'and fields[2]in internal_sites:
   internal_locations['rustc_span.symbol.ident.'+internal_sites[fields[2]]]={'File':bytes.fromhex(parts[5]).decode(),'Line':int(parts[6]),'Column':int(parts[7])}
 metadata={'rust_revision':native_profile.REVISION,**compiler,'cfg_release':'1.101.0','debug_assertions':True,'overflow_checks':True,'scope':'Symbol/ByteSymbol/Ident/IdentPrinter/MacroRulesNormalizedIdent and edit-distance matching client of complete unchanged rustc_span, actual session/hygiene/macros/hash/codecs; original43 span unit tests pass in shared build. Source18 and constructor caller-location evidence is separately explicit; no Go whole-file completion claim from native source availability.','source_graph_sha256':provenance['source_graph_sha256'],'selected_lock_sha256':provenance['selected_lock_sha256'],'registry_dependencies':provenance['registry_dependencies'],'original_rust_span_tests':profile['upstream_tests_passed'],'requests':len(cases),'records':len(output),'operations':dict(sorted(Counter(case.split('\t')[0] for case in cases).items())),'request_sha256':sha(request),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'formats':[{'pattern':pattern,'debug':debug,'options':{'Fill':32,**options}} for pattern,debug,options in FORMATS],'owned_context_aborts':aborts,'constructor_direct_locations':direct_locations,'caller_location_normalization':'Only the native build-directory prefix is normalized: original helper locations use compiler/rustc_span/src/symbol.rs; direct external constructor sites use symbol-oracle.rs. Line and column are unchanged. Go direct At primary receives the explicit caller witness.','helpers':[{'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())} for path in [HERE/'generate.py',HERE/'oracle.rs',HERE/'native_profile.py']]}
 metadata['constructor_internal_location_profile']=internal_locations
 metadata['reference_inputs']=symbol_reference.inputs()
 encoded=(json.dumps(metadata,ensure_ascii=False,indent=2)+'\n').encode()
 if args.check:
  if not manifest.exists() or manifest.read_bytes()!=encoded:raise SystemExit('Symbol manifest differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 print(json.dumps({'requests':len(cases),'records':len(output),'raw_bytes':len(raw),'compressed_bytes':len(compressed),'owned_aborts':len(aborts)}))
if __name__=='__main__':main()
