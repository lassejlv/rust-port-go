#!/usr/bin/env python3
"""Verify the exact official SDK rust-src component used by ruststr."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tomllib
import urllib.request

ROOT=Path(__file__).resolve().parents[2]
HERE=Path(__file__).resolve().parent
COMMIT='708c94ef093ecc42fb8851e17d8383d999727abc'
MANIFEST='3740867978321544cb6ab736d104730a951d0b6aeda69866118765ec073f823e'
ARCHIVE='d99600cbf56d2db40112095e99e138f9e2f30002ef2774d288dc525e28a1357c'
OWNER={'schema':1,'tool':'port-rust-str','commit':COMMIT}
def sha(data):return hashlib.sha256(data).hexdigest()
def selected(value=None):
 if value is None:value=os.environ.get('FERRUGO_RUSTC',str(ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc'))
 if not value:raise RuntimeError('FERRUGO_RUSTC explicitly empty')
 if '/'in value or '\\'in value:value=os.path.abspath(value if os.path.isabs(value)else ROOT/value)
 executable=shutil.which(value)
 if executable is None:raise RuntimeError('selected SDK compiler is unavailable')
 result=subprocess.run([executable,'-vV'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True,timeout=20)
 version=result.stdout.decode()
 if 'commit-hash: '+COMMIT not in version or 'release: 1.101.0-nightly'not in version:raise RuntimeError('selected compiler differs from the official SDK profile')
 return executable,version
def fetch(url,limit):
 with urllib.request.urlopen(url,timeout=45)as stream:data=stream.read(limit+1)
 if len(data)>limit:raise RuntimeError('SDK download exceeds bound')
 return data
def verified(cache=None,download=False):
 cache=Path(cache)if cache is not None else ROOT/'.toolchain/ruststr-sdk-1.101'
 if not cache.is_absolute():cache=ROOT/cache
 if cache.is_symlink():raise RuntimeError('symlink SDK cache')
 # Isolated checkouts may intentionally alias their trusted .toolchain base.
 # Resolve only the selected cache's parent; its own leaf and every owned
 # descendant remain subject to explicit symlink/tamper rejection.
 cache=cache.parent.resolve()/cache.name
 marker=cache/'ownership.json'
 if marker.is_symlink():raise RuntimeError('symlink SDK owner marker')
 if cache.exists()and any(cache.iterdir())and not marker.is_file():raise RuntimeError('refusing unowned SDK cache')
 if marker.exists()and json.loads(marker.read_text())!=OWNER:raise RuntimeError('SDK cache owner differs')
 cache.mkdir(parents=True,exist_ok=True)
 manifest=cache/'channel-rust-nightly.toml';archive=cache/'rust-src-nightly.tar.xz'
 if not manifest.exists():
  if not download:raise RuntimeError('SDK source missing; run prepare.py --fetch')
  data=fetch('https://static.rust-lang.org/dist/2026-11-04/channel-rust-nightly.toml',2<<20)
  if sha(data)!=MANIFEST:raise RuntimeError('official channel manifest checksum differs')
  manifest.write_bytes(data)
 if manifest.is_symlink()or manifest.stat().st_size>2<<20 or sha(manifest.read_bytes())!=MANIFEST:raise RuntimeError('official channel manifest checksum differs')
 package=tomllib.loads(manifest.read_text())['pkg']['rust-src'];target=package['target']['*']
 if package['version']!='1.101.0-nightly (708c94ef0 2026-11-03)'or target['xz_hash']!=ARCHIVE:raise RuntimeError('SDK source selection differs')
 if not archive.exists():
  if not download:raise RuntimeError('SDK source archive missing; run prepare.py --fetch')
  data=fetch(target['xz_url'],20<<20)
  if sha(data)!=ARCHIVE:raise RuntimeError('SDK source archive checksum differs')
  archive.write_bytes(data)
 if archive.is_symlink()or archive.stat().st_size>20<<20 or sha(archive.read_bytes())!=ARCHIVE:raise RuntimeError('SDK source archive checksum differs')
 expected=json.loads((HERE/'SOURCES.json').read_text())
 if expected['compiler_commit']!=COMMIT or expected['sha256']!=ARCHIVE or expected['manifest_sha256']!=MANIFEST:raise RuntimeError('retained source selection differs')
 with tarfile.open(archive,'r:xz')as tar:
  commit=tar.extractfile('rust-src-nightly/git-commit-hash').read().decode()
  if commit!=COMMIT:raise RuntimeError('SDK source commit differs')
  for name,proof in expected['sources'].items():
   member=tar.getmember(proof['member'])
   if not member.isfile()or member.size>2<<20:raise RuntimeError('invalid selected SDK source member')
   data=tar.extractfile(member).read()
   if len(data)!=proof['bytes']or sha(data)!=proof['sha256']:raise RuntimeError('selected SDK source bytes differ: '+name)
   output=cache/'original'/name
   for parent in output.parents:
    if parent==cache:break
    if parent.is_symlink():raise RuntimeError('symlink SDK source parent')
   if output.is_symlink()or output.exists()and output.read_bytes()!=data:raise RuntimeError('copied SDK source differs: '+name)
   if not output.exists():output.parent.mkdir(parents=True,exist_ok=True);output.write_bytes(data)
  for name,digest in expected['licenses'].items():
   data=tar.extractfile('rust-src-nightly/'+name).read()
   if sha(data)!=digest or (ROOT/'port/support/ruststr'/name).read_bytes()!=data:raise RuntimeError('SDK source license differs: '+name)
 marker.write_text(json.dumps(OWNER,indent=2)+'\n')
 return expected,cache
def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--cache',type=Path);parser.add_argument('--fetch',action='store_true');args=parser.parse_args();proof,_=verified(args.cache,args.fetch);print('Verified exact SDK rust-src component and',len(proof['sources']),'scoped original modules')
if __name__=='__main__':
 try:main()
 except(Exception)as error:raise SystemExit(str(error))
