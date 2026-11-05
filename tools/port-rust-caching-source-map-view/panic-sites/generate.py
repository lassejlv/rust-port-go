#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
HERE=Path(__file__).absolute().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/caching_source_map_view_panic.jsonl.gz'
SITES={34:'new.first_file_index',46:'pos_to_line.lookup_unwrap',48:'pos_to_line.line_number_add',77:'byte_pos_to_line_and_col.column_subtract',132:'span_data.lo_lower_bound',133:'span_data.lo_upper_bound',134:'span_data.hi_lower_bound',135:'span_data.hi_upper_bound',136:'span_data.file_contains_lo',137:'span_data.file_contains_hi',151:'file_for_position.file_index'}
CASES=['empty','unwrap_byte','unwrap_span','lo_upper','hi_upper','stale_hi_lower','stale_contains_lo','stale_hit','ok']
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();rows=[];locations={};aborts=0;hook_count=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-caching-panic-')as tmp:
  for d in [False,True]:
   for o in [False,True]:
    key=f'd{int(d)}o{int(o)}';profile,env,_=reference.load(d,o,args.rustc);binary=Path(tmp)/key
    subprocess.run([profile['rustc'],*profile['link_arguments'],'--remap-path-prefix='+str(ROOT)+'/=',str(HERE/'oracle.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
    for mode in ['normal','inherited','processing']:
     for case in CASES:
      for caller in ['A','B','nil']:
       run=subprocess.run([str(binary),mode,case,caller],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
       if run.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'{key}/{mode}/{case}/{caller}: unexpected status{run.returncode}')
       abort=run.returncode==-signal.SIGABRT;aborts+=abort;events=run.stdout.decode().splitlines()
       for event in events:
        if not event.startswith('H:'):continue
        _,label,kind,payload,file,line,column,panicking,can_unwind,force,counts=event.split(':');file=bytes.fromhex(file).decode();line=int(line);column=int(column);hook_count+=1
        if file.endswith('/caching_source_map_view.rs'):site='rustc_span::caching_source_map_view::'+SITES[line]
        elif file.endswith('/lib.rs'):site=f'rustc_span::lib.rs:{line}:{column}'
        elif bytes.fromhex(payload)==b'outer':site='outer'
        else:raise RuntimeError(f'unclassified original panic site{file}:{line}:{column}')
        value={'File':file,'Line':line,'Column':column}
        if site in locations and locations[site]!=value:raise RuntimeError('selected source location changes between profiles')
        locations[site]=value
       rows.append({'profile':key,'mode':mode,'case':case,'caller':caller,'abort':abort,'events':events})
 raw=b''.join((json.dumps(r,separators=(',',':'))+'\n').encode()for r in rows)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=raw:raise RuntimeError('whole-source caching panic drift')
 elif not DEST.exists()or gzip.decompress(DEST.read_bytes())!=raw:DEST.write_bytes(gzip.compress(raw,mtime=0))
 source=reference.source_bytes('compiler/rustc_span/src/caching_source_map_view.rs')
 bound=[ROOT/'port/rust/compiler/rustc_span/src'/name for name in ['caching_source_map_view.go','caching_source_map_view_panic.go','caching_source_map_view_panic_test.go']]
 proof={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_lines':len(source.splitlines()),'source_bytes':len(source),'source_sha256':hashlib.sha256(source).hexdigest(),'source_git_blob':hashlib.sha1(b'blob '+str(len(source)).encode()+b'\0'+source).hexdigest(),'records':len(rows),'actual_SIGABRT':aborts,'hook_records':hook_count,'locations':locations,'raw_sha256':hashlib.sha256(raw).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'oracle.rs').read_bytes()).hexdigest(),'generator_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'go_bindings':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in bound},'scope':'Entire unchanged rustc_span CachingSourceMapView through verified complete reference. Safe imported metadata, real source files/line locks/Arc counts, ordinary/inherited-caught/actual panic-hook processing, construction caller A then operation A/B/nil observer. All hook fields, payload kind/bytes/verbatim locations/can_unwind/force_no_backtrace, local Drop, state and owner counts retained. Native SIGABRT vs Go exit134 is the explicit host abort class. nil deliberately removes the operation observer and runtime metadata after construction under A; its SourceRuntime processing operation is still explicitly driven by the native panic hook. Construction/priming A observer events remain retained. No private cache/source-map mutation or replacement algorithm.'}
 path=HERE/'VALIDATION.json';render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('caching panic manifest drift')
 else:path.write_text(render)
 print(len(rows),'whole-source caching panic records,',aborts,'actual SIGABRT,',hook_count,'hook records')
if __name__=='__main__':main()
