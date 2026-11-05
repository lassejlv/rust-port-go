#!/usr/bin/env python3
"""Retain actual unchanged whole-span hygiene comparisons; no Rust fallback."""
import argparse,gzip,hashlib,importlib.util,itertools,io,json,os,pathlib,subprocess,sys,tempfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
HERE=pathlib.Path(__file__).resolve().parent
REFERENCE=ROOT/'tools/port-rust-span-reference'
spec=importlib.util.spec_from_file_location('hygiene_span_reference',HERE/'reference.py')
reference=importlib.util.module_from_spec(spec);spec.loader.exec_module(reference)
def sha(data):return hashlib.sha256(data).hexdigest()
def retain_trace_metadata(output,sites):
 for line in output.splitlines():
  fields=line.split('|')
  if len(fields)!=10 or fields[0]not in ['register','enabled','event','event_enabled']or fields[3]!='rustc_span::hygiene':continue
  metadata={'Name':fields[1],'Level':{'OFF':0,'ERROR':1,'WARN':2,'INFO':3,'DEBUG':4,'TRACE':5}[fields[2]],'Target':fields[3],'ModulePath':fields[4],'File':fields[5],'Line':int(fields[6]),'Kind':0 if fields[7]=='span'else 1,'Fields':fields[8].split(',')}
  key=str(metadata['Line'])
  if key in sites and sites[key]!=metadata:raise RuntimeError('verbatim tracing callsite differs:'+key)
  sites[key]=metadata
