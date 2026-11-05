#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Actual Windows std io::Error vs full Go span backend; both run under Wine."""
import argparse,gzip,hashlib,json,os,random,subprocess,sys
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(HERE/'windows-reference'));import prepare
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_io_windows_wine.tsv.gz'
def signed(n):return n if n<(1<<31) else n-(1<<32)
def requests():
 rng=random.Random(0x2d20c2);codes=list(range(-32,20001))+[-(1<<31),(1<<31)-1]+[signed(0x10000000|n) for n in [0,1,2,3,5,87,123,317,0x80000005,0xc0000005,0xc0000135]]+[signed(rng.randrange(1<<32)) for _ in range(256)]
 ident=0
 for code in codes:
  for op in ['display','debug']:yield f'{ident}\t{code}\t{op}\t0\t0\n';ident+=1
 for code in [0,1,2,5,87,123,128,258,317,995,1117,1231,15403,10013,10022,10058,-1,-(1<<31),0x10000005,signed(0xd0000005)]:
  for style in range(9):
   for fail in [0,1,2,3,5,8,13,21]:yield f'{ident}\t{code}\tdebug\t{style}\t{fail}\n';ident+=1
  for fail in [1,2,3,4,5,6,7,8,9]:yield f'{ident}\t{code}\tdisplay\t0\t{fail}\n';ident+=1

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--fetch',action='store_true');p.add_argument('--rustc');p.add_argument('--go',default=str(ROOT/'.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go'));args=p.parse_args()
 command,env,profile=prepare.prepare(args.fetch,args.rustc);cache=prepare.CACHE;source=HERE/'io_windows.rs';native=cache/'io-native.exe'
 subprocess.run([*command,str(source),'-o',str(native)],cwd=ROOT,env=env,check=True,timeout=60)
 gobinary=cache/'span-go-test.exe';goenv=os.environ.copy();goenv.update(GOOS='windows',GOARCH='amd64',GOCACHE='/tmp/ferrugo-go123-cache')
 subprocess.run([args.go,'test','-c','-o',str(gobinary),'./port/rust/compiler/rustc_span/src'],cwd=ROOT,env=goenv,check=True,timeout=120)
 data=''.join(requests()).encode();req=data.splitlines();observed=[]
 for binary,extra in [(native,[]),(gobinary,['-test.run=^TestSourceMapWindowsIOErrorChild$'])]:
  child_env=env.copy();child_env['FERRUGO_WINDOWS_IO_CHILD']='1'
  result=subprocess.run([env['WINELOADER'],str(binary),*extra],input=data,stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=ROOT,env=child_env,check=True,timeout=60)
  observed.append(result.stdout.splitlines());(cache/(binary.stem+'-io-wine-stderr.txt')).write_bytes(result.stderr)
 if any(len(v)!=len(req) for v in observed):raise RuntimeError(f'missing Windows error records {len(req)}/'+str([len(v) for v in observed]))
 out=bytearray()
 for request,rust,go in zip(req,*observed):
  if rust!=go:raise RuntimeError(f'emulated Rust/Go error mismatch request={request!r} native={rust!r} go={go!r}')
  ident,rest=rust.split(b'\t',1);assert ident==request.split(b'\t')[0];out.extend(request+b'\t'+rest+b'\n')
 data=bytes(out)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('Windows error emulated drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(req),'profile':profile,'scope':'actual pinned std Windows io::Error vs actual whole Go span native Windows backend; both PE binaries under Wine on Linux; no native Windows kernel validation or std file credit','raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'native_wrapper_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'go_toolchain':subprocess.run([args.go,'version'],check=True,stdout=subprocess.PIPE).stdout.decode().strip()}
 path=DEST.with_name('source_map_io_windows_wine_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('Windows error emulated manifest drift')
 else:path.write_text(render)
 print(len(req),'Windows io::Error records: actual Rust and whole-span Go PE agree under Wine; not native Windows kernel validation')
if __name__=='__main__':main()
