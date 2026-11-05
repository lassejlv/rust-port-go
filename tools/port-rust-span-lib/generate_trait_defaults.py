#!/usr/bin/env python3
"""Retain actual inherited trait operations from the complete pinned crate."""
import argparse,json,os,subprocess,tempfile
from pathlib import Path
import reference
ROOT=reference.ROOT;HERE=Path(__file__).resolve().parent
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-trait-defaults.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--reference-root',type=Path,default=ROOT/'.toolchain/oracles/rustc-span-reference');args=p.parse_args();rows=[];profiles=[]
 source=HERE/'native_trait_defaults.rs'
 with tempfile.TemporaryDirectory(prefix='ferrugo-lib-defaults-')as tmp:
  for debug in [False,True]:
   for overflow in [False,True]:
    name=f'd{int(debug)}o{int(overflow)}';profile,env=reference.load_profile(args.reference_root/f'build-profile-{name}.json')
    binary=Path(tmp)/name;subprocess.run([profile['rustc'],*profile['link_arguments'],str(source),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    result=subprocess.run([str(binary)],cwd=ROOT,env=env,stdout=subprocess.PIPE,check=True,timeout=30)
    rows.extend(name.encode()+b'\t'+line+b'\n'for line in result.stdout.splitlines())
    profiles.append({'name':name,'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256']})
 raw=b''.join(rows);compressed=reference.compressed_fixture(raw,DEST,16*1024*1024)
 manifest={'schema':1,'status':'lib-inherited-trait-closure-no-extra-file-credit','source_revision':reference.PIN,'profiles':profiles,'records':len(rows),'native_client_sha256':reference.sha(source.read_bytes()),'generator_sha256':reference.sha(Path(__file__).read_bytes()),'reference_tool_inputs_sha256':reference.tool_inputs(),'raw_sha256':reference.sha(raw),'fixture_sha256':reference.sha(compressed),'scope':'Actual public derived Ord/PartialOrd/PartialEq and inherited min/max/clamp from complete unchanged rustc_span. All triples of10public-value fixture classes including allfilenamealternatives, normalizedPath equality and integer limits. Owned filename min/max/clamp ties compared by within-native-process allocation identity labels, no cross-language addresses/allocator ABI. Private InnerRealFileName comparisons exercised indirectly through RealFileName, no direct private API access claimed.'}
 path=HERE/'trait-defaults-manifest.json';render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if not DEST.exists()or DEST.read_bytes()!=compressed or not path.exists()or path.read_text()!=render:raise RuntimeError('lib trait default evidence differs')
 else:DEST.parent.mkdir(parents=True,exist_ok=True);DEST.write_bytes(compressed);path.write_text(render)
 print(len(rows),'actual original inherited trait records; four profiles')
if __name__=='__main__':main()
