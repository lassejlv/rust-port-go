#!/usr/bin/env python3
"""Original component callbacks + genuine std hooks; no Go-derived oracle."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,resource,signal,subprocess,sys
ROOT=pathlib.Path(__file__).absolute().parents[3]
HERE=pathlib.Path(__file__).absolute().parent
REFERENCE=ROOT/'tools/port-rust-span-reference'
spec=importlib.util.spec_from_file_location('component_guard_reference',REFERENCE/'prepare.py')
reference=importlib.util.module_from_spec(spec);spec.loader.exec_module(reference)
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,env,timeout=180):
 p=subprocess.run(list(map(str,command)),cwd=ROOT,env=env,capture_output=True,timeout=timeout)
 if p.returncode:raise RuntimeError(f'command failed({p.returncode}):{command[:2]}\n{p.stderr.decode(errors="replace")[-6000:]}')
 return p.stdout
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def read_lexer_source(source,blob,revision,env,fetch):
 git_env=env.copy();git_env['GIT_NO_LAZY_FETCH']='1'
 status=subprocess.run(['git','-C',str(source),'cat-file','--batch-check'],input=(blob+'\n').encode(),env=git_env,capture_output=True,timeout=30)
 if status.returncode:raise RuntimeError('cannot inspect pinned lexer source cache')
 if status.stdout.decode().strip().endswith(' missing'):
  if not fetch:raise RuntimeError('pinned lexer blob missing; prepare with --fetch-source')
  fetch_env=env.copy();fetch_env.pop('GIT_NO_LAZY_FETCH',None)
  run(['git','-C',source,'-c','fetch.negotiationAlgorithm=noop','fetch','--no-tags','--no-write-fetch-head','--recurse-submodules=no','--filter=blob:none','origin',blob],fetch_env)
 actual=run(['git','-C',source,'rev-parse',revision+':compiler/rustc_lexer/src/lib.rs'],git_env).decode().strip()
 if actual!=blob:raise RuntimeError('lexer revision/path object differs')
 body=run(['git','-C',source,'cat-file','blob',blob],git_env)
 if hashlib.sha1(f'blob {len(body)}\0'.encode()+body).hexdigest()!=blob:raise RuntimeError('lexer object differs')
 return body
def requests():
 for component,cases in [
  ('stable_hash_context',['ok','call']),
  ('predicate',['ok','call','drop','call_drop','drop_field']),
  ('foreach',['ok','call','drop','call_drop','drop_field']),
  ('fold',['ok','call','drop','call_drop','drop_field','acc']),
  ('literal',['ok','call','drop','call_drop','drop_field']),
  ('fingerprint',['ok','call','call_b','drop','call_drop','drop_field']),
  ('binary',['ok','call','cmp','drop','key','cmp_key','call_drop','drop_field']),
  ('interval',['ok','clone','start','end','drop','call_drop','drop_field','original_drop'])]:
  for case in cases:
   for mode in ['normal','inherited_caught','inherited_escape','processing']:
    yield {'component':component,'case':case,'mode':mode}
 yield {'component':'predicate','case':'call','mode':'unobserved'}
 yield {'component':'internal','case':'ok','mode':'processing'}
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--prepared',action='store_true');p.add_argument('--fetch-source',action='store_true');p.add_argument('--fetch-deps',action='store_true');p.add_argument('--rustc');p.add_argument('--cargo');args=p.parse_args()
 env=os.environ.copy();rustc=reference.executable(args.rustc,'FERRUGO_RUSTC',pathlib.Path('.toolchain/rust-nightly-2026-11-04/bin/rustc'));cargo=reference.executable(args.cargo,'FERRUGO_CARGO',pathlib.Path('.toolchain/rust-nightly-2026-11-04/bin/cargo'))
 selection=json.loads((REFERENCE/'toolchain.json').read_text())
 if run([rustc,'--version'],env).decode().strip()!=selection['rustc']or run([cargo,'--version'],env).decode().strip()!=selection['cargo']:raise RuntimeError('component proof requires exact matching nightly tools')
 source=reference.path_option(env.get('FERRUGO_RUST_SOURCE','.toolchain/upstream-rust'),'FERRUGO_RUST_SOURCE');cargo_home=reference.path_option(env.get('CARGO_HOME','.toolchain/port-cargo-home'),'CARGO_HOME')
 if not source.is_dir()or not cargo_home.is_dir():raise RuntimeError('source/registry cache unavailable')
 env['CARGO_HOME']=str(cargo_home);env['RUSTC']=str(rustc);env['RUSTC_BOOTSTRAP']='1'
 if not args.prepared:run([sys.executable,REFERENCE/'prepare.py',*(['--fetch-source']if args.fetch_source else[])],env)
 profile=json.loads((ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json').read_text())
 graph=json.loads((REFERENCE/'sources.json').read_text());proof_data=pathlib.Path(profile['provenance_path']).read_bytes()
 if profile['rustc_version']!=selection['rustc']or profile['source_revision']!=graph['revision']or not profile['debug_assertions']or not profile['overflow_checks']or sha(proof_data)!=profile['provenance_sha256']or json.loads(proof_data)['source_graph_sha256']!=sha((REFERENCE/'sources.json').read_bytes()):raise RuntimeError('reference profile/provenance differs')
 for f in graph['files']:
  base=pathlib.Path(profile['workspace'])/('original'if f['path'].startswith('compiler/')else'upstream-root');data=(base/f['path']).read_bytes()
  if len(data)!=f['bytes']or sha(data)!=f['sha256']:raise RuntimeError('reference source bytes differ:'+f['path'])
 lexer_blob=json.loads((ROOT/'tools/port-lexer-oracle/manifest.json').read_text())['sources']['compiler/rustc_lexer/src/lib.rs']
 lexer_source=read_lexer_source(source,lexer_blob,graph['revision'],env,args.fetch_source)
 env['GIT_NO_LAZY_FETCH']='1'
 cache=ROOT/'.toolchain/oracles/panic-component-guards';marker=cache/'ownership.json';identity={'tool':'port-rust-panic-runtime/component_guards','revision':graph['revision']}
 if cache.exists()and any(cache.iterdir())and not marker.is_file():raise RuntimeError('unowned component output cache')
 cache.mkdir(parents=True,exist_ok=True)
 if marker.exists()and json.loads(marker.read_text())!=identity:raise RuntimeError('component cache identity differs')
 marker.write_text(json.dumps(identity,indent=2)+'\n');(cache/'lexer.rs').write_bytes(lexer_source)
 manifest=HERE/'dependencies/Cargo.toml';locked=(HERE/'dependencies/Cargo.lock').read_bytes();env['CARGO_TARGET_DIR']=str(cache/'target')
 if args.fetch_deps:run([cargo,'fetch','--locked','--manifest-path',manifest],env)
 metadata=json.loads(run([cargo,'metadata','--locked','--offline','--format-version=1','--manifest-path',manifest],env));registry=reference.verify_registry(metadata,locked,cargo_home)
 output=run([cargo,'build','--locked','--offline','--message-format=json','--manifest-path',manifest],env)
 artifacts={}
 for line in output.splitlines():
  row=json.loads(line)
  if row.get('reason')=='compiler-artifact'and row['target']['name']in ['memchr','unicode_ident','unicode_properties','rustc_literal_escaper']:artifacts[row['target']['name']]=row['filenames']
 if set(artifacts)!={'memchr','unicode_ident','unicode_properties','rustc_literal_escaper'}:raise RuntimeError('dependency artifact set differs')
 if registry!=reference.verify_registry(metadata,locked,cargo_home):raise RuntimeError('registry source bytes changed during build')
 generated=cache/'oracle.rs';body=(HERE/'oracle.rs').read_text();lines=body.splitlines();insert=next(i for i,line in enumerate(lines)if not line.startswith('#!['));lines.insert(insert,'#[path='+json.dumps(str(cache/'lexer.rs'))+'] mod lexer;');generated.write_text('\n'.join(lines)+'\n')
 command=[rustc,*profile['link_arguments'],'--remap-path-prefix',str(cache)+'=ferrugo-native/component_guards']
 for name,files in artifacts.items():
  for file in files:
   if file.endswith(('.rlib','.rmeta')):command+=['--extern',name+'='+file,'-L','dependency='+str(pathlib.Path(file).parent)]
 binary=cache/'oracle';run([*command,generated,'-o',binary],env)
 rows=[];locations={}
 for request in requests():
  result=subprocess.run([str(binary),*request.values()],cwd=ROOT,env=env,capture_output=True,timeout=15,preexec_fn=no_core)
  if result.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native status{result.returncode}:{request}\n{result.stderr.decode(errors="replace")[-4000:]}')
  text=result.stdout.decode()
  for line in text.splitlines():
   if not line.startswith('hook:'):continue
   fields=line.split(':',7);message=bytes.fromhex(fields[2]).decode();location=fields[7]
   if message not in ['callback','drop','field','acc','outer']:continue
   if message=='outer':message='outer_processing'if request['mode']=='processing'else'outer_unwind'
   if message in locations and locations[message]!=location:raise RuntimeError('source site changed within selected profile:'+message)
   locations[message]=location
  rows.append({'request':request,'want':{'abort':result.returncode==-signal.SIGABRT,'output':text}})
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows);fixture=HERE/'cases.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 paths=[HERE/'oracle.rs',HERE/'generate.py',HERE/'dependencies/Cargo.toml',HERE/'dependencies/Cargo.lock',REFERENCE/'sources.json',REFERENCE/'toolchain.json']
 paths+=sorted((HERE/'validation').glob('*.go'))
 production=['port/rust/compiler/rustc_lexer/src/callback_owned.go','port/rust/compiler/rustc_lexer/src/tokenizer.go','port/dependencies/rustc-literal-escaper/v0_0_8/src/callback_owned.go','port/rust/compiler/rustc_data_structures/src/fingerprint.go','port/rust/compiler/rustc_data_structures/src/stable_hash_context.go','port/rust/compiler/rustc_lexer/src/profile.go','port/rust/compiler/rustc_data_structures/src/binary_search_util/owned.go','port/rust/compiler/rustc_index/src/interval_owned.go']
 paths+=[ROOT/path for path in production]
 paths+=[ROOT/'port/support/rustfmt/formatter.go',ROOT/'port/support/rustfmt/panic.go']
 paths+=sorted((ROOT/'port').rglob('component_guard_runtime_test.go'))
 metadata={'complete':False,'scope':'Bounded integrated original component callback cleanup/runtime protocol: 182 original scripts and complete Go caller/Drop witnesses, including 107 true native aborts, ordinary/resume/cleanup hooks and current caller metadata. Source algorithms unchanged; independent isolated source/native/race acceptance passed. No dependency file credit.','rustc':selection['rustc'],'revision':graph['revision'],'reference_provenance_sha256':profile['provenance_sha256'],'lexer_git_blob':lexer_blob,'lexer_sha256':sha(lexer_source),'registry':registry,'source_locations':locations,'cases':len(rows),'aborts':sum(r['want']['abort']for r in rows),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in paths}}
 encoded=(json.dumps(metadata,indent=2)+'\n').encode();destination=HERE/'manifest.json'
 if args.check:
  if fixture.read_bytes()!=compressed or destination.read_bytes()!=encoded:raise RuntimeError('component guard native evidence differs')
 else:fixture.write_bytes(compressed);destination.write_bytes(encoded)
 print(f'{len(rows)} actual component guard scripts; {metadata["aborts"]} SIGABRT results')
if __name__=='__main__':main()
