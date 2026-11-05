#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,importlib.util,json,random,subprocess,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[3];HERE=Path(__file__).absolute().parent
DEST=ROOT/'port/support/rustfmt/testdata/panic-location-cells.tsv.gz'
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();spec=importlib.util.spec_from_file_location('ferrugo_location_sdk',ROOT/'tools/port-rust-str/prepare.py');sdk=importlib.util.module_from_spec(spec);spec.loader.exec_module(sdk);rustc,identity=sdk.selected(args.rustc)
 seqs=['','a','b','aa','bb','ab','ba','aba','abab','aabb','bbaa'];rng=random.Random(0x708c94)
 seqs += [''.join(rng.choice('ab')for _ in range(rng.randrange(1,25)))for _ in range(96)]
 request=''.join(f'{i}\t{s}\n'for i,s in enumerate(seqs)).encode()
 with tempfile.TemporaryDirectory(prefix='ferrugo-location-cells-')as tmp:
  binary=Path(tmp)/'original';subprocess.run([rustc,'--edition=2024',str((HERE/'oracle.rs').relative_to(ROOT)),'-o',str(binary)],cwd=ROOT,check=True,timeout=60)
  output=subprocess.run([str(binary)],input=request,stdout=subprocess.PIPE,cwd=ROOT,check=True,timeout=10).stdout.splitlines()
 if len(output)!=len(seqs):raise RuntimeError('missing native location-cell observations')
 raw=b''.join(f'{seq}\t'.encode()+row.split(b'\t',1)[1]+b'\n'for seq,row in zip(seqs,output))
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=raw:raise RuntimeError('native location-cell drift')
 elif not DEST.exists()or gzip.decompress(DEST.read_bytes())!=raw:DEST.write_bytes(gzip.compress(raw,mtime=0))
 proof={'schema':1,'scope':'Safe public actual Rust repeated/different static panic-location reference identities; pairwise equality only, no numeric addresses or Rust/Go pointer ABI claim','compiler':identity.strip(),'scripts':len(seqs),'hook_calls':sum(map(len,seqs)),'pairwise_comparisons':sum(len(s)**2 for s in seqs),'raw_sha256':sha(raw),'gzip_sha256':sha(DEST.read_bytes()),'wrapper_sha256':sha((HERE/'oracle.rs').read_bytes()),'tool_sha256':sha(Path(__file__).read_bytes()),'go_sources':{name:sha((ROOT/name).read_bytes())for name in ['port/support/rustfmt/panic.go','port/support/rustfmt/panic_location_cells_test.go']},'location_profile':'Exact relative wrapper filename passed to rustc; no posthoc native path normalization. Production location cells are caller-owned immutable profile metadata.'}
 path=HERE/'VALIDATION.json';render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('location-cell proof manifest drift')
 else:path.write_text(render)
 print(len(seqs),'actual native location-cell scripts;',proof['hook_calls'],'hooks;',proof['pairwise_comparisons'],'pairwise comparisons')
if __name__=='__main__':main()
