#!/usr/bin/env python3
"""Assemble exact pinned source files and scoped dependency contracts."""
import argparse, hashlib, io, json, os, pathlib, shutil, subprocess, sys, tarfile, tomllib
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
PIN='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
RUST='rustc 1.99.1 (58cf8f9c3 2026-10-13)'
PATHS=['compiler/rustc_span/src/span_encoding.rs','compiler/rustc_span/src/lib.rs','compiler/rustc_span/src/def_id.rs','compiler/rustc_span/src/hygiene.rs','compiler/rustc_data_structures/src/fx.rs','compiler/rustc_data_structures/src/atomic_ref.rs','compiler/rustc_data_structures/src/stable_hash.rs','compiler/rustc_data_structures/src/sync.rs','compiler/rustc_data_structures/src/sync/lock.rs','compiler/rustc_data_structures/src/marker.rs','compiler/rustc_serialize/src/int_overflow.rs','compiler/rustc_index/src/idx.rs','compiler/rustc_index_macros/src/lib.rs','compiler/rustc_index_macros/src/newtype.rs','compiler/rustc_index_macros/Cargo.toml','Cargo.lock','src/version','src/stage0']
def sha(data):return hashlib.sha256(data).hexdigest()
def execute(args,env,cwd=ROOT):
 result=subprocess.run([str(v)for v in args],cwd=cwd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180)
 if result.returncode:raise RuntimeError(f'{args}: exit{result.returncode}\n{result.stdout.decode(errors="replace")}\n{result.stderr.decode(errors="replace")}')
 return result.stdout

def selected(variable,local,fallback):
 name=os.environ.get(variable,str(ROOT/local)if(ROOT/local).exists()else fallback)
 if not name:raise RuntimeError(f'{variable} explicitly empty')
 executable=shutil.which(name)
 if not executable:raise RuntimeError(f'{variable}: executable {name!r} not found')
 return str(pathlib.Path(executable).resolve())
