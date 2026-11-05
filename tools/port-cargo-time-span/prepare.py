#!/usr/bin/env python3
"""Checksum-verified unchanged Cargo73 + exact locked anyhow; no source edits."""
import argparse,hashlib,io,json,os,pathlib,shutil,subprocess,tarfile,tomllib,urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[2]
HERE=pathlib.Path(__file__).resolve().parent
PIN='24de4721844c7a4d839072bab6fb66d41c9bc1f4'
SOURCE='src/util/time_span.rs'
SOURCE_SHA='185b6ee6c972b98a19ca7577482777c9e706b71869e6a6d3008d5a9a1c36f130'
ANYHOW_SHA='2a4385e2e34eb35d6b3efe798b9eb88096925d87726c0798709bf56d9ed84af3'
CACHE=ROOT/'.toolchain/oracles/cargo-time-span'
def sha(data):return hashlib.sha256(data).hexdigest()
def tool(name,fallback):
 value=os.environ.get(name,fallback)
 if not value:raise RuntimeError(name+' explicitly empty')
 path=shutil.which(value)if'/'not in value else str(pathlib.Path(value).absolute())
 if not path or not os.access(path,os.X_OK):raise RuntimeError('unavailable executable:'+value)
 return path
def owned_cache_path(path):
 try:path.relative_to(CACHE)
 except ValueError:raise RuntimeError('path outside owned time-span cache')
 current=path
 while current!=CACHE.parent:
  if current.is_symlink():raise RuntimeError('symlink in owned time-span cache:'+str(current))
  current=current.parent
 return path

def verify_string_sdk(fetch):
 meta=json.loads((HERE/'sdk199.json').read_text())
 if meta['compiler_commit']!='58cf8f9c35b355199727627d07ae34a4e2846d67' or meta['archive']['xz_hash']!='e485a88c45db546b73571df7bebfcb4454ba4fb552c492e25fc5b00eeb5bf22d':raise RuntimeError('String SDK source selection differs')
 archive=owned_cache_path(CACHE/'downloads/rust-src-1.99.1.tar.xz')
 if archive.is_symlink():raise RuntimeError('symlink SDK archive')
 if not archive.exists():
  if not fetch:raise RuntimeError('missing exact String SDK source; prepare with --fetch')
  with urllib.request.urlopen(meta['archive']['xz_url'],timeout=60)as response:raw=response.read()
  if sha(raw)!=meta['archive']['xz_hash']:raise RuntimeError('SDK source archive checksum differs')
  archive.write_bytes(raw)
 if sha(archive.read_bytes())!=meta['archive']['xz_hash']:raise RuntimeError('cached SDK source archive differs')
 expected={p['path']:p for p in meta['files']};found=set()
 with tarfile.open(archive,'r:xz')as source:
  for member in source:
   name=next((p for p in expected if member.name.endswith('/'+p)),None)
   if name is None or not member.isfile():continue
   data=source.extractfile(member).read()
   if sha(data)!=expected[name]['sha256']:raise RuntimeError('SDK used source differs:'+name)
   target=CACHE/'sdk199'/name
   parent=target
   while parent!=CACHE:
    if parent.is_symlink():raise RuntimeError('SDK source symlink:'+str(parent))
    parent=parent.parent
   if target.exists()and target.read_bytes()!=data:raise RuntimeError('SDK source extraction changed:'+name)
   target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data);found.add(name)
 if found!=set(expected):raise RuntimeError('incomplete SDK source archive')
 return meta

