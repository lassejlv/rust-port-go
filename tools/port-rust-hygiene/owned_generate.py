#!/usr/bin/env python3
"""Whole-source owned hygiene argument/closure lifecycle observations."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,signal,subprocess,sys
ROOT=pathlib.Path(__file__).absolute().parents[2];HERE=pathlib.Path(__file__).absolute().parent
REFERENCE=ROOT/'tools/port-rust-span-reference'
spec=importlib.util.spec_from_file_location('owned_hygiene_reference',HERE/'reference.py');reference=importlib.util.module_from_spec(spec);spec.loader.exec_module(reference)
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,env):
 p=subprocess.run(list(map(str,command)),cwd=ROOT,env=env,capture_output=True,timeout=180)
 if p.returncode:raise RuntimeError(f'command failed({p.returncode}):{command[:2]}\n{p.stderr.decode(errors="replace")[-6000:]}')
 return p.stdout
def requests(debug,overflow):
 extra=[('register_'+name,-1)for name in ['local_duplicate','foreign_parent','foreign_hash','foreign_id']]+[('decode_'+name+'_owned',0 if name in ['root','cached']or name=='local'and debug else 1)for name in ['root','cached','new','local','parent']]+[('lookup_'+name,-1)for name in ['local_empty','expn_empty','outer_empty','debug_empty','local_missing','expn_missing','hash_local_missing','foreign_data_missing','foreign_hash_missing','expect_foreign']]+[('context_'+name,-1)for name in ['outer','outer_data','edition','marks','remove','normalize','normalize_rules','adjust','normalize_adjust','glob','reverse','equal','apply']]+[('set_missing',5)]
 extra += [('collision_forced',7 if debug else 8),('fresh_parent_missing',2),('set_parent_missing',2),('context_apply_parent',-1),('context_alloc_self',-1),('context_dollar',-1)]
 extra += [(f'index_{kind}_{action}',-1)for kind in ['syntax','expn','local','decoded']for action in ['u32','usize','add_bound','add_overflow','assign_bound','assign_overflow','plus_bound','plus_overflow','increment_bound','increment_overflow']]+[('index_step_'+action,-1)for action in ['forward_bound','forward_none','backward_none','forward_overflow','backward_overflow','forward','backward']]
 extra += [('syntax_'+name+'_owned',1 if name in ['new','parent']else 0)for name in ['root','cached','new','parent','bound','reader']]+[('encode_'+name,2 if name in ['owned','borrowed']else 0)for name in ['owned','borrowed','empty_owned','bad_context_owned','bad_expn_owned','empty_expn_owned','foreign_missing_owned']]
 extra += [(f'codec_{n}',-1)for n in range(64)]
 extra += [(f'raw_hash_{n}',-1)for n in [0,15,16,17]]
 extra += [('debug_verbose',1),('debug_verbose_error',1),('already_set_debug',1),('already_set_debug_error',1),('encode_locked_owned',0),('encode_shared_owned',0)]
 extra += [('scope_missing',0)]
 extra += [('clamp_index',-1),('clamp_macro',-1)]
 extra += [('fresh_hash_duplicate',5),('set_hash_duplicate',5)]
 for operation,calls in [('fresh',5),('set',5),('mark',5),('collision',8),('reset',8),('bad_parent',0 if debug else 5),('bad_parent_set',0 if debug else 5),('already_set',0),('update',4),('borrowed_update',4),('comparator',0),*[(f'remap_{kind}{suffix}',1)for kind in ['syntax','local']for suffix in ['','_before','_repeat','_value']],*extra]:
  cases=['ok']if calls<0 else ['ok','reenter','drop','field','drop_field','reenter_drop']+[f'call{n}{suffix}'for n in range(1,calls+1)for suffix in ['','_drop','_field']]
  for case in cases:
   for mode in ['normal','inherited_caught','inherited_escape','processing','unobserved']:
    yield {'operation':operation,'case':case,'mode':mode,'debug':debug,'overflow':overflow}
def evidence_paths(output_dir=None):
 if output_dir is None:return HERE/'owned-manifest.json',ROOT/'port/rust/compiler/rustc_span/src/testdata/hygiene_owned.jsonl.gz',None
 if not output_dir:raise RuntimeError('owned output directory explicitly empty')
 directory=pathlib.Path(output_dir).absolute()
 if directory.is_symlink()or directory.exists()and not directory.is_dir():raise RuntimeError('invalid owned output directory')
 return directory/'owned-manifest.json',directory/'hygiene_owned.jsonl.gz',directory/'hygiene_owned.jsonl'
def compressed_fixture(raw,fixture,historical):
 if fixture.is_symlink():raise RuntimeError('refusing owned evidence symlink:'+str(fixture))
 if fixture.exists():
  previous=fixture.read_bytes()
  if gzip.decompress(previous)==raw:return previous
  if historical:raise RuntimeError('historical owned producer filenames/observations differ; use --output-dir for this producer')
 return gzip.compress(raw,mtime=0)
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--prepared',action='store_true',help='compatibility alias; source verification always runs');p.add_argument('--rustc');p.add_argument('--output-dir',help='fresh environment-specific producer manifest, gzip and raw JSONL; historical evidence remains the default');args=p.parse_args()
 manifest,fixture,raw_artifact=evidence_paths(args.output_dir)
 verified=reference.all_profiles(args.rustc);selection=json.loads((REFERENCE/'toolchain.json').read_text());graph=json.loads((REFERENCE/'sources.json').read_text())
 cache=ROOT/'.toolchain/oracles/rustc-hygiene-owned';marker=cache/'ownership.json';identity={'tool':'port-rust-hygiene/owned','revision':graph['revision']}
 if cache.exists()and any(cache.iterdir())and not marker.is_file():raise RuntimeError('unowned owned-hygiene cache')
 cache.mkdir(parents=True,exist_ok=True)
 if marker.exists()and json.loads(marker.read_text())!=identity:raise RuntimeError('owned hygiene cache differs')
 marker.write_text(json.dumps(identity,indent=2)+'\n');driver=cache/'owned.rs';driver.write_bytes((HERE/'owned.rs').read_bytes())
 (cache/'failures.rs').write_bytes((HERE/'failures.rs').read_bytes())
 (cache/'index_failures.rs').write_bytes((HERE/'index_failures.rs').read_bytes())
 for name in ['pipeline.rs','codec.rs','invalid_codec.rs','debug_panic.rs']:(cache/name).write_bytes((HERE/name).read_bytes())
 rows=[];locations={};profiles={}
 for debug,overflow,selected,env,_ in verified:
   rustc=selected['rustc'];key=f'd{int(debug)}o{int(overflow)}';provenance=pathlib.Path(selected['provenance_path']).read_bytes()
   if selected['rustc_version']!=selection['rustc']or selected['source_revision']!=graph['revision']or selected['debug_assertions']!=debug or selected['overflow_checks']!=overflow or sha(provenance)!=selected['provenance_sha256']or json.loads(provenance)['source_graph_sha256']!=sha((REFERENCE/'sources.json').read_bytes()):raise RuntimeError('owned source profile differs:'+key)
   profiles[key]=selected['provenance_sha256'];binary=cache/('owned-'+key);run([rustc,*selected['link_arguments'],'--remap-path-prefix',str(cache)+'=ferrugo-native/hygiene-owned',driver,'-o',binary],env)
   for request in requests(debug,overflow):
    result=subprocess.run([str(binary),request['operation'],request['case'],request['mode']],cwd=ROOT,env=env,capture_output=True,timeout=15,preexec_fn=lambda:resource.setrlimit(resource.RLIMIT_CORE,(0,0)))
    if result.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native status{result.returncode}:{request}\n{result.stderr.decode(errors="replace")[-6000:]}')
    output=result.stdout.decode()
    for line in output.splitlines():
     if not line.startswith('hook:'):continue
     fields=line.split(':',8);message=bytes.fromhex(fields[2]).decode();site=message
     if message=='outer':site='outer_processing'if request['mode']=='processing'else'outer_unwind'
     elif message.startswith('assertion `left == right` failed')and request['operation']in ['bad_parent','bad_parent_set']:site='rustc_span::hygiene::'+('fresh'if request['operation']=='bad_parent'else'set_expn_data')+'.parent_local'
     elif 'Already set disambiguator' in message:site='rustc_span::hygiene::update_disambiguator.already_set'
     elif message=='expansion data is reset for an expansion ID':site='rustc_span::hygiene::set_expn_data.reset'
     elif message=='this function can be called only once':site='rustc_span::hygiene::HygieneRemapper.commit_end_of_determinism.repeat'
     elif message=='must be set at this point':site='rustc_span::hygiene::'+('SyntaxContextRemapper'if request['operation'].startswith('remap_syntax')else'LocalExpnIdRemapper')+'.first_non_det_index.unset'
     elif message=='lock was already held':site='rustc_data_structures::sync::Lock::lock.reentry'
     elif request['operation']=='lookup_expect_foreign'and message=='called `Option::unwrap()` on a `None` value':site='rustc_span::hygiene::ExpnId::expect_local'
     elif message=='overflow in `Step::forward`':site='rustc_span::hygiene::ExpnIndex::Step::forward'
     elif message=='overflow in `Step::backward`':site='rustc_span::hygiene::ExpnIndex::Step::backward'
     elif message=='decoder':site='owned_decoder'
     elif message=='capacity overflow':site='rustc_span::hygiene::codec.capacity_overflow'
     elif message=='assertion failed: min <= max':site='rustc_span::hygiene::Ord::clamp'
     elif message=='a formatting trait implementation returned an error when the underlying stream did not: Error':site='rustc_span::hygiene::format_error'
     elif message=='called `Result::unwrap()` on an `Err` value: TryFromSliceError(())':site='rustc_data_structures::Fingerprint::decode.raw_length'
     elif message=='cannot access a scoped thread local variable without calling `set` first':site='rustc_span::scoped_tls::with'
     elif fields[8].startswith('original/compiler/'):site='source:'+fields[8].removeprefix('original/compiler/')
     elif message not in ['callback','drop','field']:continue
     if site in locations and locations[site]!=fields[8]:raise RuntimeError('selected source location differs:'+site)
     locations[site]=fields[8]
    rows.append({'request':request,'want':{'abort':result.returncode==-signal.SIGABRT,'output':output}})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);compressed=compressed_fixture(raw,fixture,args.output_dir is None)
 paths=[HERE/name for name in ['owned.rs','failures.rs','index_failures.rs','pipeline.rs','codec.rs','invalid_codec.rs','debug_panic.rs','owned_generate.py']]+[REFERENCE/'sources.json',REFERENCE/'toolchain.json'];paths+=sorted((ROOT/'port/rust/compiler/rustc_span/src').glob('hygiene*.go'))
 paths += [ROOT/name for name in ['port/support/rustfmt/panic.go','port/support/rustfmt/formatter.go','port/support/rustvec/ownership.go','port/rust/compiler/rustc_data_structures/src/stable_hash_context.go','port/rust/compiler/rustc_span/src/lib_shared.go','port/rust/compiler/rustc_span/src/lib_sync.go','port/rust/compiler/rustc_span/src/lib_session.go']]
 paths += [ROOT/'port/support/rusthashmap/once.go']+[p for p in sorted((ROOT/'port/support/rustsort').glob('*.go'))if not p.name.endswith('_test.go')]
 paths += [ROOT/'port/rust/compiler/rustc_data_structures/src/fingerprint.go']
 paths += [ROOT/name for name in reference.tool_inputs()]
 metadata={'complete':False,'scope':'Consumed StableHashCtxt/context/FnMut/FnOnce lifecycle in fresh/set/mark/collision/reset/parent/name-update/comparator/remapper under all four assertion/overflow profiles: every observed callback failure position, callback/Drop/field cleanup and abort boundaries, actual Arc counts, borrowed/owned closure phase, held-Lock reentry, generic return-place leaks, exact current Context and final full HygieneData state. Full file validation remains in progress.','source_revision':graph['revision'],'rustc':selection['rustc'],'reference_profile_provenance_sha256':profiles,'source_locations':locations,'cases':len(rows),'aborts':sum(row['want']['abort']for row in rows),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in paths}}
 metadata['artifact_kind']='hygiene-owned-producer-profile-v1'
 metadata['location_profile']={'name':'verbatim-producer-environment','environment_specific':True,'observer_normalization':'none; all native hook filenames including scoped-tls registry paths are retained verbatim'}
 encoded=(json.dumps(metadata,indent=2)+'\n').encode();outputs={fixture:compressed,manifest:encoded}
 if raw_artifact is not None:outputs[raw_artifact]=raw
 for path,data in outputs.items():
  if path.is_symlink():raise RuntimeError('refusing owned evidence symlink:'+str(path))
  if args.check:
   if path.read_bytes()!=data:raise RuntimeError('owned hygiene source proof differs:'+str(path))
  else:path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
 print(f'{len(rows)} whole-source owned hygiene lifecycles; {metadata["aborts"]} native aborts')
if __name__=='__main__':main()
