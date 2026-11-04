#!/usr/bin/env python3
"""Build complete unchanged rustc_span and its genuine dependency graph.

This development reference grants no source completion credit by itself.
Default execution is offline. Fetch flags prepare exact pinned inputs only.
"""
import argparse,fcntl,hashlib,importlib.util,json,os,pathlib,shutil,subprocess,sys,tomllib

ROOT=pathlib.Path(__file__).resolve().parents[2]
TOOL=ROOT/'tools/port-rust-span-reference'
_verify_spec=importlib.util.spec_from_file_location('rustc_span_reference_verify',TOOL/'verify.py')
_verify_module=importlib.util.module_from_spec(_verify_spec);_verify_spec.loader.exec_module(_verify_module)
verify_registry=_verify_module.verify_registry

def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,cwd=ROOT,body=None):
 result=subprocess.run([str(a)for a in args],cwd=cwd,env=env,input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180)
 if result.returncode:raise RuntimeError(f'{args}: exit{result.returncode}\n{result.stderr.decode(errors="replace")[-12000:]}')
 return result.stdout,result.stderr
def path_option(value,name):
 if not value:raise RuntimeError(f'{name} explicitly empty')
 p=pathlib.Path(value);return (p if p.is_absolute()else ROOT/p).resolve()
def executable(value,name,local):
 if value is None:value=os.environ.get(name,str(ROOT/local))
 if not value:raise RuntimeError(f'{name} explicitly empty')
 # rustc/rustdoc/cargo may be Rustup multicall symlinks. Canonicalizing the
 # link changes argv[0] to rustup and silently selects the wrong dispatch.
 if '/'in value or '\\'in value:value=os.path.abspath(value if os.path.isabs(value)else str(ROOT/value))
 resolved=shutil.which(value)
 if not resolved:raise RuntimeError(f'{name}: executable unavailable:{value}')
 return os.path.abspath(resolved)
def lock_packages(data):
 return {(p['name'],p['version']):p['checksum']for p in tomllib.loads(data.decode())['package']if p.get('source','').startswith('registry+')}

def materialize(graph,source,work,env,fetch):
 git_env=env.copy();git_env['GIT_NO_LAZY_FETCH']='1'
 objects=[f['blob']for f in graph['files']]
 status,_=run(['git','-C',source,'cat-file','--batch-check'],git_env,body=('\n'.join(objects)+'\n').encode())
 missing=[line.split()[0]for line in status.decode().splitlines()if line.endswith(' missing')]
 if missing and not fetch:raise RuntimeError(f'{len(missing)} pinned source blobs missing; prepare with --fetch-source')
 for start in range(0,len(missing),200):
  fetch_env=env.copy();fetch_env.pop('GIT_NO_LAZY_FETCH',None)
  run(['git','-C',source,'-c','fetch.negotiationAlgorithm=noop','fetch','--no-tags','--no-write-fetch-head','--recurse-submodules=no','--filter=blob:none','--stdin','origin'],fetch_env,body=('\n'.join(missing[start:start+200])+'\n').encode())
 contents={}
 for f in graph['files']:
  actual,_=run(['git','-C',source,'rev-parse',graph['revision']+':'+f['path']],git_env)
  if actual.decode().strip()!=f['blob']:raise RuntimeError('pinned Git object differs:'+f['path'])
  data,_=run(['git','-C',source,'cat-file','blob',f['blob']],git_env)
  if len(data)!=f['bytes']or sha(data)!=f['sha256']:raise RuntimeError('pinned source bytes differ:'+f['path'])
  contents[f['path']]=data
  # The complete original root manifest is provenance, not the reduced native
  # workspace manifest. Keeping it above compiler crates would nest workspaces.
  target=work/('original'if f['path'].startswith('compiler/')else'upstream-root')/f['path']
  target.parent.mkdir(parents=True,exist_ok=True)
  if target.is_symlink():raise RuntimeError('original source cache symlink:'+str(target))
  if target.exists()and target.read_bytes()!=data:raise RuntimeError('original source cache bytes changed:'+str(target))
  if not target.exists():target.write_bytes(data)
 return contents

