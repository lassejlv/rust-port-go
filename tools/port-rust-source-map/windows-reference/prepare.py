#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Workspace-local, authenticated Windows-target comparison prerequisites.

This does not validate a Windows kernel. Both native Rust and Go PE binaries
execute under the same explicitly recorded Wine emulation on Linux. No package
manager/system mutation, canonical sysroot mutation or source-body edit occurs.
"""
import argparse,hashlib,importlib.util,json,lzma,os,shutil,subprocess,tarfile,tempfile,urllib.request
from pathlib import Path
ROOT=Path(__file__).absolute().parents[3];HERE=Path(__file__).absolute().parent
CACHE=ROOT/'.toolchain/port-windows-proof';DOWNLOADS=CACHE/'downloads';INSTALL=CACHE/'root';SYSROOT=CACHE/'rust-sysroot'
TARGET='x86_64-pc-windows-gnu';TARGET_ARCHIVE='rust-std-nightly-'+TARGET+'.tar.xz';TARGET_SHA='191201d8abe237aded8a499a5fbd129336e569629b6391704151b12e68063f25'
def module(name,path):
 s=importlib.util.spec_from_file_location(name,path);v=importlib.util.module_from_spec(s);s.loader.exec_module(v);return v
PROFILE=module('ferrugo_windows_compiler_profile',ROOT/'tools/port-rust-symbol/native_profile.py')
def run(args,**kw):return subprocess.run([str(a) for a in args],check=True,timeout=kw.pop('timeout',60),**kw)
def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def fetch(path,url,sha,size,allowed):
 if not path.exists():
  if not allowed:raise RuntimeError('missing prerequisite; pass --fetch: '+str(path))
  with urllib.request.urlopen(url,timeout=60) as f:temp=path.with_suffix(path.suffix+'.download');temp.write_bytes(f.read())
  temp.replace(path)
 if digest(path)!=sha or size is not None and path.stat().st_size!=size:raise RuntimeError('authenticated artifact mismatch: '+str(path))
def verify_extracted_archive(archive):
 # Authenticated archives alone do not make mutable extraction caches trusted.
 # Compare every regular member before executing any runtime/library image.
 process=subprocess.Popen(['dpkg-deb','--fsys-tarfile',str(archive)],stdout=subprocess.PIPE)
 try:
  with tarfile.open(fileobj=process.stdout,mode='r|') as tar:
   for item in tar:
    relative=Path(item.name)
    if relative.is_absolute() or '..' in relative.parts:raise RuntimeError('unsafe authenticated archive member')
    actual=INSTALL/relative
    if item.isfile():
     if str(relative)=='usr/lib/wine/wineserver' and actual.with_suffix('.debian-original').is_file():actual=actual.with_suffix('.debian-original')
     expected=hashlib.sha256(tar.extractfile(item).read()).hexdigest()
     if not actual.is_file() or actual.stat().st_size!=item.size or digest(actual)!=expected:raise RuntimeError('extracted runtime artifact differs: '+str(actual))
  if process.wait(timeout=60):raise RuntimeError('package archive inspection failed')
 finally:
  if process.poll() is None:process.kill();process.wait()

def verify_packages(allowed):
 DOWNLOADS.mkdir(parents=True,exist_ok=True)
 signed=HERE/'trixie-InRelease';verified=DOWNLOADS/'trixie-Release-verified'
 with tempfile.TemporaryDirectory(prefix='ferrugo-windows-signature-') as tmp:
  authenticated=Path(tmp)/'Release'
  run(['sqv','--keyring','/usr/share/keyrings/debian-archive-keyring.gpg','--output',authenticated,'--cleartext',signed],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  verified.write_bytes(authenticated.read_bytes())
 release=verified.read_text();sha,size=next((line.split()[:2] for line in release.splitlines() if line.endswith(' main/binary-amd64/Packages.xz') and len(line.split()[0])==64))
 index=DOWNLOADS/'trixie-Packages.xz';fetch(index,'https://deb.debian.org/debian/dists/trixie/main/binary-amd64/by-hash/SHA256/'+sha,sha,int(size),allowed)
 selected=json.loads((HERE/'packages.json').read_text());blocks={}
 for block in lzma.decompress(index.read_bytes()).decode().split('\n\n'):
  if block.startswith('Package: '):
   name=block.splitlines()[0][9:]
   if name in selected:blocks[name]=dict(line.split(': ',1) for line in block.splitlines() if ': ' in line and not line.startswith(' '))
 for name,data in selected.items():
  if blocks.get(name)!=data:raise RuntimeError('selected package differs from signed complete index: '+name)
  archive=DOWNLOADS/Path(data['Filename']).name;fetch(archive,'https://deb.debian.org/debian/'+data['Filename'],data['SHA256'],int(data['Size']),allowed)
  # Extraction is strictly into the workspace. The original package bytes and
  # Debian copyright/license records remain available under this root.
  marker=INSTALL/('.verified-'+name)
  if not marker.exists() or marker.read_text()!=data['SHA256']:
   INSTALL.mkdir(parents=True,exist_ok=True);run(['dpkg-deb','-x',archive,INSTALL]);marker.write_text(data['SHA256'])
  verify_extracted_archive(archive)
 return {'inrelease_sha256':digest(signed),'package_index_sha256':sha,'release':release.split('MD5Sum:')[0],'packages':{name:{'version':data['Version'],'sha256':data['SHA256']} for name,data in selected.items()}}
def target(allowed):
 archive=DOWNLOADS/TARGET_ARCHIVE;fetch(archive,'https://static.rust-lang.org/dist/2026-11-04/'+TARGET_ARCHIVE,TARGET_SHA,None,allowed)
 inventory=json.loads((HERE/'rust-target-inventory.json').read_text());prefix='rust-std-nightly-'+TARGET+'/rust-std-'+TARGET+'/'
 expected={row['path']:row for row in inventory}
 with tarfile.open(archive,'r:xz') as tar:
  for item in tar:
   if item.isfile() and item.name.startswith(prefix):
    relative=item.name[len(prefix):]
    if relative not in expected:continue
    dest=SYSROOT/relative;dest.parent.mkdir(parents=True,exist_ok=True)
    if not dest.exists() or digest(dest)!=expected[relative]['sha256']:dest.write_bytes(tar.extractfile(item).read())
   elif item.isfile() and item.name.rsplit('/',1)[-1] in ['COPYRIGHT','LICENSE-MIT','LICENSE-APACHE']:
    dest=CACHE/'rust-licenses'/item.name.rsplit('/',1)[-1];dest.parent.mkdir(exist_ok=True);dest.write_bytes(tar.extractfile(item).read())
 for relative,row in expected.items():
  dest=SYSROOT/relative
  if not dest.exists() or digest(dest)!=row['sha256'] or dest.stat().st_size!=row['bytes']:raise RuntimeError('Rust target component differs: '+relative)
 return {'archive_sha256':TARGET_SHA,'inventory_sha256':digest(HERE/'rust-target-inventory.json'),'files':len(inventory)}
def environment():
 # Debian's dispatcher script has absolute installed paths. Keep its original
 # bytes and use the exact unmodified wineserver64 ELF via a local symlink.
 server=INSTALL/'usr/lib/wine/wineserver';original=server.with_suffix('.debian-original')
 if not original.exists() and not server.is_symlink():server.rename(original)
 if server.exists() or server.is_symlink():server.unlink()
 server.symlink_to('wineserver64')
 nls=INSTALL/'usr/share/wine/wine/nls';nls.parent.mkdir(exist_ok=True)
 if not nls.exists():nls.symlink_to('../nls')
 prefix=CACHE/'prefix';system=prefix/'drive_c/windows/system32';system.mkdir(parents=True,exist_ok=True)
 windows=INSTALL/'usr/lib/x86_64-linux-gnu/wine/x86_64-windows';unix=INSTALL/'usr/lib/x86_64-linux-gnu/wine/x86_64-unix'
 # Wine's normal installer populates this prefix with its own builtin images.
 # Local symlinks keep all DLL bytes pinned to the authenticated package.
 for src in windows.iterdir():
  dst=system/src.name
  if src.is_file():
   if dst.is_symlink() and dst.resolve()==src.resolve():continue
   if dst.exists() or dst.is_symlink():dst.unlink()
   dst.symlink_to(src)
 for src in [INSTALL/'usr/x86_64-w64-mingw32/lib/zlib1.dll',INSTALL/'usr/x86_64-w64-mingw32/lib/libwinpthread-1.dll',INSTALL/'usr/lib/gcc/x86_64-w64-mingw32/14-posix/libgcc_s_seh-1.dll']:
  dst=system/src.name
  if dst.exists() or dst.is_symlink():dst.unlink()
  dst.symlink_to(src)
 env=os.environ.copy();env.update(WINEPREFIX=str(prefix),WINEDEBUG='-all',WINEARCH='win64',WINESERVER=str(INSTALL/'usr/lib/wine/wineserver64'),WINELOADER=str(INSTALL/'usr/lib/wine/wine64'),WINEDLLPATH=str(windows.parent),WINEDATADIR=str(INSTALL/'usr/share/wine'),LD_LIBRARY_PATH=str(unix)+':'+str(INSTALL/'usr/lib/x86_64-linux-gnu'),XDG_CACHE_HOME=str(CACHE/'xdg-cache'))
 return env
def prepare(allowed=False,rustc=None):
 env=os.environ.copy()
 if rustc is not None:env['FERRUGO_RUSTC']=rustc
 compiler,env,identity=PROFILE.compiler_profile(env)
 packages=verify_packages(allowed);target_metadata=target(allowed);env.update(environment())
 lld=Path(compiler).parent.parent/'lib/rustlib/x86_64-unknown-linux-gnu/bin/rust-lld'
 if not lld.is_file():raise RuntimeError('matching official Rust LLD missing: '+str(lld))
 mingw=INSTALL/'usr/x86_64-w64-mingw32/lib';gcc=INSTALL/'usr/lib/gcc/x86_64-w64-mingw32/14-posix'
 args=[compiler,'--target',TARGET,'--sysroot',str(SYSROOT),'-C','linker='+str(lld),'-C','linker-flavor=ld.lld','-L','native='+str(mingw),'-L','native='+str(gcc)]
 for item in [mingw/'crt2.o',gcc/'crtbegin.o',gcc/'crtend.o']:args+=['-C','link-arg='+str(item)]
 meta={'schema':1,'scope':'both Rust and Go Windows-target PE binaries under Wine on Linux; no native Windows kernel/runtime claim','compiler':identity,'target':target_metadata,'debian':packages,'wine_version':run([env['WINELOADER'],'--version'],env=env,stdout=subprocess.PIPE).stdout.decode().strip(),'runtime_adaptations':['workspace wineserver dispatcher symlink to unmodified wineserver64','workspace nls layout symlink','workspace prefix system32 symlinks to unmodified authenticated builtin/runtime DLLs'],'host_libc':run(['ldd','--version'],stdout=subprocess.PIPE).stdout.decode().splitlines()[0]}
 (CACHE/'profile.json').write_text(json.dumps(meta,indent=2)+'\n');return args,env,meta
def main():
 p=argparse.ArgumentParser();p.add_argument('--fetch',action='store_true');p.add_argument('--rustc');args=p.parse_args();command,env,meta=prepare(args.fetch,args.rustc);print(json.dumps(meta,indent=2))
if __name__=='__main__':main()
