#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,random,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'))
import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_path_unix.tsv.gz'
def requests():
 rng=random.Random(0x2d20c0);paths=[b'',b'/',b'//',b'.',b'./',b'..',b'../',b'a',b'a/',b'a/.',b'a/..',b'a//./b',b'/a//./b/',b'./a//./b/',b'a\x00b',b'\xed\xa0\x80',b'\xf0\x9f\xa6\x80']
 paths += [bytes([c]) for c in range(256)]
 parts=[b'',b'a',b'b',b'.',b'..',b'/',b'//',b'\xff',b'\xed\xa0\x80','β'.encode(),'\u200d'.encode(),'\U0001cfaf'.encode()]
 paths += [b'/'.join(rng.choice(parts) for _ in range(rng.randrange(12))) for _ in range(384)]
 paths += [bytes(rng.randrange(256) for _ in range(rng.randrange(0,40))) for _ in range(128)]
 id=0
 for p in paths:
  for op in ['view','hash','oshash','lossy','clone']:
   yield f'{id}\t{op}\t{p.hex()}\t\t\t0\t0\n';id+=1
  for script in ['FACFBACFBA','BCAFABCFA','FFFFFFFFFFFFFFFF','BBBBBBBBBBBBBBBB']:
   yield f'{id}\titer\t{p.hex()}\t\t{script}\t0\t0\n';id+=1
  for _ in range(3):
   q=rng.choice(paths)
   for op in ['compare','strip','join']:
    yield f'{id}\t{op}\t{p.hex()}\t{q.hex()}\t\t0\t0\n';id+=1
  for style in range(9):
   for fail in [0,1,3,10]:
    yield f'{id}\tdebug\t{p.hex()}\t\t\t{style}\t{fail}\n';id+=1

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--rustc');args=parser.parse_args();profile,env,_=reference.load(rustc=args.rustc);data=''.join(requests()).encode()
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-path-') as tmp:
  binary=Path(tmp)/'original';subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'path.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  observed=subprocess.run([str(binary)],input=data,stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=30).stdout.splitlines()
  req=data.splitlines();out=bytearray()
  if len(req)!=len(observed):raise RuntimeError('path native record count')
  for r,e in zip(req,observed):
   ri=r.split(b'\t')[0];ei,expect=e.split(b'\t');assert ri==ei;out.extend(r+b'\t'+expect+b'\n')
 data=bytes(out)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('path native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(observed),'host_profile':'actual matching1.101 std Unix Path/OsStr;64-bit','raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'path.rs').read_bytes()).hexdigest(),'scope':'All public used-path operations, hash method dispatch+actualborrowoffsets, ownedbufferindependence, mixeddoubleendedcursor state andallDebugC/Scalls+UTF8chunkaliases; Windows separatelypending; no stdsourcefilecredit.'}
 path=DEST.with_name('source_map_path_unix_manifest.json');rendered=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=rendered:raise RuntimeError('path manifest drift')
 else:path.write_text(rendered)
 print(len(observed),'original Unix std Path/OsStr records')
if __name__=='__main__':main()
