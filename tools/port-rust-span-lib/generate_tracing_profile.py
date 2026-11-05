#!/usr/bin/env python3
"""Observe complete lib.rs tracing with verbatim actual metadata and lazy fields."""
import argparse,json,subprocess,tempfile
from pathlib import Path
import reference
ROOT=reference.ROOT;HERE=Path(__file__).resolve().parent
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-tracing-profile.jsonl.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--reference-root',type=Path,default=ROOT/'.toolchain/oracles/rustc-span-reference');args=p.parse_args();rows=[];profiles=[];metadata={}
 source=HERE/'native_tracing_profile.rs'
 with tempfile.TemporaryDirectory(prefix='ferrugo-lib-tracing-profile-')as tmp:
  for debug in [False,True]:
   for overflow in [False,True]:
    name=f'd{int(debug)}o{int(overflow)}';profile,env=reference.load_profile(args.reference_root/f'build-profile-{name}.json')
    binary=Path(tmp)/name;command=[profile['rustc'],*profile['link_arguments']]
    for filename in profile['package_artifacts']['tracing'][0]['filenames']:command.extend(['--extern','tracing='+filename])
    subprocess.run([*command,str(source.relative_to(ROOT)),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for interest in ['always','sometimes','never']:
     for maximum in ['off','info','debug','trace']:
      for enabled in ['false','true']:
       for mode in ['present','missing']:
        query=[interest,maximum,enabled,mode];result=subprocess.run([str(binary),*query],cwd=ROOT,env=env,stdout=subprocess.PIPE,check=True,timeout=10)
        output=result.stdout.decode()
        for event in output.splitlines():
         if event.startswith('register|'):
          fields=event.split('|');line=fields[6];entry={'Name':fields[1],'Level':fields[2],'Target':fields[3],'ModulePath':fields[4],'File':fields[5],'Line':int(line),'Kind':fields[7],'Fields':fields[8].split(',')}
          if line in metadata and metadata[line]!=entry:raise RuntimeError('actual lib trace metadata differs across profiles')
          metadata[line]=entry
        rows.append({'profile':name,'interest':interest,'maximum':maximum,'enabled':enabled=='true','mode':mode,'stdout':output})
    profiles.append({'name':name,'rustc_version':profile['rustc_version'],'reference_provenance_sha256':profile['provenance_sha256']})
 if set(metadata)!={'2519','2546','2547','2548','2570'}:raise RuntimeError('not all original lib sites observed')
 raw=b''.join((json.dumps(row,sort_keys=True,separators=(',',':'))+'\n').encode()for row in rows);compressed=reference.compressed_fixture(raw,DEST,8*1024*1024)
 manifest={'schema':1,'status':'lib-verbatim-tracing-profile-no-extra-file-credit','source_revision':reference.PIN,'profiles':profiles,'records':len(rows),'actual_metadata':metadata,'native_client_sha256':reference.sha(source.read_bytes()),'generator_sha256':reference.sha(Path(__file__).read_bytes()),'reference_tool_inputs_sha256':reference.tool_inputs(),'raw_sha256':reference.sha(raw),'fixture_sha256':reference.sha(compressed),'scope':'Actual complete unchanged SourceFile lookup with present/unavailablecode, allfive libsites, repeateduse, realtracing subscriber/callsitefiltering/lazy fullmessage render/drop, all4profiles. Metadata Name/File/Target/ModulePath/Line/Fields retained VERBATIM, no observer path rewriting.'}
 path=HERE/'tracing-profile-manifest.json';render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if not DEST.exists()or DEST.read_bytes()!=compressed or not path.exists()or path.read_text()!=render:raise RuntimeError('lib verbatim tracing evidence differs')
 else:DEST.parent.mkdir(parents=True,exist_ok=True);DEST.write_bytes(compressed);path.write_text(render)
 print(len(rows),'verbatim complete-crate tracing cases; five actual sites')
if __name__=='__main__':main()
