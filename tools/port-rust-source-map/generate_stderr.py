#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,os,resource,shlex,signal,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_stderr.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--cc');args=p.parse_args()
 if not sys.platform.startswith('linux'):raise RuntimeError('retained full-crate stderr injection proof is Linux-only')
 cc=args.cc if args.cc is not None else os.environ.get('CC','cc')
 if not cc:raise RuntimeError('selected CC explicitly empty')
 rows=[];aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-stderr-')as tmp:
  library=Path(tmp)/'stderr_errno.so';subprocess.run([*shlex.split(cc),'-shared','-fPIC',str(HERE/'stderr_errno.c'),'-o',str(library)],cwd=ROOT,check=True,timeout=30)
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'stderr.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for mode in ['normal','inherited_caught','inherited_escape','processing']:
     for error in ['open','closed','0','9','133','500','4095','-1','capture','held']:
      child=env.copy()
      if error!='closed':child.update(LD_PRELOAD=str(library),FERRUGO_STDERR_ERRNO=('pass' if error in ['open','capture','held'] else error))
      r=subprocess.run([str(binary),mode,error],cwd=ROOT,env=child,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
      if r.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected stderr native exit {key}/{mode}/{error}: {r.returncode}/{r.stderr!r}')
      kind='ok'if r.returncode==0 else'abort';aborts+=kind=='abort';rows.append('\t'.join([key,mode,error,kind,r.stdout.hex(),r.stderr.hex()if kind=='ok'else''])+'\n')
 data=''.join(rows).encode()
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise RuntimeError('SourceMap stderr native drift')
 elif not DEST.exists()or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 meta={'schema':1,'records':len(rows),'actual_SIGABRT':aborts,'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','wrapper_sha256':hashlib.sha256((HERE/'stderr.rs').read_bytes()).hexdigest(),'ffi_injection_sha256':hashlib.sha256((HERE/'stderr_errno.c').read_bytes()).hexdigest(),'scope':'Actual full SourceMap fatal new_source_file path with safe metadata-only MAX import. Actual SDK eprintln, open/closed stderr plus explicit child-only libc write errno injection incl unknown500; all hook payload/kind/location, fatal marker and runtime/cleanup outcomes. No original source body modified. Only wrapper filename prefix removed; ordinary SDK paths exact. Abort stderr runtime diagnostic text is not signal ABI and stdout hook events are retained.'}
 path=DEST.with_name('source_map_stderr_manifest.json');render=json.dumps(meta,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('SourceMap stderr manifest drift')
 else:path.write_text(render)
 print(len(rows),'whole-crate stderr scripts,',aborts,'actual SIGABRT')
if __name__=='__main__':main()
