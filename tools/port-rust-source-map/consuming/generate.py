#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
import argparse,gzip,hashlib,json,resource,signal,subprocess,sys,tempfile
from pathlib import Path
HERE=Path(__file__).absolute().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'));import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_consuming.jsonl.gz'
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--rustc');args=p.parse_args();profile,env,_=reference.load(rustc=args.rustc);rows=[];locations={};aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-consuming-')as tmp:
  binary=Path(tmp)/'original';subprocess.run([profile['rustc'],*profile['link_arguments'],'--remap-path-prefix='+str(ROOT)+'/=',str(HERE/'oracle.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
  for mode in ['normal','inherited_caught','inherited_escape','processing']:
   for action in ['drop','take','forget','forget_panic']:
    run=subprocess.run([str(binary),mode,action],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,cwd=ROOT,env=env,timeout=10,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
    if run.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'{mode}/{action}: unexpected native status{run.returncode}')
    abort=run.returncode==-signal.SIGABRT;aborts+=abort;events=run.stdout.decode().splitlines()
    for event in events:
     if not event.startswith('hook:'):continue
     _,tag,panicking,file,line,column=event.split(':');loc={'File':file,'Line':int(line),'Column':int(column)}
     site=tag+('_processing'if tag=='outer'and mode=='processing'else'')
     if site in locations and locations[site]!=loc:raise RuntimeError('source producer location changed')
     locations[site]=loc
    rows.append({'mode':mode,'action':action,'abort':abort,'events':events})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows)
 if args.check:
  if gzip.decompress(DEST.read_bytes())!=raw:raise RuntimeError('consuming Into native drift')
 elif not DEST.exists()or gzip.decompress(DEST.read_bytes())!=raw:DEST.write_bytes(gzip.compress(raw,mtime=0))
 names=['port/rust/compiler/rustc_span/src/source_map_consuming.go','port/rust/compiler/rustc_span/src/source_map_consuming_test.go','port/rust/compiler/rustc_span/src/source_map_mapping.go']
 source=reference.source_bytes('compiler/rustc_span/src/source_map.rs')
 proof={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','source_sha256':hashlib.sha256(source).hexdigest(),'compiler':profile['rustc_version'],'source_profile':'d1o1; receiver token mutation is7+100 and introduces no flag-dependent arithmetic','records':len(rows),'actual_SIGABRT':aborts,'locations':locations,'raw_sha256':hashlib.sha256(raw).hexdigest(),'gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'wrapper_sha256':hashlib.sha256((HERE/'oracle.rs').read_bytes()).hexdigest(),'generator_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'go_bindings':{name:hashlib.sha256((ROOT/name).read_bytes()).hexdigest()for name in names},'scope':'Actual unchanged full-crate public FilePathMapping::to_real_filename accepts a legal custom Into<Cow<Path>> consuming Self. Complete actual receiver retains an owned path and Token with real Arc<AtomicUsize>. Normal destruction, whole-Self retention and later explicit Drop, forget, forget-then-panic; normal/inherited-caught/inherited-escape/processing contexts. All stdout hooks, user/field Drop, result Debug and Rc/drop counters retained. No default filename inference; client compiler remap removes only its repository prefix. Native SIGABRT vs Go exit134 is the explicit host abort representation. Existing512 borrowed Into receiver cases remain separate and unchanged.'}
 path=HERE/'VALIDATION.json';render=json.dumps(proof,indent=2)+'\n'
 if args.check:
  if path.read_text()!=render:raise RuntimeError('consuming proof metadata drift')
 else:path.write_text(render)
 print(len(rows),'full-original consuming Into cases,',aborts,'actual SIGABRT')
if __name__=='__main__':main()
