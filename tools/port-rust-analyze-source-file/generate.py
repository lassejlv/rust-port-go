#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,os,random,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'))
import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/analyze_source_file.tsv.gz'
def requests():
 rng=random.Random(0xa2d20);texts=['','\n','a\nβ\n🦀','\n'*100,'β'*100]
 for size in range(0,81):
  for c in ['β','…','🦀','\n','\0','\t']:
   texts.append('a'*size+c+'a'*40+'\n')
 atoms=['a','b','\n','\r','\t','\0','β','…','🦀','\u0085','\u3000']
 texts+=[''.join(rng.choice(atoms) for _ in range(rng.randrange(200))) for _ in range(1024)]
 i=0
 for text in texts:
  for mode in ['full','sse2','generic']:
   yield f'{i}\t{mode}\t{text.encode().hex()}\t{len(text.encode())}\t0\t0\t0\t0\t0\n';i+=1
 for text in ['', '\n','a\nβ…🦀\na','β…🦀\na','aaa\nβ','β'*20,'\n'*20]:
  for scan in range(len(text.encode())+2):
   for offset in [0,1,0xffffffff,0xfffffffe,0xfffffff8,0xffffffc0]:
    for length,capacity in [(0,0),(1,1),(3,16),(4,4),(7,7)]:
     yield f'{i}\tgeneric\t{text.encode().hex()}\t{scan}\t{offset}\t{length}\t{capacity}\t{length}\t{capacity}\n';i+=1

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();rawin=''.join(requests()).encode();out=bytearray()
 source=ROOT/'.toolchain/upstream-rust/compiler/rustc_span/src/analyze_source_file.rs'
 with tempfile.TemporaryDirectory(prefix='ferrugo-analyze-') as tmp:
  for d in (False,True):
   for o in (False,True):
    key=f'd{int(d)}o{int(o)}';graph,env,_=reference.load(d,o,args.rustc)
    source=Path(graph['original_root'])/'compiler/rustc_span/src/analyze_source_file.rs';binary=Path(tmp)/key;env['FERRUGO_ANALYZE_SOURCE']=str(source)
    subprocess.run([graph['rustc'],*graph['link_arguments'],str(HERE/'oracle.rs'),'-o',str(binary)],env=env,cwd=ROOT,check=True,timeout=60)
    result=subprocess.run([str(binary)],input=rawin,stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=30).stdout.splitlines();requests_list=rawin.splitlines()
    if len(result)!=len(requests_list):raise RuntimeError('record count')
    for req,res in zip(requests_list,result):
     ri,*fields=res.split(b'\t');qi,*_=req.split(b'\t');assert ri==qi;out.extend(key.encode()+b'\t'+req+b'\t'+b'\t'.join(fields)+b'\n')
    print(key,len(result),'native records',flush=True)
 data=bytes(out);DEST.parent.mkdir(parents=True,exist_ok=True)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise SystemExit('analyzer oracle drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 tests=source.parent/'analyze_source_file/tests.rs'
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'source_lines':len(source.read_bytes().splitlines()),'tests_sha256':hashlib.sha256(tests.read_bytes()).hexdigest(),'tests_lines':len(tests.read_bytes().splitlines()),'records':len(data.splitlines()),'profiles':['d0o0','d0o1','d1o0','d1o1'],'rustc_version':graph['rustc_version'],'graph_provenance_sha256':graph['provenance_sha256'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'oracle.rs').read_bytes()).hexdigest(),'native_architecture':'x86_64+sse2','lsx':'Portable Go mask/control-flow branch is compared with original SSE2/generic; native LSX hardware has not been executed.'}
 path=DEST.with_name('analyze_source_file_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise SystemExit('analyzer manifest drift')
 else:path.write_text(render)
if __name__=='__main__':main()
