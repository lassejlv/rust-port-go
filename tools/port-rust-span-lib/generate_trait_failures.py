#!/usr/bin/env python3
"""Actual safe custom-Decoder capacity and inherited Idx failure operations."""
import argparse,json,os,resource,signal,subprocess,tempfile
from pathlib import Path
import reference
ROOT=reference.ROOT;HERE=Path(__file__).resolve().parent
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-trait-failures.jsonl.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--reference-root',type=Path,default=ROOT/'.toolchain/oracles/rustc-span-reference');args=p.parse_args();rows=[];profiles=[];aborts=0
 source=HERE/'native_trait_failures.rs'
 with tempfile.TemporaryDirectory(prefix='ferrugo-lib-trait-failure-')as tmp:
  for debug in [False,True]:
   for overflow in [False,True]:
    name=f'd{int(debug)}o{int(overflow)}';profile,env=reference.load_profile(args.reference_root/f'build-profile-{name}.json')
    binary=Path(tmp)/name;subprocess.run([profile['rustc'],*profile['link_arguments'],str(source.relative_to(ROOT)),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for op in ['vec_3','vec_4','clamp','idx_0_0','idx_1_18446744073709551615','idx_4294967040_1','idx_42_18446744073709551615']:
     for ambient in ['normal','unwinding','processing']:
      result=subprocess.run([str(binary),op,ambient],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
      if result.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected source failure exit{name}/{op}/{ambient}:{result.returncode}:{result.stderr!r}')
      aborted=result.returncode==-signal.SIGABRT;aborts+=aborted
      rows.append({'profile':name,'op':op,'ambient':ambient,'abort':aborted,'stdout':result.stdout.decode()})
    profiles.append({'name':name,'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256']})
 raw=b''.join((json.dumps(row,sort_keys=True,separators=(',',':'))+'\n').encode()for row in rows);compressed=reference.compressed_fixture(raw,DEST,4*1024*1024)
 manifest={'schema':1,'status':'lib-inherited-failure-closure-no-extra-file-credit','source_revision':reference.PIN,'profiles':profiles,'records':len(rows),'actual_SIGABRT':aborts,'native_client_sha256':reference.sha(source.read_bytes()),'generator_sha256':reference.sha(Path(__file__).read_bytes()),'reference_tool_inputs_sha256':reference.tool_inputs(),'raw_sha256':reference.sha(raw),'fixture_sha256':reference.sha(compressed),'scope':'Complete unchanged original crate; safe Decoder overrides read_usize3/4 withusizeMAX after consuming actual original emptySourceFile encoding, no fabricated Vec/private states. Exact32/64position events, static capacity-overflow beforeelementcalls andactualdecoderDrop. Inherited AttrId Idx increment versusmacro Add metadata, invalidOrdclamp, allnormal/inherited/processing operations. Wrapper source compiled by portable relative filename; all raw native hook filenames retained; terminal SIGABRTstderr notcompared.'}
 path=HERE/'trait-failures-manifest.json';render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if not DEST.exists()or DEST.read_bytes()!=compressed or not path.exists()or path.read_text()!=render:raise RuntimeError('lib trait failure evidence differs')
 else:DEST.parent.mkdir(parents=True,exist_ok=True);DEST.write_bytes(compressed);path.write_text(render)
 print(len(rows),'original failure cases,',aborts,'actual SIGABRT')
if __name__=='__main__':main()
