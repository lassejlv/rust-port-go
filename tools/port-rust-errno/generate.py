#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Standalone actual SDK std-vs-host-cgo errno/CStr proof (Linux/macOS/Unix).

--check always executes both implementations and compares current native
results; it does not pretend Linux's libc messages are macOS fixture data.
"""
import argparse, hashlib, importlib.util, json, os, platform, random, shutil, subprocess, tarfile, tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
def module(name,path):
 s=importlib.util.spec_from_file_location(name,path);v=importlib.util.module_from_spec(s);s.loader.exec_module(v);return v
def sha(data):return hashlib.sha256(data).hexdigest()
def requests():
 rows=[]
 for code in [*range(-256,4097),-(1<<31),(1<<31)-1]:rows.append(('errno',str(code),0))
 for code in [-1,0,1,2,5,22,34,84,133,4096,-(1<<31),(1<<31)-1]:
  for fail in range(1,9):rows.append(('errno',str(code),fail))
 for byte in range(1,256):rows.append(('bytes',bytes([byte]).hex(),0))
 for first in range(1,256):
  for second in range(1,256):rows.append(('bytes',bytes([first,second]).hex(),0))
 rng=random.Random(0x708c94)
 cases=[b'',b'abc',b'\xe1\x80',b'\xf1\x80\x80',b'a\xffb',b'\xe0\x80\x80',b'\xed\xa0\x80',b'\xf4\x90\x80\x80']
 cases += [bytes(rng.randrange(1,256)for _ in range(rng.randrange(1,128)))for _ in range(2048)]
 for value in cases:
  for fail in [0,1,2,3,5,8,13]:rows.append(('bytes',value.hex(),fail))
 return b''.join(f'{i}\t{op}\t{value}\t{fail}\n'.encode()for i,(op,value,fail)in enumerate(rows))
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--go');p.add_argument('--output',type=Path);args=p.parse_args()
 sdk=module('ferrugo_errno_sdk',ROOT/'tools/port-rust-str/prepare.py');compiler,identity=sdk.selected(args.rustc);source_meta,cache=sdk.verified()
 selected=args.go if args.go is not None else os.environ.get('FERRUGO_GO',str(ROOT/'.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go'))
 if not selected:raise RuntimeError('selected Go executable is explicitly empty')
 go=shutil.which(selected)
 if go is None:raise RuntimeError('selected Go executable is missing')
 env=os.environ.copy();env['CGO_ENABLED']='1';data=requests()
 sources={}
 with tarfile.open(cache/'rust-src-nightly.tar.xz','r:xz')as archive:
  for name in ['library/std/src/sys/io/error/unix.rs','library/core/src/ffi/c_str.rs','library/core/src/byte_str/mod.rs']:
   original=archive.extractfile('rust-src-nightly/rust-src/lib/rustlib/src/rust/'+name).read();sources[name]={'bytes':len(original),'sha256':sha(original)}
 with tempfile.TemporaryDirectory(prefix='ferrugo-native-errno-')as tmp:
  native=Path(tmp)/'original';ported=Path(tmp)/'ported'
  subprocess.run([compiler,str(HERE/'oracle.rs'),'-o',str(native)],cwd=ROOT,check=True,timeout=60)
  subprocess.run([go,'build','-o',str(ported),'./tools/port-rust-errno/replay'],cwd=ROOT,env=env,check=True,timeout=120)
  outputs=[subprocess.run([str(binary)],input=data,stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=60).stdout for binary in [native,ported]]
  if outputs[0]!=outputs[1]:
   for req,rust,goout in zip(data.splitlines(),outputs[0].splitlines(),outputs[1].splitlines()):
    if rust!=goout:raise RuntimeError(f'native errno mismatch {req!r}: Rust={rust!r}, Go={goout!r}')
   raise RuntimeError('native errno result count differs')
 retained=json.loads((HERE/'SOURCES.json').read_text())
 if retained['sources']!=sources or retained['archive_sha256']!=source_meta['sha256'] or retained['compiler_commit']!=sdk.COMMIT:raise RuntimeError('retained SDK source inventory differs')
 gofiles=[*sorted((ROOT/'port/support/rusterrno').glob('*.go')),*sorted((ROOT/'port/support/rustfmt').glob('*.go')),HERE/'replay/main.go']
 proof={'schema':1,'scope':'Actual host std Error::Display and CStr::display vs scoped SDK Unix strerror_r/UTF8-chunk adapter; no standard-library source-file credit','compiler':identity.strip(),'sdk_archive_sha256':source_meta['sha256'],'sources':sources,'host':platform.platform(),'libc':platform.libc_ver(),'records':len(data.splitlines()),'request_sha256':sha(data),'output_sha256':sha(outputs[0]),'go':subprocess.run([go,'version'],stdout=subprocess.PIPE,check=True,timeout=15).stdout.decode().strip(),'cgo':True,'go_sources':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in gofiles if not path.name.endswith('_test.go')},'tools':{name:sha((HERE/name).read_bytes())for name in ['generate.py','oracle.rs','SOURCES.json']}}
 if args.output:args.output.write_text(json.dumps(proof,indent=2)+'\n')
 elif not args.check:(HERE/'VALIDATION.json').write_text(json.dumps(proof,indent=2)+'\n')
 print(proof['records'],'actual native errno/CStr writer records agree on',platform.system())
if __name__=='__main__':main()