def prepare(fetch=False):
 rustc=tool('FERRUGO_RUSTC',str(ROOT/'.toolchain/rust/bin/rustc')if(ROOT/'.toolchain/rust/bin/rustc').exists()else'rustc')
 cargo=tool('FERRUGO_CARGO',str(ROOT/'.toolchain/rust/bin/cargo')if(ROOT/'.toolchain/rust/bin/cargo').exists()else'cargo')
 version=subprocess.check_output([rustc,'--version'],text=True,timeout=10).strip()
 if version!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('time-span reference requires exact SDK1.99.1/Unicode17:'+version)
 source_value=os.environ.get('FERRUGO_CARGO_SOURCE',str(ROOT/'.toolchain/upstream-cargo'))
 if not source_value:raise RuntimeError('FERRUGO_CARGO_SOURCE explicitly empty')
 source=pathlib.Path(source_value).absolute()
 env=os.environ.copy();env['GIT_NO_LAZY_FETCH']='1'
 data=subprocess.check_output(['git','-C',str(source),'show',PIN+':'+SOURCE],env=env,timeout=30)
 if sha(data)!=SOURCE_SHA:raise RuntimeError('Cargo source differs')
 lock=subprocess.check_output(['git','-C',str(source),'show',PIN+':Cargo.lock'],env=env,timeout=30)
 packages=[p for p in tomllib.loads(lock.decode())['package']if p['name']=='anyhow']
 if len(packages)!=1 or packages[0]['version']!='1.0.103' or packages[0]['checksum']!=ANYHOW_SHA:raise RuntimeError('original Cargo locked anyhow selection differs')
 alias_source=subprocess.check_output(['git','-C',str(source),'show',PIN+':src/util/errors.rs'],env=env,timeout=30)
 alias=b'pub type CargoResult<T> = anyhow::Result<T>;'
 if alias not in alias_source:raise RuntimeError('actual CargoResult alias differs')
 # Validate any pre-existing owner BEFORE creating even one child directory.
 # A foreign marker is read-only evidence; preparation must not rewrite it.
 owned_cache_path(CACHE)
 marker=owned_cache_path(CACHE/'ownership.json');identity={'tool':'port-cargo-time-span','revision':PIN}
 if CACHE.exists():
  if not CACHE.is_dir():raise RuntimeError('refusing non-directory time-span cache')
  if marker.exists():
   if not marker.is_file():raise RuntimeError('invalid time-span ownership marker')
   try:actual_owner=json.loads(marker.read_text())
   except (ValueError,UnicodeError):raise RuntimeError('invalid time-span ownership marker')
   if actual_owner!=identity:raise RuntimeError('foreign time-span cache')
  elif any(CACHE.iterdir()):raise RuntimeError('refusing unowned time-span cache')
 CACHE.mkdir(parents=True,exist_ok=True)
 if not marker.exists():
  with marker.open('x')as output:output.write(json.dumps(identity,sort_keys=True)+'\n')
 for path in [CACHE/'vendor',CACHE/'original',CACHE/'downloads']:
  owned_cache_path(path);path.mkdir(parents=True,exist_ok=True)
 original=owned_cache_path(CACHE/'original'/SOURCE);original.parent.mkdir(parents=True,exist_ok=True)
 if original.is_symlink()or original.exists()and original.read_bytes()!=data:raise RuntimeError('original source copy changed')
 original.write_bytes(data)
 archive=owned_cache_path(CACHE/'downloads/anyhow-1.0.103.crate')
 if not archive.exists():
  if not fetch:raise RuntimeError('missing exact anyhow archive; prepare with --fetch')
  with urllib.request.urlopen('https://static.crates.io/crates/anyhow/anyhow-1.0.103.crate',timeout=60)as response:body=response.read()
  if sha(body)!=ANYHOW_SHA:raise RuntimeError('anyhow archive checksum differs')
  archive.write_bytes(body)
 if archive.is_symlink():raise RuntimeError('refusing symlink anyhow archive')
 body=archive.read_bytes()
 if sha(body)!=ANYHOW_SHA:raise RuntimeError('cached anyhow archive checksum differs')
 files={}
 with tarfile.open(fileobj=io.BytesIO(body),mode='r:gz')as crate:
  for member in crate:
   relative=pathlib.PurePosixPath(member.name)
   if relative.is_absolute()or'..'in relative.parts or not relative.parts or relative.parts[0]!='anyhow-1.0.103':raise RuntimeError('invalid crate member')
   if not member.isfile():
    if not member.isdir():raise RuntimeError('nonregular crate member')
    continue
   raw=crate.extractfile(member).read();dest=CACHE/'vendor'/member.name
   if dest.is_symlink()or dest.exists()and dest.read_bytes()!=raw:raise RuntimeError('extracted anyhow source differs:'+member.name)
   parent=dest.parent
   while parent!=CACHE:
    if parent.is_symlink():raise RuntimeError('symlink extracted parent:'+str(parent))
    parent=parent.parent
   dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(raw);files[member.name]={'bytes':len(raw),'sha256':sha(raw)}
 sdk=verify_string_sdk(fetch)
 return {'cargo_pin':PIN,'source_sha256':SOURCE_SHA,'cargo_lock_sha256':sha(lock),'cargo_result_alias_sha256':sha(alias),'anyhow_version':'1.0.103','anyhow_archive_sha256':ANYHOW_SHA,'anyhow_files':files,'rustc_version':version,'owned_string_sdk':sdk},dict(env,RUSTC=rustc,FERRUGO_TIME_SPAN_SOURCE=str(original),RUST_BACKTRACE='0',RUST_LIB_BACKTRACE='0'),cargo
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--fetch',action='store_true');a=p.parse_args();profile,_,_=prepare(a.fetch);print(json.dumps(profile,indent=2,sort_keys=True))
