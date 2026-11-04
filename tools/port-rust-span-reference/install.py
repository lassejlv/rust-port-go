#!/usr/bin/env python3
"""Install the checksum-pinned official reference toolchain in its own prefix."""
import argparse,hashlib,json,os,pathlib,subprocess,sys,tarfile,tempfile,urllib.request

ROOT=pathlib.Path(__file__).resolve().parents[2]
TOOL=ROOT/'tools/port-rust-span-reference'
def sha(data):return hashlib.sha256(data).hexdigest()
def command(args):
 result=subprocess.run([str(a)for a in args],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
 if result.returncode:raise RuntimeError(result.stderr.decode(errors='replace'))
 return result.stdout.decode().strip()
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--fetch',action='store_true');parser.add_argument('--prefix');parser.add_argument('--archives');args=parser.parse_args()
 selection=json.loads((TOOL/'toolchain.json').read_text())
 def path(value,default,name):
  if value=='':raise RuntimeError(name+' explicitly empty')
  p=pathlib.Path(value if value is not None else default);return(p if p.is_absolute()else ROOT/p).resolve()
 prefix=path(args.prefix,'.toolchain/rust-nightly-2026-11-04','prefix');archives=path(args.archives,'.toolchain/downloads/rust-nightly-2026-11-04','archives')
 old=ROOT/'.toolchain/rust'
 if prefix==old or prefix.is_relative_to(old)or old.is_relative_to(prefix):raise RuntimeError('reference installation must not overlap the prior1.99 toolchain')
 if prefix.exists()and any(prefix.iterdir()):
  rustc=prefix/'bin/rustc';cargo=prefix/'bin/cargo'
  if not rustc.exists()or not cargo.exists()or command([rustc,'--version'])!=selection['rustc']or command([cargo,'--version'])!=selection['cargo']:raise RuntimeError('refusing to overwrite an existing different/unowned toolchain prefix')
  libdir=pathlib.Path(command([rustc,'--print','target-libdir']))
  if not list(libdir.glob('libstd-*.rlib')):raise RuntimeError('reference compiler is present but its matching standard library is missing')
  print('Exact reference toolchain already installed:'+str(prefix));return
 archives.mkdir(parents=True,exist_ok=True)
 for component in selection['components']:
  archive=archives/(component['component']+'.tar.xz')
  if not archive.exists():
   if not args.fetch:raise RuntimeError('missing exact archive; prepare with --fetch:'+str(archive))
   with urllib.request.urlopen(component['url'],timeout=120)as response:data=response.read()
   if sha(data)!=component['sha256']:raise RuntimeError('official archive checksum differs:'+component['component'])
   archive.write_bytes(data)
  if sha(archive.read_bytes())!=component['sha256']:raise RuntimeError('cached official archive checksum differs:'+str(archive))
 with tempfile.TemporaryDirectory(prefix='ferrugo-span-nightly-')as temporary:
  unpack=pathlib.Path(temporary)
  for component in selection['components']:
   with tarfile.open(archives/(component['component']+'.tar.xz'),'r:xz')as archive:archive.extractall(unpack,filter='data')
   source=unpack/(component['component']+'-nightly-'+selection['target'])
   command(['sh',source/'install.sh','--prefix='+str(prefix),'--disable-ldconfig'])
 if command([prefix/'bin/rustc','--version'])!=selection['rustc']or command([prefix/'bin/cargo','--version'])!=selection['cargo']:raise RuntimeError('installed compiler/Cargo identity differs')
 (prefix/'ferrugo-selection.json').write_text(json.dumps(selection,indent=2)+'\n')
 print('Installed exact official reference toolchain:'+str(prefix))
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
