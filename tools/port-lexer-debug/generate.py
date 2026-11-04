#!/usr/bin/env python3
"""Original, unmodified lexer/literal source contract evidence; no runtime fallback."""
import argparse,gzip,hashlib,io,json,os,pathlib,shutil,subprocess,sys,tarfile,tempfile,tomllib
ROOT=pathlib.Path(__file__).resolve().parents[2]
TOOL=ROOT/'tools/port-lexer-debug'
REV='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
RUST='rustc 1.99.1 (58cf8f9c3 2026-10-13)'
SOURCES={
 'Cargo.lock':('feeba051664e2fb683fdc8914790a81261b3b5d7','080fd4f801bd66eac0f2e1ec5de21ccd845ec75063922d35efe7a0203e46fcca'),
 'compiler/rustc_lexer/src/lib.rs':('0aa57e75889fa2d81deb1ec8bf5480b1c930fff1','fb7f451aae9d46a171e5599635483b27613a4067d107edd9caf89d79a32ceadf'),
 'compiler/rustc_lexer/src/tests.rs':('a7357ba38c8e4484da814ef3c22bf33b75bdbeb2','90516c7444d6102ab1fceae40cf260cf473ec6542bc372d8bf9d6033eaf507da'),
}
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,cwd=ROOT):
 result=subprocess.run([str(x) for x in args],cwd=cwd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
 if result.returncode:raise RuntimeError(f'{args}: exit {result.returncode}\n{result.stderr.decode(errors="replace")}')
 return result.stdout

def executable(variable,local,fallback):
 value=os.environ.get(variable,str(ROOT/local) if (ROOT/local).exists() else fallback)
 if not value:raise RuntimeError(f'{variable} explicitly empty')
 value=str(ROOT/value) if not pathlib.Path(value).is_absolute() and ("/" in value or "\\" in value) else value
 resolved=shutil.which(value)
 if not resolved:raise RuntimeError(f'{variable}: executable {value!r} unavailable')
 return os.path.abspath(resolved)

def verify_registry(metadata,lock,cargo_home):
 locked={(p['name'],p['version']):p for p in tomllib.loads(lock.decode())['package'] if p.get('source','').startswith('registry+')}
 retained=[]
 for package in metadata['packages']:
  if not package.get('source','') or not package['source'].startswith('registry+'):continue
  key=(package['name'],package['version']);entry=locked[key]
  source=pathlib.Path(package['manifest_path']).parent
  if source.is_symlink() or not source.is_dir():raise RuntimeError(f'nonregular crate root: {source}')
  # Cargo locates registry/src/<registry>/<name-version>; the matching archive
  # must be under that same registry, never an arbitrary archive name search.
  archive=cargo_home/'registry/cache'/source.parent.name/(source.name+'.crate')
  blob=archive.read_bytes()
  if sha(blob)!=entry['checksum']:raise RuntimeError(f'archive checksum changed: {key}')
  expected={}
  with tarfile.open(fileobj=io.BytesIO(blob),mode='r:gz') as tar:
   prefix=source.name+'/'
   for member in tar:
    if member.name==source.name or member.name==prefix:continue
    if not member.name.startswith(prefix):raise RuntimeError(f'archive path outside crate: {member.name}')
    name=member.name[len(prefix):]
    path=pathlib.PurePosixPath(name)
    if '..' in path.parts or path.is_absolute():raise RuntimeError(f'unsafe crate path: {name}')
    if member.isdir():continue
    if not member.isfile():raise RuntimeError(f'nonregular crate member: {name}')
    if name in expected:raise RuntimeError(f'duplicate crate member: {name}')
    expected[name]=tar.extractfile(member).read()
  actual=set()
  for path in source.rglob('*'):
   name=path.relative_to(source).as_posix()
   if path.is_symlink():raise RuntimeError(f'crate source symlink: {path}')
   if path.is_dir():continue
   if not path.is_file():raise RuntimeError(f'nonregular crate source: {path}')
   if name=='.cargo-ok':continue
   actual.add(name)
   if name not in expected or path.read_bytes()!=expected[name]:raise RuntimeError(f'compiled registry source differs: {path}')
  if actual!=expected.keys():raise RuntimeError(f'missing registry sources for {key}: {expected.keys()-actual}')
  retained.append({'name':key[0],'version':key[1],'checksum':entry['checksum'],'compiled_files':len(actual)})
 return sorted(retained,key=lambda p:p['name'])


def main():
 parser=argparse.ArgumentParser();parser.add_argument('-check','--check',action='store_true');parser.add_argument('--scope',choices=['lexer','literal','order','owned-lexer','owned-literal','owned-iterator','processing-lexer','processing-literal','processing-iterator','profiles','fold-next','internal-context','all'],default='all');args=parser.parse_args()
 env=os.environ.copy()
 for key in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','UPDATE_EXPECT','CARGO_BUILD_TARGET']:env.pop(key,None)
 rust=executable('FERRUGO_RUSTC',pathlib.Path('.toolchain/rust/bin/rustc'),'rustc')
 cargo=executable('FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo')
 if run([rust,'--version'],env).decode().strip()!=RUST:raise RuntimeError('formatter reference compiler differs')
 if b'target_pointer_width="64"' not in run([rust,'--print','cfg'],env).splitlines():raise RuntimeError('native corpus requires 64-bit usize')
 if 'CARGO_HOME' in env and not env['CARGO_HOME']:raise RuntimeError('CARGO_HOME explicitly empty')
 cargo_home=pathlib.Path(env.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home')))
 cargo_home=(cargo_home if cargo_home.is_absolute() else ROOT/cargo_home).resolve()
 env.update({'CARGO_PROFILE_DEV_OVERFLOW_CHECKS':'true','CARGO_PROFILE_DEV_DEBUG_ASSERTIONS':'true','CARGO_PROFILE_DEV_OPT_LEVEL':'0','CARGO_PROFILE_DEV_PANIC':'unwind','RUSTC':rust,'CARGO_HOME':str(cargo_home),'GIT_NO_LAZY_FETCH':'1','RUSTC_BOOTSTRAP':'1'})
 if 'FERRUGO_RUST_SOURCE' in env and not env['FERRUGO_RUST_SOURCE']:raise RuntimeError('FERRUGO_RUST_SOURCE explicitly empty')
 git_root=pathlib.Path(env.get('FERRUGO_RUST_SOURCE',str(ROOT/'.toolchain/upstream-rust')))
 git_root=(git_root if git_root.is_absolute() else ROOT/git_root).resolve()
 source={}
 for path,(oid,checksum) in SOURCES.items():
  found=run(['git','-C',git_root,'rev-parse',f'{REV}:{path}'],env).decode().strip();data=run(['git','-C',git_root,'show',f'{REV}:{path}'],env)
  if found!=oid or sha(data)!=checksum:raise RuntimeError(f'pinned source differs: {path}')
  source[path]=data
 support={n:(TOOL/n).read_bytes() for n in ['Cargo.toml','Cargo.lock','oracle.rs','owned.rs','profile.rs']}
 def packages(b):return {(p['name'],p['version']):p['checksum'] for p in tomllib.loads(b.decode())['package'] if p.get('source','').startswith('registry+')}
 known=packages(source['Cargo.lock'])
 for key,checksum in packages(support['Cargo.lock']).items():
  if known.get(key)!=checksum:raise RuntimeError(f'non-upstream dependency: {key}')
 bound_paths=list((TOOL/'validation').glob('*.go'))+list((ROOT/'port/support/rustfmt').glob('*.go'))+[TOOL/n for n in ['Cargo.toml','Cargo.lock','oracle.rs','owned.rs','profile.rs','generate.py','test_generate.py']]
 for package in ['port/rust/compiler/rustc_lexer/src','port/dependencies/rustc-literal-escaper/v0_0_8/src']:
  bound_paths.extend((ROOT/package).glob('*.go'))
 bound_bytes={p.relative_to(ROOT).as_posix():p.read_bytes()for p in sorted(bound_paths)}
 bound={name:sha(data)for name,data in bound_bytes.items()}
 pending=[]
 # Fetching is an explicit CI preparation step; every execution remains offline.
 with tempfile.TemporaryDirectory(prefix='ferrugo-lexer-contracts-') as tmp:
  work=pathlib.Path(tmp)
  for name,data in support.items():(work/name).write_bytes(data)
  for path,data in source.items():
   if path=='Cargo.lock':continue
   target=work/'upstream'/path.removeprefix('compiler/');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
  env['CARGO_TARGET_DIR']=str(work/'target')
  metadata=json.loads(run([cargo,'metadata','--locked','--offline','--format-version=1'],env,work));deps=verify_registry(metadata,support['Cargo.lock'],cargo_home)
  run([cargo,'build','--locked','--offline','--bin','contracts'],env,work)
  tests=run([cargo,'test','--locked','--offline','--bin','contracts','lexer::tests::'],env,work)
  if b'17 passed; 0 failed;' not in tests:raise RuntimeError('expected all17 original lexer tests')
  scopes=['lexer','literal','order','owned-lexer','owned-literal','owned-iterator','processing-lexer','processing-literal','processing-iterator','profiles','fold-next','internal-context'] if args.scope=='all' else [args.scope]
  for scope in scopes:
   if scope=='internal-context':
    lines=[]
    for case in range(42):
     result=subprocess.run([str(work/'target/debug/contracts'),'internal',str(case)],cwd=work,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
     expected=-6 if case%3==2 else 0
     if result.returncode!=expected:raise RuntimeError(f'internal site case{case}: expected{expected}, got{result.returncode}: {result.stderr.decode(errors="replace")}')
     lines.append(f'{case}\t{"ok" if result.returncode==0 else "abort"}\t{result.stdout.hex()}\n'.encode())
    raw=b''.join(lines)
   elif scope=='fold-next':
    lines=[]
    for case in range(3):
     result=subprocess.run([str(work/'target/debug/contracts'),'fold-next',str(case)],cwd=work,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
     if result.returncode not in [0,-6,134]:raise RuntimeError('unexpected fold Next failure class')
     lines.append(f'{case}\t{"ok" if result.returncode==0 else "abort"}\t{result.stdout.hex()}\n'.encode())
    raw=b''.join(lines)
   elif scope=='profiles':
    lines=[]
    for checked in [False,True]:
     for debug in [False,True]:
      env['CARGO_PROFILE_DEV_OVERFLOW_CHECKS']=str(checked).lower();env['CARGO_PROFILE_DEV_DEBUG_ASSERTIONS']=str(debug).lower()
      env['CARGO_TARGET_DIR']=str(work/f'target-c{int(checked)}d{int(debug)}')
      run([cargo,'build','--locked','--offline','--bin','contracts'],env,work)
      binary=pathlib.Path(env['CARGO_TARGET_DIR'])/'debug/contracts'
      for command in ['cursor','iterator']:
       result=run([binary,command],env,work)
       lines.extend(f'{str(checked).lower()}\t{str(debug).lower()}\t'.encode()+line+b'\n' for line in result.splitlines())
    raw=b''.join(lines)
   elif scope.startswith('processing-'):
    target_scope=scope.removeprefix('processing-');count={'lexer':2,'literal':28,'iterator':6}[target_scope];lines=[]
    for case in range(count):
     result=subprocess.run([str(work/'target/debug/contracts'),'processing',target_scope,str(case)],cwd=work,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
     if result.returncode not in [-6,134]:raise RuntimeError(f'processing case{case}: expected abort, status{result.returncode}')
     lines.append(f'{case}\tabort\t{result.stdout.hex()}\n'.encode())
    raw=b''.join(lines)
   elif scope.startswith('owned-'):
    target_scope='literal' if scope=='owned-literal' else 'iterator' if scope=='owned-iterator' else 'lexer'
    count=int(run([work/'target/debug/contracts','owned-count',target_scope],env,work))
    lines=[]
    for case in range(count):
     result=subprocess.run([str(work/'target/debug/contracts'),'owned',target_scope,str(case)],cwd=work,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
     if result.returncode not in [0,-6,134]:raise RuntimeError(f'owned case{case}: unexpected status{result.returncode}: {result.stderr.decode(errors="replace")}')
     lines.append(f'{case}\t{"ok" if result.returncode==0 else "abort"}\t{result.stdout.hex()}\n'.encode())
    raw=b''.join(lines)
   else:raw=run([work/'target/debug/contracts',scope],env,work)
   directory=ROOT/'tools/port-literal-contracts' if scope in ['literal','owned-literal','processing-literal'] else TOOL
   stem='internal-context' if scope=='internal-context' else 'iterator-next' if scope=='fold-next' else 'iterator-processing' if scope=='processing-iterator' else 'processing' if scope.startswith('processing-') else 'iterator-owned' if scope=='owned-iterator' else 'owned' if scope.startswith('owned-') else scope if scope in ['order','profiles'] else 'debug'
   fixture=directory/(stem+'.tsv.gz');manifest=directory/(stem+'-manifest.json')
   info={'format':1,'scope':scope,'original_lexer_tests_passed':17,'bound_sha256':bound,'revision':REV,'rustc':RUST,'records':raw.count(b'\n'),'raw_sha256':sha(raw),'sources':{p:{'git_blob':v[0],'sha256':v[1]}for p,v in SOURCES.items()},'support_sha256':{p:sha(b)for p,b in support.items()},'generator_sha256':sha((TOOL/'generate.py').read_bytes()),'registry_dependencies':deps}
   if fixture.exists() and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
   else:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
   info['fixture_sha256']=sha(compressed)
   if args.check:
    if not fixture.exists() or gzip.decompress(fixture.read_bytes())!=raw:raise RuntimeError(f'native {scope} observations differ')
    if json.loads(manifest.read_text())!=info:raise RuntimeError(f'native {scope} evidence manifest differs')
   else:pending.extend([(fixture,compressed),(manifest,(json.dumps(info,indent=2)+'\n').encode())])
   print(f'Original {scope}: {info["records"]} records; {"checked" if args.check else "retained"}')
  if deps!=verify_registry(metadata,support['Cargo.lock'],cargo_home):raise RuntimeError('registry source changed during build')
  for name,data in bound_bytes.items():
   if (ROOT/name).read_bytes()!=data:raise RuntimeError(f'evidence input changed during validation: {name}')
  for path,data in pending:path.write_bytes(data)
if __name__=='__main__':
 try:main()
 except Exception as e:print(e,file=sys.stderr);sys.exit(1)
