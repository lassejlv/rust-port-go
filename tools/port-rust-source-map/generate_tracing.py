#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'))
import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_tracing.tsv.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--verbatim',action='store_true');args=p.parse_args();rows=[];events=0
 global DEST
 if args.verbatim:DEST=DEST.with_name('source_map_tracing_verbatim.tsv.gz')
 configs=[('always','trace','true'),('sometimes','trace','true'),('sometimes','trace','false'),('never','trace','true'),('always','info','true'),('always','debug','true'),('always','off','true'),('always','trace','false')]
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-tracing-') as tmp:
  for d in (False,True):
   for o in (False,True):
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key;command=[profile['rustc'],*profile['link_arguments']]
    for f in profile['package_artifacts']['tracing'][0]['filenames']:command+=['--extern','tracing='+f]
    subprocess.run([*command,str(HERE/'tracing.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    count=0
    for interest,max_level,enabled in configs:
     for op in (['construct','valid','lines','point','empty','malformed','different','remap','remap_miss','reverse'] if args.verbatim else ['construct','valid','lines','point','empty','malformed','different','remap']):
      for style in range(4):
       for fail in (0,1,3):
        fields=[interest,max_level,enabled,op,str(style),str(fail)]
        data=subprocess.run([str(binary),*fields,*( ["verbatim"] if args.verbatim else [])],stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=10).stdout;events+=len(data.splitlines());rows.append('\t'.join([key,*fields,data.hex()])+'\n');count+=1
    print(key,count,'actual SourceMap tracing scripts',flush=True)
 data=''.join(rows).encode();DEST.parent.mkdir(parents=True,exist_ok=True)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise SystemExit('SourceMap tracing native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','scripts':len(rows),'events':events,'profiles':['d0o0','d0o1','d1o0','d1o1'],'wrapper_sha256':hashlib.sha256((HERE/'tracing.rs').read_bytes()).hexdigest(),'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'normalization':'Only original/ copied-source prefix is removed from Metadata.file and generated event name. Every other field, writer C/S call, context flag and state is retained.','complete':False,'remaining':'Subscriber/loader panic ownership, imported reverse-map, inherited/processing contexts and extra mapping paths remain WIP.'}
 if args.verbatim:manifest['normalization']='NONE: all native metadata names, file spellings, source lines, fields, writers and events retained verbatim; selected Go profiles match the original copied-source build.'
 path=DEST.with_name('source_map_tracing_verbatim_manifest.json' if args.verbatim else 'source_map_tracing_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise SystemExit('source tracing manifest drift')
 else:path.write_text(render)
 print(len(rows),'scripts;',events,'events')
if __name__=='__main__':main()