def environment():
 env=os.environ.copy()
 for key in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','CARGO_BUILD_TARGET','RUSTC_BOOTSTRAP','UPDATE_EXPECT']:env.pop(key,None)
 env['RUSTC']=selected('FERRUGO_RUSTC',pathlib.Path('.toolchain/rust/bin/rustc'),'rustc')
 env['RUSTC_BOOTSTRAP']='1';env['GIT_NO_LAZY_FETCH']='1'
 if 'CARGO_HOME'in env and not env['CARGO_HOME']:raise RuntimeError('CARGO_HOME explicitly empty')
 env['CARGO_HOME']=str(pathlib.Path(env.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home'))).resolve())
 env.setdefault('GOCACHE',str(ROOT/'.toolchain/oracles/span-encoding-go-cache'))
 return env

def write(path,data):
 path.parent.mkdir(parents=True,exist_ok=True)
 if not path.exists()or path.read_bytes()!=data:path.write_bytes(data)

def rust_block(data,needle,start=None):
 # Exact selected source bytes; lexical scanning skips comments and literals so
 # braces in source diagnostics are never mistaken for the item's closing brace.
 first=data.index(needle,start or 0);brace=data.index(b'{',first);i=brace;depth=0;comment=0;string=False;escape=False
 while i<len(data):
  c=data[i]
  if comment:
   if data[i:i+2]==b'/*':comment+=1;i+=2;continue
   if data[i:i+2]==b'*/':comment-=1;i+=2;continue
   i+=1;continue
  if string:
   if escape:escape=False
   elif c==92:escape=True
   elif c==34:string=False
   i+=1;continue
  if data[i:i+2]==b'//':
   end=data.find(b'\n',i);i=len(data)if end<0 else end+1;continue
  if data[i:i+2]==b'/*':comment=1;i+=2;continue
  if c==34:string=True
  elif c==123:depth+=1
  elif c==125:
   depth-=1
   if depth==0:return first,i+1,data[first:i+1]
  i+=1
 raise RuntimeError(f'unterminated source block {needle!r}')

def declaration(data,needle,derive=False):
 start=data.index(needle);end=data.index(b';',start)+1
 if b'struct 'in needle and b'{'in data[start:end]:start,end,_=rust_block(data,needle)
 if derive:start=data.rfind(b'#[derive(',0,start)
 return start,end,data[start:end]

def prepare(destination,env):
 upstream=pathlib.Path(os.environ.get('FERRUGO_RUST_SOURCE',str(ROOT/'.toolchain/upstream-rust'))).resolve()
 original={};provenance=[];fragments=[]
 for path in PATHS:
  oid=execute(['git','-C',upstream,'rev-parse',f'{PIN}:{path}'],env).decode().strip()
  data=execute(['git','-C',upstream,'show',oid],env)
  if hashlib.sha1(f'blob {len(data)}\0'.encode()+data).hexdigest()!=oid:raise RuntimeError(f'Git blob bytes differ:{path}')
  original[path]=data;write(destination/'original'/path,data)
  provenance.append({'path':path,'git_blob':oid,'sha256':sha(data),'bytes':len(data)})
 def fragment(name,path,selection):
  start,end,data=selection
  if original[path][start:end]!=data:raise RuntimeError(f'fragment bytes differ:{name}')
  fragments.append({'name':name,'source':path,'start_byte':start,'end_byte':end,'sha256':sha(data),'bytes':len(data)})
  write(destination/'fragments'/(name+'.rs'),data)
  return data
 lib='compiler/rustc_span/src/lib.rs';defs='compiler/rustc_span/src/def_id.rs';hygiene='compiler/rustc_span/src/hygiene.rs';stable='compiler/rustc_data_structures/src/stable_hash.rs'
 def item(name,path,needle):return fragment(name,path,rust_block(original[path],needle))
 def declared(name,path,needle,derive=False):return fragment(name,path,declaration(original[path],needle,derive))
 def invocation(name,path,typename):
  source=original[path];where=source.index(b'pub struct '+typename);begin=source.rfind(b'rustc_index::newtype_index!',0,where)
  return fragment(name,path,rust_block(source,b'rustc_index::newtype_index!',begin))
 defindex=invocation('def_index',defs,b'DefIndex');localdef=declared('local_def_id',defs,b'pub struct LocalDefId',True)
 context=invocation('syntax_context',hygiene,b'SyntaxContext')
 context_impl=rust_block(original[hygiene],b'impl SyntaxContext {')
 root=fragment('syntax_context_root',hygiene,rust_block(original[hygiene],b'pub const fn root()',context_impl[0]))
 isroot=fragment('syntax_context_is_root',hygiene,rust_block(original[hygiene],b'pub const fn is_root(',context_impl[0]))
 write(destination/'identities.rs',b'pub mod def_id {\n'+defindex+b'\n'+localdef+b'\n}\npub mod hygiene {\n'+context+b'\nimpl SyntaxContext {\n'+root+b'\n'+isroot+b'\n}\n}\npub use hygiene::SyntaxContext;\n')
 bytepos=declared('byte_pos',lib,b'pub struct BytePos(pub u32)',True)
 spandata=declared('span_data',lib,b'pub struct SpanData',True)
 spanimpl=item('span_data_impl',lib,b'impl SpanData {')
 getters=[item('span_'+name,lib,needle)for name,needle in [('lo',b'pub fn lo(self)'),('hi',b'pub fn hi(self)'),('lo_hi',b'pub fn lo_hi(self)')]]
 debug=item('span_debug',lib,b'impl fmt::Debug for Span {');datadebug=item('span_data_debug',lib,b'impl fmt::Debug for SpanData {')
 write(destination/'foundations.rs',bytepos+b'\n'+spandata+b'\n'+spanimpl+b'\nimpl Span {\n'+b'\n'.join(getters)+b'\n}\n'+debug+b'\n'+datadebug+b'\n')
 raw=declared('raw_span',stable,b'pub struct RawSpan(pub u64)')
 globalsfn=item('with_session_globals',lib,b'pub fn with_session_globals<')
 track=declared('span_track',lib,b'pub static SPAN_TRACK:')
 tls=declared('session_tls',lib,b'scoped_tls::scoped_thread_local!(static SESSION_GLOBALS:')
 runtime=(HERE/'runtime.rs').read_bytes().replace(b'// EXACT_RAW_SPAN',raw).replace(b'// EXACT_GLOBALS_FN',globalsfn).replace(b'// EXACT_SPAN_TRACK',track).replace(b'// EXACT_TLS',tls)
 mode=item('sync_mode','compiler/rustc_data_structures/src/sync.rs',b'mod mode {')
 dynsend=item('dyn_send','compiler/rustc_data_structures/src/marker.rs',b'pub unsafe auto trait DynSend')
 dynsync=item('dyn_sync','compiler/rustc_data_structures/src/marker.rs',b'pub unsafe auto trait DynSync')
 write(destination/'sync_runtime.rs',dynsend+b'\n'+dynsync+b'\n'+mode+b'\n#[path="sync_lock.rs"]mod lock;pub use lock::Lock;\n')
 write(destination/'reference_runtime.rs',runtime)
 for name,path in [('span_encoding_original.rs','compiler/rustc_span/src/span_encoding.rs'),('idx.rs','compiler/rustc_index/src/idx.rs'),('fx.rs','compiler/rustc_data_structures/src/fx.rs'),('atomic_ref.rs','compiler/rustc_data_structures/src/atomic_ref.rs'),('int_overflow.rs','compiler/rustc_serialize/src/int_overflow.rs'),('sync_lock.rs','compiler/rustc_data_structures/src/sync/lock.rs')]:write(destination/name,original[path])
 for name in ['lib.rs','newtype.rs']:write(destination/'upstream/rustc_index_macros/src'/name,original['compiler/rustc_index_macros/src/'+name])
 for name in ['Cargo.toml','Cargo.lock','oracle.rs','adapters.rs']:write(destination/name,(HERE/name).read_bytes())
 write(destination/'upstream/rustc_index_macros/Cargo.toml',(HERE/'upstream/rustc_index_macros/Cargo.toml').read_bytes())
 result={'revision':PIN,'source_version':original['src/version'].decode().strip(),'sources':provenance,'exact_fragments':fragments}
 write(destination/'source-provenance.json',(json.dumps(result,indent=2)+'\n').encode())
 return result,original['Cargo.lock']

def verify_lock(lock,original):
 def packages(data):return {(p['name'],p['version']):p['checksum']for p in tomllib.loads(data.decode())['package']if p.get('source','').startswith('registry+')}
 known=packages(original)
 for key,value in packages(lock).items():
  if known.get(key)!=value:raise RuntimeError(f'dependency differs from original lock:{key}')

def verify_packages(metadata,destination):
 allowed={'ferrugo-span-encoding-oracle':destination/'Cargo.toml','rustc_index_macros':destination/'upstream/rustc_index_macros/Cargo.toml'}
 for package in metadata['packages']:
  source=package.get('source')
  if source is None:
   if package['name']not in allowed or pathlib.Path(package['manifest_path']).resolve()!=allowed[package['name']].resolve():raise RuntimeError(f'unexpected local dependency replacing original code:{package["name"]}')
  elif source!='registry+https://github.com/rust-lang/crates.io-index':raise RuntimeError(f'unexpected dependency source:{source}')

# Shared independent verifier is retained locally, not imported from mutable
# installed packages. It checks exact archives and actual expanded source trees.
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
 parser=argparse.ArgumentParser();parser.add_argument('--destination',type=pathlib.Path,default=ROOT/'.toolchain/oracles/span-encoding-reference');parser.add_argument('--offline',action='store_true');parser.add_argument('--write-lock',action='store_true');args=parser.parse_args()
 env=environment();destination=args.destination.resolve();result,original=prepare(destination,env)
 cargo=selected('FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo')
 if execute([env['RUSTC'],'--version'],env).decode().strip()!=RUST:raise RuntimeError('reference compiler differs')
 command=[cargo,'metadata','--offline','--format-version=1']
 if not args.write_lock:command.append('--locked')
 metadata=json.loads(execute(command,env,destination));verify_packages(metadata,destination);lock=(destination/'Cargo.lock').read_bytes();verify_lock(lock,original)
 registry=verify_registry(metadata,lock,pathlib.Path(env['CARGO_HOME']))
 if args.write_lock:write(HERE/'Cargo.lock',lock)
 print(f'Prepared {len(result["sources"])} exact files/{len(result["exact_fragments"])} exact scoped fragments; {len(registry)} verified registry packages.')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
