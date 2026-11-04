#!/usr/bin/env python3
"""Compile unchanged pinned parser bytes and retain native Debug observations."""
import argparse, gzip, hashlib, io, json, os, pathlib, shutil, subprocess, sys, tarfile, tempfile, tomllib
ROOT=pathlib.Path(__file__).resolve().parents[3]
TOOL=ROOT/'tools/port-rust-parse-format/debug'
PARENT=TOOL.parent
REV='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
RUST='rustc 1.99.1 (58cf8f9c3 2026-10-13)'
FIXTURE=ROOT/'port/rust/compiler/rustc_parse_format/src/debug_testdata/native.tsv.gz'
MANIFEST=TOOL/'manifest.json'
SOURCES={
 'Cargo.lock':('feeba051664e2fb683fdc8914790a81261b3b5d7','080fd4f801bd66eac0f2e1ec5de21ccd845ec75063922d35efe7a0203e46fcca'),
 'compiler/rustc_parse_format/src/lib.rs':('90e04fe388ad6f592d5785e8c36964ce284a0da6','451e6c20b1fe8c186fb19c996cc72581220327148154140a307d6d1fafb5b59e'),
 'compiler/rustc_parse_format/src/tests.rs':('a2e03a514fa5fcc8304933ef59e02aa4400c4181','8ad525beb126768ecdb887c4b4a2774db4e419aa4c4f7677b8a545c7405fc531'),
 'compiler/rustc_lexer/src/lib.rs':('0aa57e75889fa2d81deb1ec8bf5480b1c930fff1','fb7f451aae9d46a171e5599635483b27613a4067d107edd9caf89d79a32ceadf'),
 'compiler/rustc_lexer/src/tests.rs':('a7357ba38c8e4484da814ef3c22bf33b75bdbeb2','90516c7444d6102ab1fceae40cf260cf473ec6542bc372d8bf9d6033eaf507da'),
}
SUPPORT=['Cargo.toml','Cargo.lock','upstream/rustc_parse_format/Cargo.toml','upstream/rustc_lexer/Cargo.toml','upstream/rustc_index/Cargo.toml','upstream/rustc_index/src/lib.rs']
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,cwd=ROOT):
 result=subprocess.run([str(x) for x in args],cwd=cwd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 if result.returncode:raise RuntimeError(f'{args}: exit {result.returncode}\n{result.stderr.decode(errors="replace")}')
 return result.stdout

def executable(variable,local,fallback):
 value=os.environ.get(variable,str(ROOT/local) if (ROOT/local).exists() else fallback)
 if not value:raise RuntimeError(f'{variable} explicitly empty')
 resolved=shutil.which(value)
 if not resolved:raise RuntimeError(f'{variable}: executable {value!r} unavailable')
 return str(pathlib.Path(resolved).resolve())

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

def verify_lock(selected,upstream):
 def packages(data):return {(p['name'],p['version']):p['checksum'] for p in tomllib.loads(data.decode())['package'] if p.get('source','').startswith('registry+')}
 known=packages(upstream);chosen=packages(selected)
 if len(chosen)!=16:raise RuntimeError('expected the parent reference workspace\'s 16 pinned registry dependencies')
 for key,checksum in chosen.items():
  if known.get(key)!=checksum:raise RuntimeError(f'reference dependency differs from original upstream lock: {key}')

def main():
 parser=argparse.ArgumentParser();parser.add_argument('-check','--check',action='store_true');args=parser.parse_args()
 env=os.environ.copy()
 env.setdefault('GOCACHE',str(ROOT/'.toolchain/oracles/parsefmt-debug-go-cache'))
 for key in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','RUSTC_BOOTSTRAP','UPDATE_EXPECT','CARGO_BUILD_TARGET']:env.pop(key,None)
 rust=executable('FERRUGO_RUSTC',pathlib.Path('.toolchain/rust/bin/rustc'),'rustc')
 cargo=executable('FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo')
 go=executable('FERRUGO_GO',pathlib.Path('.toolchain/go/bin/go'),'go')
 if run([rust,'--version'],env).decode().strip()!=RUST:raise RuntimeError('formatter reference Rust compiler differs')
 if b'target_pointer_width="64"' not in run([rust,'--print','cfg'],env).splitlines():raise RuntimeError('native corpus requires a 64-bit Rust host')
 if 'CARGO_HOME' in env and not env['CARGO_HOME']:raise RuntimeError('CARGO_HOME explicitly empty')
 cargo_home=pathlib.Path(env.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home'))).resolve()
 env.update({'RUSTC':rust,'CARGO_HOME':str(cargo_home),'GIT_NO_LAZY_FETCH':'1'})
 git_root=pathlib.Path(env.get('FERRUGO_RUST_SOURCE',str(ROOT/'.toolchain/upstream-rust'))).resolve()
 support={name:(PARENT/name).read_bytes() for name in SUPPORT}
 source={}
 for path,(oid,checksum) in SOURCES.items():
  found=run(['git','-C',git_root,'rev-parse',f'{REV}:{path}'],env).decode().strip()
  data=run(['git','-C',git_root,'show',f'{REV}:{path}'],env)
  if found!=oid or sha(data)!=checksum:raise RuntimeError(f'pinned source differs: {path}')
  source[path]=data
 verify_lock(support['Cargo.lock'],source['Cargo.lock'])
 with tempfile.TemporaryDirectory(prefix='ferrugo-parsefmt-debug-') as temporary:
  work=pathlib.Path(temporary)
  for name,data in support.items():
   target=work/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
  for path,data in source.items():
   if path=='Cargo.lock':continue
   target=work/'upstream'/path.removeprefix('compiler/');target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
  (work/'oracle.rs').write_bytes((TOOL/'oracle.rs').read_bytes())
  env['CARGO_TARGET_DIR']=str(work/'target')
  metadata=json.loads(run([cargo,'metadata','--locked','--offline','--format-version=1'],env,work))
  deps=verify_registry(metadata,support['Cargo.lock'],cargo_home)
  run([cargo,'build','--locked','--offline','--bin','ferrugo-parse-format-oracle'],env,work)
  raw=run([work/'target/debug/ferrugo-parse-format-oracle'],env,work)
  if deps!=verify_registry(metadata,support['Cargo.lock'],cargo_home):raise RuntimeError('registry sources changed during build')
 info={'format':1,'revision':REV,'rustc':RUST,'pointer_bits':64,'derived_types':9,'value_cases':40,'format_patterns':19,'records':raw.count(b'\n'),'raw_sha256':sha(raw),'sources':{p:{'git_blob':v[0],'sha256':v[1]}for p,v in SOURCES.items()},'parent_support_sha256':{p:sha(b)for p,b in support.items()},'tool_sha256':{p:sha((TOOL/p).read_bytes())for p in ['oracle.rs','generate.py','test_generate.py','PORT.md']},'registry_dependencies':deps,'scope':'Original nine derived Debug impls and their used std formatter contracts; no parser algorithm or additional source completion credit.'}
 if args.check:
  old=FIXTURE.read_bytes()
  if gzip.decompress(old)!=raw:raise RuntimeError('native Debug observations changed')
  info['fixture_sha256']=sha(old)
  if json.loads(MANIFEST.read_text())!=info:raise RuntimeError('Debug evidence manifest differs')
 else:
  if FIXTURE.exists() and gzip.decompress(FIXTURE.read_bytes())==raw:compressed=FIXTURE.read_bytes()
  else:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
  FIXTURE.write_bytes(compressed);info['fixture_sha256']=sha(compressed);MANIFEST.write_text(json.dumps(info,indent=2)+'\n')
 run([go,'test','./port/rust/compiler/rustc_parse_format/src','-run','^TestOriginalParseFormatDebug$','-count=1'],env)
 print(f'Native parse-format Debug: {info["records"]} records; nine derived types; {"checked" if args.check else "retained"}.')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
