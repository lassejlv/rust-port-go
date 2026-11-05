#!/usr/bin/env python3
"""Compare actual concrete/alias/Span-Self projections versus Rust wrapper types."""
import argparse,json,subprocess,tempfile
from pathlib import Path
import reference
ROOT=reference.ROOT;HERE=Path(__file__).resolve().parent
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-byte-slice-strategy.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--reference-root',type=Path,default=ROOT/'.toolchain/oracles/rustc-span-reference');args=p.parse_args();rows=[];profiles=[]
 source=HERE/'native_byte_slice_strategy.rs'
 with tempfile.TemporaryDirectory(prefix='ferrugo-lib-byte-strategy-')as tmp:
  for debug in [False,True]:
   for overflow in [False,True]:
    name=f'd{int(debug)}o{int(overflow)}';profile,env=reference.load_profile(args.reference_root/f'build-profile-{name}.json')
    binary=Path(tmp)/name;subprocess.run([profile['rustc'],*profile['link_arguments'],str(source.relative_to(ROOT)),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    result=subprocess.run([str(binary)],cwd=ROOT,env=env,stdout=subprocess.PIPE,check=True,timeout=20);rows.extend(name.encode()+b'\t'+line+b'\n'for line in result.stdout.splitlines())
    profiles.append({'name':name,'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256']})
 raw=b''.join(rows);compressed=reference.compressed_fixture(raw,DEST,2*1024*1024)
 manifest={'schema':1,'status':'scoped-byte-slice-source-Self-closure-no-extra-file-credit','source_revision':reference.PIN,'profiles':profiles,'records':len(rows),'native_client_sha256':reference.sha(source.read_bytes()),'generator_sha256':reference.sha(Path(__file__).read_bytes()),'reference_tool_inputs_sha256':reference.tool_inputs(),'raw_sha256':reference.sha(raw),'fixture_sha256':reference.sha(compressed),'scope':'Complete unchanged original rustc_serialize+rustc_span; concrete MemEncoder, exact type alias and generic SpanEncoder actual-Self projection use opaque specialization. Distinct safe Rust Wrapped(MemEncoder) uses generic EmitU8 operations. Actual capacities/bytes and wrapper method calls retained. SourceFileHash derived array encoding included. No inheritable marker discovery or Rust production fallback.'}
 path=HERE/'byte-slice-strategy-manifest.json';render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if not DEST.exists()or DEST.read_bytes()!=compressed or not path.exists()or path.read_text()!=render:raise RuntimeError('byte-slice source-Self evidence differs')
 else:DEST.parent.mkdir(parents=True,exist_ok=True);DEST.write_bytes(compressed);path.write_text(render)
 print(len(rows),'actual concrete/alias/Span-Self/wrapper byte-slice records')
if __name__=='__main__':main()
