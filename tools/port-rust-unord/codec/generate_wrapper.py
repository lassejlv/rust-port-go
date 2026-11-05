#!/usr/bin/env python3
"""Observe exact opaque specialization versus distinct encoder wrappers."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('index_context_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
spec=importlib.util.spec_from_file_location('verified_span_reference',ROOT/'tools/port-rust-span-lib/reference.py');REFERENCE=importlib.util.module_from_spec(spec);spec.loader.exec_module(REFERENCE)
def load_reference():
 return REFERENCE.load_profile(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 profile,environment=load_reference()
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or not profile['debug_assertions']or not profile['overflow_checks']:raise SystemExit('Wrapper reference profile differs')
 if subprocess.check_output([profile['rustc'],'--version'],text=True).strip()!=profile['rustc_version']:raise SystemExit('Wrapper SDK differs')
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-vector-wrapper-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'wrapper_oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  for mode in ['normal','unwind','processing']:
   for kind in range(4):
    for bag in [False,True]:
     for length in [0,1,31]:
      for fail in sorted({-1,0,1,length-1,length}):
       for resume in [False,True]:
        values=[kind,bag,length,fail,resume];arguments=','.join(str(int(value))for value in values)
        result=subprocess.run([executable,mode,arguments,str(pathlib.Path(temporary)/'bytes')],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=TOOLS.no_core,env=dict(environment,RUST_BACKTRACE='0'))
        if result.returncode not in [0,-6]:raise SystemExit('Wrapper native outcome differs:'+str(result.returncode)+result.stderr.decode(errors='replace'))
        stdout=result.stdout.decode()
        for line in stdout.splitlines():
         if not line.startswith('HOOK:'):continue
         f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode;location={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
         if key in locations and locations[key]!=location:raise SystemExit('Wrapper source location varies:'+key)
         locations[key]=location
        rows.append(dict(mode=mode,kind=kind,bag=bag,length=length,fail=fail,resume=resume,outcome='abort'if result.returncode==-6 else'completed',native_exit_code=result.returncode,stdout=stdout))
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=sum(row['outcome']=='abort'for row in rows)
 if len(rows)!=528 or aborts!=32:raise SystemExit('Wrapper coverage differs:'+str((len(rows),aborts)))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_vector_wrapper.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 names=['port/rust/compiler/rustc_serialize/src/'+name for name in ['container_vec.go','opaque_memory.go','opaque_file.go']]+['port/rust/compiler/rustc_data_structures/src/'+name for name in ['unord_bag.go','unord_bag_codec.go','unord_vector_wrapper_test.go']]+['tools/port-rust-unord/codec/'+name for name in ['wrapper_oracle.rs','generate_wrapper.py','README.md','test_wrapper_tools.py']]
 metadata=dict(scope='Actual unchanged concrete Mem/File opaque byte specialization versus distinct Wrapper<E> generic Vec/UnordBag encoding, emitted byte calls/capacity, method-local Drop and fresh/resume/current panic contexts',cases=len(rows),aborts=aborts,source_revision=profile['source_revision'],compiler=profile['rustc_version'],provenance_sha256=profile['provenance_sha256'],reference_inputs=REFERENCE.tool_inputs(),locations=locations,raw_sha256=sha(raw),fixture_sha256=sha(compressed),files={name:sha((ROOT/name).read_bytes())for name in names});path=HERE/'wrapper_manifest.json'
 if a.check:
  if path.read_text()!=json.dumps(metadata,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Wrapper proof differs')
 else:fixture.write_bytes(compressed);path.write_text(json.dumps(metadata,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordVectorWrapper$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual direct/wrapped vector cases/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