def main():
 parser=argparse.ArgumentParser()
 parser.add_argument('--source');parser.add_argument('--rustc');parser.add_argument('--rustdoc');parser.add_argument('--cargo');parser.add_argument('--cargo-home');parser.add_argument('--output')
 parser.add_argument('--fetch-source',action='store_true');parser.add_argument('--fetch-deps',action='store_true')
 parser.add_argument('--debug-assertions',choices=['true','false'],default='true');parser.add_argument('--overflow-checks',choices=['true','false'],default='true')
 args=parser.parse_args()
 graph=json.loads((TOOL/'sources.json').read_text());selection=json.loads((TOOL/'toolchain.json').read_text())
 env=os.environ.copy()
 for name in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','CARGO_BUILD_TARGET','UPDATE_EXPECT']:env.pop(name,None)
 rustc=executable(args.rustc,'FERRUGO_RUSTC',pathlib.Path('.toolchain/rust-nightly-2026-11-04/bin/rustc'))
 rustdoc=executable(args.rustdoc,'FERRUGO_RUSTDOC',pathlib.Path(rustc).parent/'rustdoc')
 cargo=executable(args.cargo,'FERRUGO_CARGO',pathlib.Path('.toolchain/rust-nightly-2026-11-04/bin/cargo'))
 version,_=run([rustc,'--version'],env);cargo_version,_=run([cargo,'--version'],env);rustdoc_version,_=run([rustdoc,'--version'],env)
 if version.decode().strip()!=selection['rustc']:raise RuntimeError('whole-span compiler identity differs from toolchain.json')
 if cargo_version.decode().strip()!=selection['cargo']:raise RuntimeError('whole-span Cargo identity differs from toolchain.json')
 if rustdoc_version.decode().strip()!=selection['rustc'].replace('rustc ','rustdoc ',1):raise RuntimeError('whole-span rustdoc identity differs from toolchain.json')
 cfg,_=run([rustc,'--print','cfg'],env)
 if b'target_pointer_width="64"'not in cfg.splitlines():raise RuntimeError('reference profile requires64-bit native host')
 source=path_option(args.source if args.source is not None else env.get('FERRUGO_RUST_SOURCE',str(ROOT/'.toolchain/upstream-rust')),'FERRUGO_RUST_SOURCE')
 cargo_home=path_option(args.cargo_home if args.cargo_home is not None else env.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home')),'CARGO_HOME')
 output=path_option(args.output if args.output is not None else str(ROOT/'.toolchain/oracles/rustc-span-reference'),'output')
 if output==source or output.is_relative_to(source)or source.is_relative_to(output):raise RuntimeError('output overlaps upstream source cache')
 marker=output/'reference-cache.json'
 if output.exists()and any(output.iterdir())and not marker.is_file():raise RuntimeError('refusing to overwrite an unowned output directory')
 output.mkdir(parents=True,exist_ok=True)
 ownership={'schema':1,'tool':'port-rust-span-reference','revision':graph['revision']}
 if marker.exists()and json.loads(marker.read_text())!=ownership:raise RuntimeError('output cache ownership/revision differs')
 marker.write_text(json.dumps(ownership,indent=2)+'\n')
 with (output/'.prepare.lock').open('a+b')as cache_lock:
  fcntl.flock(cache_lock.fileno(),fcntl.LOCK_EX)
  checked=args.overflow_checks=='true';debug=args.debug_assertions=='true';profile_name=f'd{int(debug)}o{int(checked)}'
  env.update({'RUSTC':rustc,'RUSTDOC':rustdoc,'RUSTC_BOOTSTRAP':'1','CFG_RELEASE':selection['cfg_release'],'CARGO_HOME':str(cargo_home),'CARGO_TARGET_DIR':str(output/'target'/profile_name),'CARGO_PROFILE_DEV_DEBUG_ASSERTIONS':args.debug_assertions,'CARGO_PROFILE_DEV_OVERFLOW_CHECKS':args.overflow_checks,'CARGO_PROFILE_DEV_OPT_LEVEL':'0','CARGO_PROFILE_DEV_PANIC':'unwind','CARGO_PROFILE_TEST_DEBUG_ASSERTIONS':args.debug_assertions,'CARGO_PROFILE_TEST_OVERFLOW_CHECKS':args.overflow_checks,'CARGO_PROFILE_TEST_OPT_LEVEL':'0','CARGO_PROFILE_TEST_PANIC':'unwind'})
  original=materialize(graph,source,output,env,args.fetch_source)
  selected_lock=(TOOL/'Cargo.lock').read_bytes();known=lock_packages(original['Cargo.lock']);chosen=lock_packages(selected_lock)
  if len(chosen)!=108:raise RuntimeError('expected108 pinned registry dependencies')
  for key,checksum in chosen.items():
   if known.get(key)!=checksum:raise RuntimeError('registry selection differs from original lock:'+str(key))
  workspace='[workspace]\nresolver="2"\nmembers='+json.dumps(['original/'+path for path in graph['crates']])+'\n'
  (output/'Cargo.toml').write_text(workspace);(output/'Cargo.lock').write_bytes(selected_lock)
  if args.fetch_deps:run([cargo,'fetch','--locked','--manifest-path',output/'Cargo.toml'],env,output)
  metadata,_=run([cargo,'metadata','--locked','--offline','--format-version=1'],env,output)
  metadata=json.loads(metadata);dependencies=verify_registry(metadata,selected_lock,cargo_home)
  logs=output/'logs';logs.mkdir(exist_ok=True)
  build,stderr=run([cargo,'build','--locked','--offline','-p','rustc_span','--message-format=json'],env,output)
  (logs/(profile_name+'-build.jsonl')).write_bytes(build);(logs/(profile_name+'-build.stderr')).write_bytes(stderr)
  artifacts={};directories=set()
  for line in build.splitlines():
   row=json.loads(line)
   if row.get('reason')!='compiler-artifact':continue
   files=[p for p in row.get('filenames',[])if p.endswith(('.rlib','.rmeta','.so','.dylib'))]
   if not files:continue
   directories.update(str(pathlib.Path(p).parent)for p in files)
   name=row['target']['name'];library=next((p for p in files if p.endswith(('.rlib','.so','.dylib'))),None)
   artifact={'filenames':files,'library':library,'kinds':row['target']['kind'],'package_id':row['package_id']}
   # Source compiler crate names are unique. Registry target-name duplicates
   # remain available through package_artifacts rather than silently replacing.
   artifacts.setdefault(name,[]).append(artifact)
  compiler_artifacts={}
  for path in graph['crates']:
   name=path.rsplit('/',1)[1];found=artifacts.get(name,[])
   if len(found)!=1:raise RuntimeError('missing/ambiguous original crate artifact:'+name)
   compiler_artifacts[name]=found[0]
  tests,test_stderr=run([cargo,'test','--locked','--offline','-p','rustc_span','--lib'],env,output)
  (logs/(profile_name+'-tests.stdout')).write_bytes(tests);(logs/(profile_name+'-tests.stderr')).write_bytes(test_stderr)
  if b'43 passed; 0 failed;'not in tests:raise RuntimeError('expected all43 original rustc_span tests')
  if dependencies!=verify_registry(metadata,selected_lock,cargo_home):raise RuntimeError('expanded registry sources changed during build')
  for path,data in original.items():
   target=output/('original'if path.startswith('compiler/')else'upstream-root')/path
   if target.read_bytes()!=data:raise RuntimeError('original source changed during build:'+path)
  arguments=['--edition=2024','-C','opt-level=0','-C','debug-assertions='+('yes'if debug else'no'),'-C','overflow-checks='+('yes'if checked else'no')]
  for directory in sorted(directories):arguments.extend(['-L','dependency='+directory])
  for name,artifact in sorted(compiler_artifacts.items()):
   # Cargo/rustc1.101 split full metadata out of rlib. Both files are required
   # for external client compilation and linking, not either file alone.
   for file in artifact['filenames']:arguments.extend(['--extern',name+'='+file])
  smoke=output/('smoke-'+profile_name+'.rs');smoke.write_text('fn main(){rustc_span::create_default_session_globals_then(||{let s=rustc_span::Symbol::intern("actual");println!("{} {:?}",s,rustc_span::SyntaxContext::root());});}\n')
  run([rustc,*arguments,smoke,'-o',output/('smoke-'+profile_name)],env,output);observed,_=run([output/('smoke-'+profile_name)],env,output)
  if observed!=b'actual #0\n':raise RuntimeError('whole-span external linkage smoke differs')
  provenance={'schema':1,'source_revision':graph['revision'],'source_graph_sha256':sha((TOOL/'sources.json').read_bytes()),'source_files':graph['files'],'source_crates':graph['crates'],'registry_dependencies':dependencies,'selected_lock_sha256':sha(selected_lock),'toolchain_sha256':sha((TOOL/'toolchain.json').read_bytes()),'tool_sha256':{name:sha((TOOL/name).read_bytes())for name in ['prepare.py','verify.py']},'rustc':selection['rustc'],'cargo':selection['cargo'],'cfg_release':selection['cfg_release'],'cfg_bootstrap':False,'injected_features':[]}
  provenance_path=output/('source-provenance-'+profile_name+'.json');provenance_path.write_text(json.dumps(provenance,indent=2)+'\n')
  if debug and checked:(output/'source-provenance.json').write_bytes(provenance_path.read_bytes())
  profile={'schema':1,'source_revision':graph['revision'],'rustc':rustc,'rustc_version':selection['rustc'],'rustdoc':rustdoc,'rustdoc_version':rustdoc_version.decode().strip(),'cargo':cargo,'cargo_version':selection['cargo'],'cfg_release':selection['cfg_release'],'cfg_bootstrap':False,'debug_assertions':debug,'overflow_checks':checked,'unicode':selection['unicode'],'original_root':str(output/'original'),'workspace':str(output),'artifacts':compiler_artifacts,'package_artifacts':artifacts,'dependency_library_directories':sorted(directories),'link_arguments':arguments,'upstream_tests_passed':43,'external_smoke':'actual #0','provenance_path':str(provenance_path),'provenance_sha256':sha(provenance_path.read_bytes())}
  path=output/('build-profile-'+profile_name+'.json');path.write_text(json.dumps(profile,indent=2)+'\n')
  if debug and checked:(output/'build-profile.json').write_bytes(path.read_bytes())
  print(f'Whole original rustc_span:10 compiler crates,108 exact registry dependencies,43 original tests and external linkage passed; {profile_name}; {path}')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
