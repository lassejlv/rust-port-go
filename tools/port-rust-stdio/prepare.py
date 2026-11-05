#!/usr/bin/env python3
"""Verify exact selected SDK sources and compiler identities for scoped stdio."""
import hashlib,json,os,shutil,subprocess,tarfile,urllib.request
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2]
HERE=Path(__file__).absolute().parent
MAX_ARCHIVE=16*1024*1024
PROFILE={
 '1.99':{'commit':'58cf8f9c35b355199727627d07ae34a4e2846d67','prefix':'rust-src-1.99.1','url':'https://static.rust-lang.org/dist/2026-10-15/rust-src-1.99.1.tar.xz','sha256':'e485a88c45db546b73571df7bebfcb4454ba4fb552c492e25fc5b00eeb5bf22d','default':'.toolchain/rust/bin/rustc'},
 '1.101':{'commit':'708c94ef093ecc42fb8851e17d8383d999727abc','prefix':'rust-src-nightly','url':'https://static.rust-lang.org/dist/2026-11-04/rust-src-nightly.tar.xz','sha256':'d99600cbf56d2db40112095e99e138f9e2f30002ef2774d288dc525e28a1357c','default':'.toolchain/rust-nightly-2026-11-04/bin/rustc'},
}
def sha(data):return hashlib.sha256(data).hexdigest()
def selected(sdk,override=None):
 envkey='FERRUGO_STDIO_RUSTC_'+sdk.replace('.','')
 value=override if override is not None else os.environ.get(envkey,str(ROOT/PROFILE[sdk]['default']))
 if not value:raise RuntimeError(f'{envkey} is explicitly empty')
 found=shutil.which(value)
 if not found:raise RuntimeError(f'selected {sdk} compiler missing: {value}')
 # rustup is a multiplexer: resolving its rustc symlink changes argv[0].
 rustc=str(Path(found).absolute());version=subprocess.check_output([rustc,'-Vv'],text=True,timeout=20)
 if f"commit-hash: {PROFILE[sdk]['commit']}\n"not in version:raise RuntimeError(f'selected {sdk} compiler identity differs')
 return rustc,version
def verified(cache=None,offline=False):
 requested=Path(cache)if cache is not None else ROOT/'.toolchain/stdio-sdk'
 # A checkout may share its trusted .toolchain BASE through a symlink. Only
 # canonicalize the parent; an owned cache leaf/descendant symlink is tampering.
 requested.parent.mkdir(parents=True,exist_ok=True)
 cache=requested.parent.resolve()/requested.name
 if cache.is_symlink():raise RuntimeError('stdio cache root is a symlink')
 cache.mkdir(exist_ok=True)
 for folder,dirs,files in os.walk(cache,followlinks=False):
  for name in dirs+files:
   if (Path(folder)/name).is_symlink():raise RuntimeError('stdio cache descendant is a symlink')
 marker=cache/'ownership.json';body={'owner':'tools/port-rust-stdio','schema':1}
 if marker.exists():
  if json.loads(marker.read_text())!=body:raise RuntimeError('stdio cache ownership differs')
 elif any(cache.iterdir()):raise RuntimeError('refusing unowned nonempty stdio cache')
 else:marker.write_text(json.dumps(body,sort_keys=True)+'\n')
 sources={}
 for sdk,profile in PROFILE.items():
  archive=cache/('sdk-'+sdk+'.tar.xz')
  if not archive.exists():
   if offline:raise RuntimeError('missing offline stdio SDK source archive')
   with urllib.request.urlopen(profile['url'],timeout=60)as response:raw=response.read(MAX_ARCHIVE+1)
   if len(raw)>MAX_ARCHIVE or sha(raw)!=profile['sha256']:raise RuntimeError('downloaded SDK source archive differs')
   archive.write_bytes(raw)
  if not archive.is_file()or archive.stat().st_size>MAX_ARCHIVE:raise RuntimeError('invalid SDK source archive')
  if sha(archive.read_bytes())!=profile['sha256']:raise RuntimeError('SDK source archive checksum differs')
  files={}
  with tarfile.open(archive)as stream:
   for name in ['std/src/io/stdio.rs','core/src/io/write.rs','std/src/sync/reentrant_lock.rs','std/src/sync/poison.rs','std/src/sync/poison/mutex.rs','alloc/src/io/impls.rs','alloc/src/raw_vec/mod.rs','alloc/src/vec/mod.rs','alloc/src/vec/spec_extend.rs','alloc/src/sync.rs']:
    member=stream.getmember(profile['prefix']+'/rust-src/lib/rustlib/src/rust/library/'+name)
    if not member.isfile()or member.size>1024*1024:raise RuntimeError('invalid SDK source member')
    files['library/'+name]=sha(stream.extractfile(member).read())
   reentrant=stream.extractfile(profile['prefix']+'/rust-src/lib/rustlib/src/rust/library/std/src/sync/reentrant_lock.rs').read().decode()
   if 'lock_count: UnsafeCell<u32>'not in reentrant or '(*self.lock_count.get()).checked_add(1)?'not in reentrant or '.expect("lock count overflow in reentrant mutex")'not in reentrant:raise RuntimeError('selected reentrant source branch differs')
  sources[sdk]={'archive_url':profile['url'],'archive_sha256':profile['sha256'],'files_sha256':files}
 return sources
