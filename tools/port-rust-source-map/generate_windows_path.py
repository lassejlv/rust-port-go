#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Windows std Path vs actual Go helpers; both PE binaries execute under Wine."""
import argparse,gzip,hashlib,importlib.util,json,os,random,subprocess,sys
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(HERE/'windows-reference'));import prepare,bridge
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_path_windows_wine.tsv.gz'
def wide(s):return s.encode('utf-16-le','surrogatepass')
def requests():
 rng=random.Random(0x2d20c1)
 paths=[wide(s) for s in ['', '.', './', '..', '..\\', '/', '\\', '//', '\\\\', 'C:', 'c:', 'C:\\','C:/','C:foo','c:foo/.','C:foo\\..\\bar','\\foo','/foo','\\\\host','\\\\host\\share','\\\\host\\share\\','\\\\host\\share\\a/./b','//host/share/a','\\\\?\\C:','\\\\?\\C:\\a/./b','\\\\?\\UNC\\server\\share\\a','\\\\?\\UNC\\server','\\\\.\\COM1','\\\\?\\volume\\a','\\??\\C:\\a','a\x00b','a\\.\\b//../c','\ud800','\udfff','\ud800\udfff','\ud800\ud800','\udc00\udc00','\U0001cfaf/\u200d/é']]
 paths += [n.to_bytes(2,'little') for n in range(256)]
 paths += [n.to_bytes(2,'little') for n in range(0xd800,0xe000)]
 pieces=[wide(s) for s in ['','a','b','.','..','/','\\','C:','\\\\?\\','\ud800','\udfff','β','\u200d']]
 paths += [wide('\\').join(rng.choice(pieces) for _ in range(rng.randrange(12))) for _ in range(512)]
 paths += [b''.join(rng.randrange(65536).to_bytes(2,'little') for _ in range(rng.randrange(0,30))) for _ in range(256)]
 ident=0
 for index,p in enumerate(paths):
  for op in ['view','hash','oshash','lossy','clone']:
   yield f'{ident}\t{op}\t{p.hex()}\t\t\t0\t0\n';ident+=1
  for script in ['FACFBACFBA','BCAFABCFA','FFFFFFFFFFFFFFFF','BBBBBBBBBBBBBBBB']:
   yield f'{ident}\titer\t{p.hex()}\t\t{script}\t0\t0\n';ident+=1
  for _ in range(3):
   q=rng.choice(paths)
   for op in ['compare','strip','join']:
    yield f'{ident}\t{op}\t{p.hex()}\t{q.hex()}\t\t0\t0\n';ident+=1
  styles=range(9) if index<256 or index>=256+2048 else [0,5]
  for style in styles:
   for fail in [0,1,3,10]:
    yield f'{ident}\tdebug\t{p.hex()}\t\t\t{style}\t{fail}\n';ident+=1

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--fetch',action='store_true');p.add_argument('--rustc');p.add_argument('--go',default=str(ROOT/'.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go'));args=p.parse_args()
 command,env,profile=prepare.prepare(args.fetch,args.rustc);cache=prepare.CACHE
 source=HERE/'path_windows.rs';native=cache/'path-native.exe';subprocess.run([*command,str(source),'-o',str(native)],cwd=ROOT,env=env,check=True,timeout=60)
 dest=cache/'bridge';dest.mkdir(exist_ok=True);selected=bridge.sources(dest);gobinary=cache/'path-go.exe'
 goenv=os.environ.copy();goenv.update(GOOS='windows',GOARCH='amd64',GOCACHE='/tmp/ferrugo-go123-cache')
 subprocess.run([args.go,'build','-o',str(gobinary),'./'+str(dest.relative_to(ROOT))],cwd=ROOT,env=goenv,check=True,timeout=120)
 data=''.join(requests()).encode();observations=[]
 for binary in [native,gobinary]:
  result=subprocess.run([env['WINELOADER'],str(binary)],input=data,stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=60)
  observations.append(result.stdout.splitlines());(cache/(binary.stem+'-wine-stderr.txt')).write_bytes(result.stderr)
 req=data.splitlines();native_rows,go_rows=observations
 if len(native_rows)!=len(req) or len(go_rows)!=len(req):raise RuntimeError(f'missing emulated records {len(req)}/{len(native_rows)}/{len(go_rows)}')
 out=bytearray()
 for row,rust,go in zip(req,native_rows,go_rows):
  if rust!=go:
   raise RuntimeError(f'emulated Rust/Go mismatch request={row!r} native={rust!r} go={go!r}')
  ident,observed=rust.split(b'\t');assert ident==row.split(b'\t')[0];out.extend(row+b'\t'+observed+b'\n')
 data=bytes(out)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('Windows emulated path trace drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(req),'profile':profile,'selected_go_declarations':selected,'scope':'actual pinned std Windows Path/OsStr vs complete portable Go path helpers; both Windows-target PE binaries under Wine on Linux; no native Windows kernel validation or std file credit','raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'native_wrapper_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'go_toolchain':subprocess.run([args.go,'version'],check=True,stdout=subprocess.PIPE).stdout.decode().strip()}
 path=DEST.with_name('source_map_path_windows_wine_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('Windows emulated path manifest drift')
 else:path.write_text(render)
 print(len(req),'Windows Path/OsStr records: actual Rust and Go PE binaries agree under Wine; not native Windows kernel validation')
if __name__=='__main__':main()
