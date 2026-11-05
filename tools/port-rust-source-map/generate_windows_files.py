#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,os,subprocess,sys
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(HERE/'windows-reference'));import prepare
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_files_windows_wine.tsv.gz'
def requests():
 paths=['','empty','text','invalid','bom','directory','missing','missing\\child','text\\child','link','broken','dirlink','w\ud800x','a\x00b','NUL','nul','./text','.\\text','directory\\..\\text','text.','text ']
 paths += ['bytes'+str(n) for n in [1,2,15,16,31,32,33,63,64,65,511,512,513,4096]]
 long='\\'.join(['abcdefghijklmnopqrstuvwx']*28);paths += [long+'\\deep.txt',long+'\\missing']
 ident=0
 for name in paths:
  for op in ['exists','open_meta','open_read','text']:
   yield f'{ident}\t{op}\t{name.encode("utf-16-le","surrogatepass").hex()}\n';ident+=1
 for name in ['directory',long,'missing']:
  yield f'{ident}\tcwd\t{name.encode("utf-16-le").hex()}\n';ident+=1

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--fetch',action='store_true');p.add_argument('--original-tests',action='store_true',help='also execute translated original SourceMap tests in the real Go PE package');p.add_argument('--rustc');p.add_argument('--go',default=str(ROOT/'.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go'));args=p.parse_args();command,env,profile=prepare.prepare(args.fetch,args.rustc);cache=prepare.CACHE
 native=cache/'files-native.exe';source=HERE/'files_windows.rs';subprocess.run([*command,str(source),'-o',str(native)],cwd=ROOT,env=env,check=True,timeout=60)
 fixtures=cache/'fs-fixtures';fixtures.mkdir(exist_ok=True);setup=subprocess.run([env['WINELOADER'],str(native),'--setup'],cwd=fixtures,env=env,check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60)
 gobinary=cache/'span-go-test.exe';goenv=os.environ.copy();goenv.update(GOOS='windows',GOARCH='amd64',GOCACHE='/tmp/ferrugo-go123-cache');subprocess.run([args.go,'test','-c','-o',str(gobinary),'./port/rust/compiler/rustc_span/src'],cwd=ROOT,env=goenv,check=True,timeout=120)
 if args.original_tests:
  original_binary=cache/'source-map-original-go-test.exe';subprocess.run([args.go,'test','-c','-o',str(original_binary),'./port/rust/compiler/rustc_span/src/source_map'],cwd=ROOT,env=goenv,check=True,timeout=120)
  result=subprocess.run([env['WINELOADER'],str(original_binary),'-test.run=^TestSourceMapOriginal','-test.v'],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60)
  (cache/'original-go-tests-wine.log').write_bytes(result.stdout+result.stderr)
  if result.returncode:raise RuntimeError('translated original tests failed under Wine: '+result.stdout.decode(errors='replace')+result.stderr.decode(errors='replace'))
 data=''.join(requests()).encode();rows=data.splitlines();observations=[]
 for binary,extra in [(native,[]),(gobinary,['-test.run=^TestSourceMapWindowsFilesChild$'])]:
  child=env.copy();child['FERRUGO_WINDOWS_FILES_CHILD']='1';result=subprocess.run([env['WINELOADER'],str(binary),*extra],input=data,cwd=fixtures,env=child,check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60);observations.append(result.stdout.splitlines());(cache/(binary.stem+'-files-stderr.txt')).write_bytes(result.stderr)
 if any(len(v)!=len(rows) for v in observations):raise RuntimeError('missing native/Go Windows file records')
 out=bytearray()
 for req,rust,go in zip(rows,*observations):
  if rust!=go:raise RuntimeError(f'emulated filesystem mismatch request={req!r} native={rust!r} go={go!r}')
  ident,value=rust.split(b'\t');assert ident==req.split(b'\t')[0];out.extend(req+b'\t'+value+b'\n')
 data=bytes(out)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('emulated Windows file trace drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(rows),'profile':profile,'fixture_setup_stderr':setup.stderr.decode(errors='replace'),'scope':'Actual std Windows Path.exists/File.open/metadata/read/read_to_string/current_dir used host contracts vs actual whole Go span backend; both PE binaries under Wine. Real filesystem fixtures, invalid/NUL/surrogate paths, files/directories/symlinks/missing paths, device NUL and >512 UTF16 paths. No Windows-kernel claim. ReadBinary lying-stat protocol remains separately proved on native Linux.','raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'native_wrapper_sha256':hashlib.sha256(source.read_bytes()).hexdigest()}
 path=DEST.with_name('source_map_files_windows_wine_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('Windows file manifest drift')
 else:path.write_text(render)
 print(len(rows),'actual std Windows filesystem records agree with full Go backend under Wine; no Windows-kernel claim')
if __name__=='__main__':main()
