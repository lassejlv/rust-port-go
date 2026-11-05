#!/usr/bin/env python3
"""Observed original get_only/decoder cleanup, actual Drop fields/moves/hooks."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('index_context_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 profile=json.loads(pathlib.Path(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json')).read_text());TOOLS.verify_sources(profile)
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or profile['upstream_tests_passed']!=43 or not profile['debug_assertions']or not profile['overflow_checks']:raise SystemExit('Shared reference profile differs')
 if subprocess.check_output([profile['rustc'],'--version'],text=True).strip()!=profile['rustc_version']:raise SystemExit('SDK differs')
 patterns=[(0,0,0,-1),(2,0,0,-1),(3,0,0,-1),(6,0,0,-1),(2,2,0,-1),(2,0,2,-1),(0,2,0,-1),(0,1,0,-1),(0,0,0,1),(1,0,0,1),(1,0,0,2),(4,0,0,3),(3,0,0,2)]
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-bag-context-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'context_oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for action in ['get_only','decode_vec','decode_bag','decode_map','decode_set','decode_bytes']:
    for length in [0,1,2,4]:
     for body,token,moved,read in patterns:
      for resume in [False,True]:
       config={'length':length,'body':body,'token':token,'moved':moved,'read':read,'resume':resume};arguments=','.join(str(int(config[key]))for key in ['length','body','token','moved','read','resume'])
       result=subprocess.run([executable,mode,action,arguments],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(os.environ,RUST_BACKTRACE='0'))
       if result.returncode not in [0,-6]:raise SystemExit('Original context outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
       stdout=result.stdout.decode()
       for line in stdout.splitlines():
        if not line.startswith('HOOK:'):continue
        f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;loc={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
        if key in locations and locations[key]!=loc:raise SystemExit('Panic locations vary:'+key)
        locations[key]=loc
       rows.append({'mode':mode,'action':action,'config':config,'outcome':'abort'if result.returncode==-6 else'completed','native_exit_code':result.returncode,'stdout':stdout})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(r['outcome']=='abort'for r in rows)
 if len(rows)!=1872 or aborts!=602:raise SystemExit('Context coverage differs:'+str((len(rows),aborts)))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_bag_context.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 names=['tools/port-rust-unord/bag/'+n for n in ['context_oracle.rs','generate_context.py']]+['tools/port-rust-unord/index-context/generate.py']+['port/rust/compiler/rustc_data_structures/src/'+n for n in ['unord_bag.go','unord_bag_codec.go','unord_codec.go','unord_bag_context_test.go']]+['port/rust/compiler/rustc_serialize/src/'+n for n in ['container_decode.go','container_vec.go','opaque_memory.go']]
 manifest={'scope':'Actual original fixed Bag get_only and generic/Unord decoding with real Value/Token owned cells, fields/moves/hooks/resume/cleanup; no std or full Unord credit','cases':len(rows),'aborts':aborts,'source_revision':profile['source_revision'],'compiler':profile['rustc_version'],'provenance_sha256':profile['provenance_sha256'],'locations':locations,'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{n:sha((ROOT/n).read_bytes())for n in names}}
 metadata=HERE/'context_manifest.json'
 if a.check:
  if metadata.read_text()!=json.dumps(manifest,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Original context proof differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordBagContext$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual Bag/decoder context processes/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
