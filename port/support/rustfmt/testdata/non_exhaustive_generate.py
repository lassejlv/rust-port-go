#!/usr/bin/env python3
"""Retain/recheck actual DebugStruct::finish_non_exhaustive writer contracts."""
import argparse,gzip,hashlib,json,os,pathlib,shutil,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[3]
RUSTC='rustc 1.99.1 (58cf8f9c3 2026-10-13)'
def digest(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');a=p.parse_args()
 selected=os.environ.get('FERRUGO_RUSTC')
 rustc=shutil.which(selected) if selected else shutil.which('rustc')
 if selected and not rustc:raise SystemExit('FERRUGO_RUSTC does not resolve: '+selected)
 if not rustc:rustc=str(ROOT/'.toolchain/rust/bin/rustc')
 if subprocess.check_output([rustc,'--version'],text=True).strip()!=RUSTC:raise SystemExit('Rust reference compiler differs')
 requests=''.join(f'{pattern}\t{count}\t{str(chars).lower()}\t{fail}\t{str(reuse).lower()}\n' for pattern in range(16) for count in range(4) for chars in [False,True] for reuse in [False,True] for fail in [-1,*range(81)])
 with tempfile.TemporaryDirectory(prefix='ferrugo-non-exhaustive-') as tmp:
  executable=str(pathlib.Path(tmp)/'oracle')
  subprocess.run([rustc,'--edition=2024',str(HERE/'non_exhaustive.rs'),'-o',executable],check=True)
  result=subprocess.run([executable],input=requests.encode(),stdout=subprocess.PIPE,check=True).stdout
 fixture=HERE/'non_exhaustive.tsv.gz';metadata=HERE/'non_exhaustive_manifest.json'
 compressed=gzip.compress(result,mtime=0)
 if fixture.exists() and gzip.decompress(fixture.read_bytes())==result:compressed=fixture.read_bytes()
 manifest={'scope':'Scoped Rust1.99.1 DebugStruct::finish_non_exhaustive supporting contract; no std-file completion credit','rustc':RUSTC,'cases':len(result.splitlines()),'requests_sha256':digest(requests.encode()),'raw_sha256':digest(result),'fixture_sha256':digest(compressed),'source_sha256':digest((HERE/'non_exhaustive.rs').read_bytes()),'generator_sha256':digest(pathlib.Path(__file__).read_bytes())}
 if a.check:
  old=json.loads(metadata.read_text())
  if old!=manifest or fixture.read_bytes()!=compressed:raise SystemExit('Non-exhaustive original-reference proof differs')
 else:
  fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print(f'{manifest["cases"]} native non-exhaustive builder cases match')
if __name__=='__main__':main()
