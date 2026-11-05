#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,importlib.util,json,subprocess,tarfile,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_binary_layout.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();spec=importlib.util.spec_from_file_location('binary_arc_sdk',ROOT/'tools/port-rust-str/prepare.py');sdk=importlib.util.module_from_spec(spec);spec.loader.exec_module(sdk);compiler,identity=sdk.selected(args.rustc);meta,cache=sdk.verified()
 config=subprocess.run([compiler,'--print','cfg'],stdout=subprocess.PIPE,check=True,timeout=10).stdout.decode().splitlines();bits=int(next(line.split('=')[1].strip('"')for line in config if line.startswith('target_pointer_width=')))
 if bits not in [32,64]:raise RuntimeError('unsupported native source usize profile')
 maximum=(1<<(bits-1))-1;inputs=[]
 for length in [0,1,2,7,8,15,16,31,32,255,1<<32,*range(maximum-32,maximum+2),(1<<64)-1]:inputs.append(['layout',str(length)])
 for length in [0,1,32,*range(maximum-(3*(bits//8)-2),maximum+2),(1<<64)-1]:inputs.append(['actual',str(length)])
 data=''.join(f'{i}\t'+ '\t'.join(r)+'\n'for i,r in enumerate(inputs)).encode()
 with tempfile.TemporaryDirectory(prefix='ferrugo-arc-layout-')as tmp:
  binary=Path(tmp)/'original';subprocess.run([compiler,str(HERE/'binary_layout.rs'),'-o',str(binary)],cwd=ROOT,check=True,timeout=60);lines=subprocess.run([str(binary)],input=data,stdout=subprocess.PIPE,cwd=ROOT,check=True,timeout=10).stdout.splitlines()
 if len(lines)!=len(inputs):raise RuntimeError('missing Arc/layout records')
 raw=b''.join(f'{bits}\t'.encode()+'\t'.join(request).encode()+b'\t'+line.split(b'\t',1)[1]+b'\n'for request,line in zip(inputs,lines))
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=raw:raise RuntimeError('Arc/layout native drift')
 elif not DEST.exists()or gzip.decompress(DEST.read_bytes())!=raw:DEST.write_bytes(gzip.compress(raw,mtime=0))
 with tarfile.open(cache/'rust-src-nightly.tar.xz','r:xz')as archive:
  names=['library/alloc/src/rcs/arc.rs','library/core/src/alloc/layout.rs'];sources={name:hashlib.sha256(archive.extractfile('rust-src-nightly/rust-src/lib/rustlib/src/rust/'+name).read()).hexdigest()for name in names}
 proof={'schema':1,'records':len(inputs),'compiler':identity.strip(),'sdk_archive_sha256':meta['sha256'],'sdk_sources':sources,'raw_sha256':hashlib.sha256(raw).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'binary_layout.rs').read_bytes()).hexdigest(),'scope':'Actual Arc<[u8]>::new_uninit_slice for small valid/large invalid lengths, plus exact public Layout recipe for boundary-valid huge lengths without allocation. Header shim is only repr(C) two actual atomic usize counters and unit, matching selected SDK ArcInner<()> fields; no Arc or allocator file credit. No fake SourceFile/File metadata and no huge allocations.'}
 path=DEST.with_name('source_map_binary_layout_manifest.json');render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('Arc/layout manifest drift')
 else:path.write_text(render)
 print(len(inputs),'actual SDK Arc/public-layout records')
if __name__=='__main__':main()
