#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,subprocess,sys,tempfile
import panic_site_producer as producer
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2];HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_panic_sites.tsv.gz'
def requests():
 rows=[]
 for src in ['aé𐀀z','\x00\n\u0301','abc','\u200d\u034f\u0001\u00ad','\ufeffé\r\nΩ','']:
  for op in ['idx_empty','local_expect','doctest','indent','while','prev_while','start']:
   for i in range(len(src.encode())+2):
    for offset in (['-1','1',str(-(1<<63))] if op=='doctest' else ['0']):rows.append([src.encode().hex(),str(i),str(i),op,offset])
 for i in [0,1,1<<63,(1<<64)-1]:
  for offset in [-1,1,-(1<<63),(1<<63)-1]:rows.append(['',str(i),str(i),'doctest',str(offset)])
 return rows
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');p.add_argument('--max-domain',action='store_true');p.add_argument('--session-map',action='store_true');p.add_argument('--assert-id',action='store_true');p.add_argument('--width-metadata',action='store_true');p.add_argument('--output-dir',help='fresh verbatim session-map/assert-id producer artifacts; preserves historical evidence');args=p.parse_args();req=requests()
 if sum([args.max_domain,args.session_map,args.assert_id,args.width_metadata])>1:p.error('select only one panic-site scope')
 if args.output_dir is not None and not(args.session_map or args.assert_id):p.error('--output-dir requires --session-map or --assert-id')
 scope='session' if args.session_map else 'assert' if args.assert_id else None
 fresh=producer.evidence_paths(scope,args.output_dir) if scope else None
 bindings=producer.input_bindings() if args.output_dir is not None else None
 native_profiles=[]
 global DEST
 DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_panic_sites.tsv.gz'
 if args.max_domain:
  DEST=DEST.with_name('source_map_panic_max.tsv.gz');req=[]
  for length in [0,1,(1<<32)-2,(1<<32)-1]:
   for lo in [0,1,(1<<32)-2,(1<<32)-1]:
    for hi in [0,1,(1<<32)-2,(1<<32)-1]:
     for op in ['max_start','max_end','max_next']:req.append([str(length).encode().hex(),str(lo),str(hi),op,'0'])
   req.append([str(length).encode().hex(),'0','0','max_import','0'])
 if args.session_map:DEST=DEST.with_name('source_map_panic_session.tsv.gz');req=[['','0','0','getmap_empty','0']]
 if args.assert_id:DEST=DEST.with_name('source_map_panic_assert.tsv.gz');req=[['','0','0','assert_id','0']]
 if args.width_metadata:
  DEST=DEST.with_name('source_map_panic_width.tsv.gz');req=[]
  for src in ['','a','é','a𐀀','\ufeffa\r\n']:
   for length in [0,1,4,10]:
    for lo in [0,1]:
     for hi in [0,1,2,4,9]:
      for op in ['width_next','width_end']:req.append([src.encode().hex(),str(lo),str(hi),op,str(length)])
 if fresh:DEST=fresh[1]
 data_in=''.join(str(i)+'\t'+'\t'.join(r)+'\n' for i,r in enumerate(req)).encode();rows=[];panics=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-sites-') as tmp:
  for d in (False,True):
   for o in (False,True):
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key
    subprocess.run([profile['rustc'],*profile['link_arguments'],str(HERE/'panic_sites.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    output=Path(tmp)/(key+'.tsv')
    with output.open('wb') as stream:subprocess.run([str(binary)],input=data_in,stdout=stream,cwd=ROOT,env=env,check=True,timeout=30)
    if output.stat().st_size>producer.LIMIT:raise RuntimeError('native panic output exceeds bound')
    result=output.read_text().splitlines()
    native_profiles.append({'name':key,'reference_provenance_sha256':profile['provenance_sha256'],'rustc_version':profile['rustc_version']})
    if len(result)!=len(req):raise RuntimeError('missing native site records')
    for i,line in enumerate(result):
     ident,caught,events=line.split('\t')
     if ident!=str(i):raise RuntimeError('native request order differs')
     panics+=caught=='true';rows.append(key+'\t'+'\t'.join(req[i])+'\t'+caught+'\t'+events+'\n')
 data=''.join(rows).encode()
 if scope:
  compressed=producer.fixture(data,DEST,args.output_dir is None)
  if args.check:
   if not DEST.is_file() or DEST.read_bytes()!=compressed:raise SystemExit('SourceMap panic-site native drift; use --output-dir for a new producer')
  elif args.output_dir is not None:
   DEST.parent.mkdir(parents=True,exist_ok=True);DEST.write_bytes(compressed)
 elif args.check:
  if gzip.decompress(DEST.read_bytes())!=data:raise SystemExit('SourceMap panic-site native drift')
 elif not DEST.exists() or gzip.decompress(DEST.read_bytes())!=data:DEST.write_bytes(gzip.compress(data,mtime=0))
 manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_sha256':hashlib.sha256(reference.source_bytes('compiler/rustc_span/src/source_map.rs')).hexdigest(),'records':len(rows),'caught_panics':panics,'profiles':['d0o0','d0o1','d1o0','d1o1'],'raw_sha256':hashlib.sha256(data).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'panic_sites.rs').read_bytes()).hexdigest(),'normalization':'None. Native original/compiler/... file spelling is explicitly supplied by the Go profile; payload kind, byte message, line and column are retained.','complete':False,'remaining':'Other SourceMap assertion/allocation/arithmetic operation sites and runtime contexts remain under validation.'}
 if args.max_domain:manifest['scope']='Actual safe metadata-only imported files over full u32 lengths/positions; start/end/next points and imported-address expect failure, independent arithmetic/debug profiles; exact hook payloads and actual source locations. No giant allocations.'
 if args.width_metadata:manifest['scope']='Actual public imported SourceFile metadata lengths may differ from its hash-matching lazily loaded text; next/end points retain source arithmetic failures and exact hook locations. Original add_external_src validates the hash, not imported length.'
 if args.session_map:
  manifest['scope']='Actual public get_source_map on a fresh source thread without SessionGlobals; genuine full-crate scoped-TLS failure and original resumed payload, not a dummy SessionGlobals.'
  origins=set()
  for row in rows:
   fields=bytes.fromhex(row.rstrip('\n').split('\t')[-1]).decode().split('|')
   origins.add((fields[1],int(fields[2]),int(fields[3])))
  if len(origins)!=1:raise RuntimeError('scoped TLS origin differs across selected profiles')
  file,line,column=origins.pop();manifest['scoped_tls_location']={'File':file,'Line':line,'Column':column}
 if args.assert_id:
  manifest['scope']='Compositional actual SDK debug_assert_eq with genuine StableSourceFileId operands/Debug; source try_new_source_file assertion condition cannot fail with lawful deterministic constructors. This validates the complete failure expansion, not a public SourceMap private-branch trigger.'
  origins=set()
  for row in rows:
   value=row.rstrip('\n').split('\t')[-1]
   if value:
    fields=bytes.fromhex(value).decode().split('|');origins.add((fields[1],int(fields[2]),int(fields[3])))
  if len(origins)!=1:raise RuntimeError('assert source origin differs')
  file,line,column=origins.pop();manifest['assert_location']={'File':file,'Line':line,'Column':column}
 if args.output_dir is not None:
  if bindings!=producer.input_bindings():raise RuntimeError('panic producer inputs changed during execution')
  manifest.update({'schema':2,'artifact_kind':'source-map-panic-producer-v1','scope_id':scope,'files':bindings,'native_profiles':native_profiles,'comparison_profile':'Exact unmodified native filename; no normalization or filename substitution.'})
 path=DEST.with_name('source_map_panic_width_manifest.json' if args.width_metadata else 'source_map_panic_assert_manifest.json' if args.assert_id else 'source_map_panic_session_manifest.json' if args.session_map else 'source_map_panic_max_manifest.json' if args.max_domain else 'source_map_panic_sites_manifest.json');render=json.dumps(manifest,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise SystemExit('SourceMap site manifest drift')
 elif scope and args.output_dir is None:
  if path.read_text()!=render:raise SystemExit('historical SourceMap panic manifest differs; use --output-dir')
 else:path.write_text(render)
 if args.output_dir is not None:
  if args.check:
   if not fresh[2].is_file() or fresh[2].read_bytes()!=data:raise SystemExit('fresh SourceMap panic raw artifact differs')
  else:fresh[2].write_bytes(data)
 print(len(rows),'native site records,',panics,'caught panics')
if __name__=='__main__':main()