def run(args,env):
 with tempfile.TemporaryFile()as stdout:
  p=subprocess.run(list(map(str,args)),cwd=ROOT,env=env,stdout=stdout,stderr=subprocess.PIPE,timeout=180)
  if p.returncode:raise RuntimeError(f'command failed ({p.returncode}): {args}\n{p.stderr.decode(errors="replace")[-8000:]}')
  if stdout.tell()>8<<20:raise RuntimeError('bounded native response exceeds8MiB: '+str(args))
  stdout.seek(0);return stdout.read()
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--prepared',action='store_true',help='compatibility alias; source verification always runs');parser.add_argument('--check',action='store_true');parser.add_argument('--suite',choices=['all','core','graph','remap','queue','decode','format','codec','tracing','hash','invalid_codec','order'],default='all');parser.add_argument('--profile',help='optional checked-profile assertion; cached commands are never executed');args=parser.parse_args()
 profile,env,_=reference.select_main_profile(args.profile);rustc=profile['rustc']
 selection=json.loads((REFERENCE/'toolchain.json').read_text())
 if profile['rustc_version']!=selection['rustc']or profile['source_revision']!=json.loads((REFERENCE/'sources.json').read_text())['revision']:raise RuntimeError('reference profile/compiler pin differs')
 if not profile['debug_assertions']or not profile['overflow_checks']:raise RuntimeError('core corpus requires independently checked/default assertion profile')
 provenance=pathlib.Path(profile['provenance_path']).read_bytes()
 if sha(provenance)!=profile['provenance_sha256']:raise RuntimeError('reference provenance changed')
 proof=json.loads(provenance)
 graph=json.loads((REFERENCE/'sources.json').read_text())
 if proof['source_graph_sha256']!=sha((REFERENCE/'sources.json').read_bytes()):raise RuntimeError('reference source graph differs')
 for file in graph['files']:
  base=pathlib.Path(profile['workspace'])/('original'if file['path'].startswith('compiler/')else'upstream-root')
  data=(base/file['path']).read_bytes()
  if len(data)!=file['bytes']or sha(data)!=file['sha256']:raise RuntimeError('compiled source bytes differ:'+file['path'])
 output=ROOT/'.toolchain/oracles/rustc-hygiene';marker=output/'reference-cache.json';ownership={'tool':'port-rust-hygiene','revision':graph['revision']}
 if output.exists()and any(output.iterdir())and not marker.is_file():raise RuntimeError('refusing to overwrite unowned native output directory')
 output.mkdir(parents=True,exist_ok=True)
 if marker.exists()and json.loads(marker.read_text())!=ownership:raise RuntimeError('native cache ownership differs')
 marker.write_text(json.dumps(ownership,indent=2)+'\n');env['RUSTC_BOOTSTRAP']='1'
 bindings={str(p.relative_to(ROOT)):sha(p.read_bytes())for p in sorted((ROOT/'port/rust/compiler/rustc_span/src').glob('hygiene*.go'))}
 suites=['core','graph','remap','queue','decode','format','codec','tracing','hash','invalid_codec','order']if args.suite=='all'else[args.suite]
 for suite in suites:
  source=HERE/(suite+'.rs');binary=output/suite;link=list(profile['link_arguments'])
  if suite=='tracing':
   for f in profile['package_artifacts']['tracing'][0]['filenames']:link+=['--extern','tracing='+f]
  run([rustc,*link,source,'-o',binary],env)
  if suite=='tracing':requests=[{'n':n,'mode':mode}for n in range(4)for mode in range(96)]
  elif suite=='remap':requests=[{'old':old,'extra':extra,'mode':mode}for old,extra,mode in itertools.product([0,1,4,10],[0,1,2,3,8,32],range(8))]
  else:requests=[{'n':n,'mode':mode}for n in ([0,1,2,3,4,8,12,16]if suite=='core'else range(6))for mode in range(8)]
  def rows():
   if suite=='hash':
    for debug in [False,True]:
     selected,selected_env,_=reference.load(debug,True);selected_rustc=selected['rustc']
     if selected['rustc_version']!=selection['rustc']or selected['source_revision']!=graph['revision']or selected['debug_assertions']!=debug or not selected['overflow_checks']:raise RuntimeError('hash assertion profile differs')
     provenance=pathlib.Path(selected['provenance_path']).read_bytes()
     if sha(provenance)!=selected['provenance_sha256']or json.loads(provenance)['source_graph_sha256']!=proof['source_graph_sha256']:raise RuntimeError('hash reference provenance differs')
     chosen=output/('hash-d'+str(int(debug)));run([selected_rustc,*selected['link_arguments'],source,'-o',chosen],selected_env)
     for n in range(int(run([chosen,'--count'],selected_env))):
      for line in run([chosen,n],selected_env).splitlines():
       raw_n,raw_mode,data=line.split(b'\t')
       if int(raw_n)!=n:raise RuntimeError('hash case identity differs')
       yield {'n':n,'mode':int(raw_mode),'extra':int(debug),'output':bytes.fromhex(data.decode()).decode()}
   elif suite in ['format','codec','invalid_codec','order']:
    for n in range(int(run([binary,'--count'],env))):
     for style in (range(32)if suite=='format'else[None]):
      command=[binary,n,style]if suite=='format'else[binary,n]
      for line in run(command,env).splitlines():
       raw_n,raw_mode,payload=line.split(b'\t')
       if int(raw_n)!=n:raise RuntimeError('format case identity differs')
       yield {'n':n,'mode':int(raw_mode),'output':bytes.fromhex(payload.decode()).decode()}
   else:
    for request in requests:yield {**request,'output':run([binary,*request.values()],env).decode()}
  count=0;digest=hashlib.sha256();compressed=io.BytesIO();trace_metadata={}
  with gzip.GzipFile(fileobj=compressed,mode='wb',filename='',mtime=0)as fixture:
   for row in rows():
    if suite=='tracing':retain_trace_metadata(row['output'],trace_metadata)
    raw=(json.dumps(row,ensure_ascii=False,separators=(',',':'))+'\n').encode();digest.update(raw);fixture.write(raw);count+=1
  encoded=compressed.getvalue();content_sha=digest.hexdigest()
  scope=('Allocation/context chains, all transparencies, outside-lock dollar-crate updates, stable expansion hashes and complete compact/pretty HygieneData states.'if suite=='core'else 'Graph public API: expansion lookup/causes/ancestry, external-macro kind branches, mark removal, adjustment/normalization, glob/reverse-glob partial mutation, hygienic equality, walk/collapsed chains and final complete state.')
  if suite=='invalid_codec':scope='Malformed safe Decoder values: every enum tag domain, Option discriminants, 29-bit index bounds, foreign LocalExpnId rejection, huge TrustedLen collection sizes before symbol callbacks, arbitrary private disambiguator values, bool defaults/overrides and exact caught payload/read state.'
  if suite=='order':scope='Inherited PartialOrd predicates on all three Transparency variants and inherited Ord predicates/min/max/clamp on all MacroKind variants and boundary/interior ExpnIndex values; every bound-order combination and exact caught clamp payload; StepsBetween directions and valid checked/overflowing/default/unsafe Step methods through the complete concrete Step dictionary.'
  if suite=='hash':scope='Every concrete public hygiene Hash implementation and inherited hash_slice default; raw required-only/typed Hasher callbacks, all native write failure positions, empty/repeated slices, preserved borrowed state, direct and concrete-H dictionaries, exact/reference/wrapped Unhasher under independent assertion profiles.'
  if suite=='tracing':scope='Every concrete original hygiene tracing callsite: true tracing subscriber interest/enable/max filtering, exact metadata and message fields, borrowed mutable formatting, all graph/encode/decode/disambiguator sites, writer C/S/error behavior and RefCell guard lifetime during round-event dispatch; each script has an independent process registry.'
  if suite=='codec':scope='Every original public hygiene codec and enum variant, actual user SpanEncoder/SpanDecoder dictionaries, independent default/overridden bool operations, all emit/read failure positions, raw fingerprint bytes and decoded owned Arc state.'
  if suite=='format':scope='All public hygiene custom/derived Debug implementations and every variant; 73 values, sixteen Formatter options, independent C/S sinks, every native write failure, same mutable formatter reuse, post-error fill/alignment state and actual pinned Unicode18 symbol/span delegates.'
  if suite=='decode':scope='Whole-source syntax and expansion decode helpers, real MemDecoder positions, root/cache fast paths consuming unused closures, recursive same-cache callbacks, borrowed decoder identity, FnOnce cleanup, invalid-parent partial allocations and caught-failure retry.'
  if suite=='queue':scope='Complete encode queue state machine, deduplication, reentrant scheduling across rounds, borrowed/owned FnMut cleanup order, caught callback failure and resumed partial mem::take queues, local Arc clone counts and foreign data omission.'
  if suite=='remap':scope='Complete public HygieneRemapper required/default/override dictionary, two generic outputs borrowing one non-comparable state, all conversion callbacks, zero/one-key sorting shortcut, stable collisions and repeated-commit/hash-panic state.'
  support={str(p.relative_to(ROOT)):sha(p.read_bytes())for p in sorted((ROOT/'port/support/rustsort').glob('*.go'))if not p.name.endswith('_test.go')}
  if suite=='tracing':
   support.update({str(p.relative_to(ROOT)):sha(p.read_bytes())for p in sorted((ROOT/'port/support/rusttracing').glob('*.go'))if not p.name.endswith('_test.go')})
   support.update({str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [ROOT/'port/support/rustfmt/formatter.go',ROOT/'port/support/rustfmt/panic.go']})
  oracle_support=reference.tool_inputs()
  if suite=='invalid_codec':oracle_support[str((HERE/'codec.rs').relative_to(ROOT))]=sha((HERE/'codec.rs').read_bytes())
  manifest={'schema':1,'scope':scope+' Full-file credit remains pending complete owned/codec/tracing/panic evidence.','source_revision':profile['source_revision'],'rustc':selection['rustc'],'source_graph_sha256':proof['source_graph_sha256'],'cases':count,'uncompressed_sha256':content_sha,'oracle_sha256':sha(source.read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'go_bindings':bindings,'support_bindings':support,'oracle_support_bindings':oracle_support}
  if suite=='tracing':
   if len(trace_metadata)!=16:raise RuntimeError('expected all sixteen verbatim original hygiene callsites')
   manifest['metadata_profile']={'name':'unchanged-original-reference-'+profile['source_revision'],'sites':trace_metadata}
  metadata=(json.dumps(manifest,indent=2,ensure_ascii=False)+'\n').encode();manifest_path=HERE/(suite+'-manifest.json');fixture=ROOT/('port/rust/compiler/rustc_span/src/testdata/hygiene_'+suite+'.jsonl.gz')
  if args.check:
   if sha(gzip.decompress(fixture.read_bytes()))!=content_sha or manifest_path.read_bytes()!=metadata:raise RuntimeError(suite+' corpus or source bindings differ')
  else:
   fixture.parent.mkdir(parents=True,exist_ok=True)
   if not fixture.exists()or sha(gzip.decompress(fixture.read_bytes()))!=content_sha:fixture.write_bytes(encoded)
   manifest_path.write_bytes(metadata)
  print(f'{count} whole-source hygiene {suite} states match retained corpus')
if __name__=='__main__':main()
