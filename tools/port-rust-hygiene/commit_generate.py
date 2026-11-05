#!/usr/bin/env python3
"""Retain non-Copy default-method lifecycles from unchanged original hygiene."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,signal,subprocess,sys
ROOT=pathlib.Path(__file__).absolute().parents[2];HERE=pathlib.Path(__file__).absolute().parent;REFERENCE=ROOT/'tools/port-rust-span-reference'
spec=importlib.util.spec_from_file_location('hygiene_commit_reference',HERE/'reference.py');reference=importlib.util.module_from_spec(spec);spec.loader.exec_module(reference)
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,env):
 p=subprocess.run(list(map(str,command)),cwd=ROOT,env=env,capture_output=True,timeout=180)
 if p.returncode:raise RuntimeError(f'command failed({p.returncode}):{command[:2]}\n{p.stderr.decode(errors="replace")[-6000:]}')
 return p.stdout
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--prepared',action='store_true',help='compatibility alias; source verification always runs');args=p.parse_args()
 verified=reference.all_profiles();selection=json.loads((REFERENCE/'toolchain.json').read_text());graph=json.loads((REFERENCE/'sources.json').read_text())
 cache=ROOT/'.toolchain/oracles/rustc-hygiene-commit';marker=cache/'ownership.json';identity={'tool':'port-rust-hygiene/commit','revision':graph['revision']}
 if cache.exists()and any(cache.iterdir())and not marker.is_file():raise RuntimeError('unowned commit cache')
 cache.mkdir(parents=True,exist_ok=True)
 if marker.exists()and json.loads(marker.read_text())!=identity:raise RuntimeError('commit cache identity differs')
 marker.write_text(json.dumps(identity,indent=2)+'\n')
 for name in ['commit.rs','commit_mir.rs']:(cache/name).write_bytes((HERE/name).read_bytes())
 rows=[];locations={};profiles={};mir=None;mir_output=None
 for debug,overflow,profile,env,_ in verified:
   rustc=profile['rustc'];key=f'd{int(debug)}o{int(overflow)}';provenance=pathlib.Path(profile['provenance_path']).read_bytes()
   if profile['rustc_version']!=selection['rustc']or profile['source_revision']!=graph['revision']or profile['debug_assertions']!=debug or profile['overflow_checks']!=overflow or sha(provenance)!=profile['provenance_sha256']or json.loads(provenance)['source_graph_sha256']!=sha((REFERENCE/'sources.json').read_bytes()):raise RuntimeError('commit source profile differs:'+key)
   profiles[key]=profile['provenance_sha256']
   for f in graph['files']:
    base=pathlib.Path(profile['workspace'])/('original'if f['path'].startswith('compiler/')else'upstream-root');data=(base/f['path']).read_bytes()
    if len(data)!=f['bytes']or sha(data)!=f['sha256']:raise RuntimeError('original source changed:'+f['path'])
   remap=['--remap-path-prefix',str(cache)+'=ferrugo-native/hygiene-commit'];binary=cache/('commit-'+key)
   run([rustc,*profile['link_arguments'],*remap,cache/'commit.rs','-o',binary],env)
   for operation in ['empty_primary','repeat_primary','empty_ref','repeat_ref']:
    for case in ['ok','len','slot','from','old_drop','old_field','old_drop_field','new_drop','new_field','new_drop_field']:
     for mode in ['normal','inherited_caught','inherited_escape','processing','unobserved']:
      result=subprocess.run([str(binary),operation,case,mode],cwd=ROOT,env=env,capture_output=True,timeout=15,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
      if result.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native commit status{result.returncode}:{operation}/{case}/{mode}\n{result.stderr.decode(errors="replace")[-4000:]}')
      output=result.stdout.decode()
      for line in output.splitlines():
       if not line.startswith('hook:'):continue
       fields=line.split(':',8);message=bytes.fromhex(fields[2]).decode();site='rustc_span::hygiene::HygieneRemapper.commit_end_of_determinism.repeat'if message=='this function can be called only once'else message
       if site in locations and locations[site]!=fields[8]:raise RuntimeError('commit source location differs:'+site)
       locations[site]=fields[8]
      rows.append({'request':{'operation':operation,'case':case,'mode':mode,'debug':debug,'overflow':overflow},'want':{'abort':result.returncode==-signal.SIGABRT,'output':output}})
   if debug and overflow:
    dump=cache/'mir';dump.mkdir(exist_ok=True);mir_binary=cache/'commit-mir'
    flags=['-Zmir-opt-level=4','-Zinline-mir=yes','-Zinline-mir-threshold=100000','-Zinline-mir-hint-threshold=100000','-Zcross-crate-inline-threshold=always','-Zdump-mir=all','-Zdump-mir-exclude-pass-number','-Zdump-mir-dir='+str(dump)]
    run([rustc,*profile['link_arguments'],*remap,*flags,cache/'commit_mir.rs','-o',mir_binary],env)
    mir=(dump/'commit_mir.main-{closure#1}.Inline.after.mir').read_bytes();mir_output=run([mir_binary],env)
    for witness in [b'inlined <Remapper as HygieneRemapper<Key>>::commit_end_of_determinism',b'Option::<Key>::Some',b'drop(_7)',b'unwind terminate(cleanup)']:
     if witness not in mir:raise RuntimeError('actual original MIR witness absent:'+witness.decode())
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);compressed=gzip.compress(raw,mtime=0);fixture=ROOT/'port/rust/compiler/rustc_span/src/testdata/hygiene_commit.jsonl.gz'
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 paths=[HERE/name for name in ['commit.rs','commit_mir.rs','commit_generate.py']]+[REFERENCE/'sources.json',REFERENCE/'toolchain.json']+sorted((ROOT/'port/rust/compiler/rustc_span/src').glob('hygiene*.go'))+[ROOT/name for name in ['port/support/rustfmt/panic.go','port/support/rustfmt/formatter.go','port/rust/compiler/rustc_span/src/lib_shared.go']]+[ROOT/name for name in reference.tool_inputs()]
 metadata={'complete':False,'scope':'Original HygieneRemapper<T>::commit_end_of_determinism permits non-Copy T:From<u32>: real owned Arc/Vec fields, actual old Option Drop and field glue, slot installation before assertion, len/slot/From failures, old/new Drop and second field failures, all four assertion/overflow profiles and current/inherited/processing/runtime0 contexts. Both complete value-ops primary and borrowed remapper type-dictionary dispatch replay the same unchanged-source callers. Independent full-file acceptance remains pending.','source_revision':graph['revision'],'rustc':selection['rustc'],'reference_profile_provenance_sha256':profiles,'source_locations':locations,'cases':len(rows),'aborts':sum(r['want']['abort']for r in rows),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'mir_sha256':sha(mir),'mir_stdout_sha256':sha(mir_output),'files':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in paths}}
 outputs={fixture:compressed,HERE/'commit-manifest.json':(json.dumps(metadata,indent=2)+'\n').encode(),HERE/'commit-mir.txt':mir,HERE/'commit-mir.stdout':mir_output}
 for path,data in outputs.items():
  if args.check:
   if path.read_bytes()!=data:raise RuntimeError('commit source proof differs:'+str(path))
  else:path.write_bytes(data)
 print(f'{len(rows)} whole-source non-Copy commits; {metadata["aborts"]} native aborts; original imported MIR retained')
if __name__=='__main__':main()
