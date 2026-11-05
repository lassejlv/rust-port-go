#!/usr/bin/env python3
"""Actual whole UnordBag and SDK Vec operation-context/cleanup observations."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('sort_tools',HERE.parent/'sort/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(b):return hashlib.sha256(b).hexdigest()
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def verify_sources(profile):
 provenance=pathlib.Path(profile['provenance_path']).read_bytes()
 if sha(provenance)!=profile['provenance_sha256']:raise ValueError('Source graph provenance differs')
 for item in json.loads(provenance)['source_files']:
  path=(pathlib.Path(profile['original_root'])if item['path'].startswith('compiler/')else pathlib.Path(profile['workspace'])/'upstream-root')/item['path']
  if path.is_symlink()or not path.is_file()or sha(path.read_bytes())!=item['sha256']:raise ValueError('Source graph differs:'+item['path'])
def compressed_bytes(raw,fixture):
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:return fixture.read_bytes()
 return gzip.compress(raw,mtime=0)
def verify_coverage(rows):
 aborts=sum(r['outcome']=='abort'for r in rows)
 if len(rows)!=228 or aborts!=88:raise ValueError('Coverage differs:'+str((len(rows),aborts)))
 return aborts
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 profile=json.loads(pathlib.Path(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json')).read_text())
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or profile['upstream_tests_passed']!=43:raise SystemExit('Shared reference profile differs')
 verify_sources(profile)
 if subprocess.check_output([profile['rustc'],'--version'],text=True).strip()!=profile['rustc_version']or not profile['debug_assertions']or not profile['overflow_checks']:raise SystemExit('Compiled SDK/profile differs')
 rows=[];locations={}
 with tempfile.TemporaryDirectory(prefix='ferrugo-index-context-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');subprocess.run([profile['rustc'],str((HERE/'oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,check=True)
  for mode in ['normal','unwind','processing']:
   for observer in ['B','none']:
    for action in ['clone_ok','clone_fail','drop_fail','clone_fail_drop','capacity','extend_fail','extend_double','iter_drop_fail','extend_capacity_generic','extend_capacity_trusted','extend_capacity_owned','clone_zst_max','collect_fail','collect_double','clone_from_ok','clone_from_fail','clone_from_old_drop','extend_trusted_none','collect_trusted_none']:
     for resume in [False,True]:
      result=subprocess.run([executable,mode,observer,action,str(resume).lower()],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=no_core,env=dict(os.environ,RUST_BACKTRACE='0'))
      if result.returncode not in [0,-6]:raise SystemExit('Unexpected original exit:'+str(result.returncode)+result.stderr.decode(errors='replace'))
      stdout=result.stdout.decode()
      for line in stdout.splitlines():
       if not line.startswith('HOOK:'):continue
       f=line.split(':');payload=bytes.fromhex(f[2]).decode();key=payload if payload!='outer'else'outer.'+mode
       if payload=='capacity overflow'and action.endswith('_none'):key+='.'+action
       loc={'File':bytes.fromhex(f[6]).decode(),'Line':int(f[7]),'Column':int(f[8])}
       if key in locations and locations[key]!=loc:raise SystemExit('Source locations vary:'+key)
       locations[key]=loc
      rows.append({'mode':mode,'observer':observer,'action':action,'resume':resume,'outcome':'abort'if result.returncode==-6 else'completed','native_exit_code':result.returncode,'stdout':stdout})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);aborts=verify_coverage(rows)
 fixture=ROOT/'port/rust/compiler/rustc_index/src/testdata/runtime_context.jsonl.gz';compressed=compressed_bytes(raw,fixture)
 names=['vec_context.go','vec.go','slice.go','vec_runtime.go','idx.go','vec_context_oracle_test.go','vec_context_test.go']
 manifest={'scope':'Additive current-operation context bridge over genuine IndexVec storage; actual unchanged whole UnordBag and SDK Vec oracle; no std file completion credit','cases':len(rows),'aborts':aborts,'source_revision':profile['source_revision'],'compiler':profile['rustc_version'],'provenance_sha256':profile['provenance_sha256'],'locations':locations,'fixture_sha256':sha(compressed),'raw_sha256':sha(raw),'go_sources':{n:sha((ROOT/'port/rust/compiler/rustc_index/src'/n).read_bytes())for n in names},'reference_sources':{n:sha((HERE/n).read_bytes())for n in ['oracle.rs','generate.py','README.md','test_tools.py']},'sdk_selector_sha256':sha((HERE.parent/'sort/generate.py').read_bytes())}
 metadata=HERE/'manifest.json'
 if a.check:
  if metadata.read_text()!=json.dumps(manifest,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Index context proof differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_index/src','-run','^TestOriginalIndexRuntimeContext$','-count=1'],cwd=ROOT,check=True)
 print(f'{len(rows)} actual Bag/Vec context processes/{aborts} SIGABRT prefixes')
if __name__=='__main__':main()
