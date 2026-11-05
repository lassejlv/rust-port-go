#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Exact Linux/GNU default-C-locale io::Error used-contract observations."""
import argparse,gzip,hashlib,json,os,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'))
import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_io_errors_linux.tsv.gz'
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--rustc');args=parser.parse_args()
 if not sys.platform.startswith('linux'):raise RuntimeError('native I/O proof requires the declared Linux/GNU host profile')
 libc=os.confstr('CS_GNU_LIBC_VERSION')
 if not libc or not libc.startswith('glibc '):raise RuntimeError('native I/O proof requires the declared GNU libc binding')
 profile,env,_=reference.load(rustc=args.rustc)
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-io-') as temporary:
  binary=Path(temporary)/'original';subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'io-errors.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  data=subprocess.run([str(binary)],cwd=ROOT,env=env,stdout=subprocess.PIPE,check=True,timeout=10).stdout
 previous=gzip.decompress(DEST.read_bytes()) if DEST.exists() else None
 if args.check:
  if previous!=data:raise RuntimeError('native Linux/GNU I/O error data drift')
 elif previous!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 path=DEST.with_name('source_map_io_errors_linux_manifest.json');previous_manifest=json.loads(path.read_text()) if path.exists() else {}
 producer=previous_manifest.get('fixture_producer_libc',libc) if previous==data else libc
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','records':len(data.splitlines()),'profile':'Linux/GNU default C locale; source io::Error from_raw_os_error(i32)','fixture_producer_libc':producer,'rustc_version':profile['rustc_version'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'io-errors.rs').read_bytes()).hexdigest(),'scope':'Kind + exact Display/Debug text for -256..4096 and i32 extrema; std/libc source files are not claimed ported. Message/writer/owned-error matrix is separate.'}
 rendered=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=rendered:raise RuntimeError('Linux/GNU IO manifest drift')
 else:path.write_text(rendered)
 print(len(data.splitlines()),'actual std OS-error cases;',libc)
if __name__=='__main__':main()
