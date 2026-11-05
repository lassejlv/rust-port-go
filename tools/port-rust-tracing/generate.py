#!/usr/bin/env python3
"""Compare scoped tracing used contracts with unchanged pinned dependencies."""
import argparse,gzip,hashlib,importlib.util,itertools,json,os,pathlib,subprocess,sys,tempfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
HERE=pathlib.Path(__file__).resolve().parent
REFERENCE=ROOT/'tools/port-rust-span-reference'
spec=importlib.util.spec_from_file_location('tracing_span_reference',REFERENCE/'prepare.py')
reference=importlib.util.module_from_spec(spec);spec.loader.exec_module(reference)
verified_spec=importlib.util.spec_from_file_location('tracing_verified_native_reference',ROOT/'tools/port-rust-fatal-error/reference.py')
verified=importlib.util.module_from_spec(verified_spec);verified_spec.loader.exec_module(verified)
FIXTURE=ROOT/'port/support/rusttracing/testdata/dispatch.jsonl.gz'
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env):
 with tempfile.TemporaryFile()as stdout:
  result=subprocess.run(list(map(str,args)),cwd=ROOT,env=env,stdout=stdout,stderr=subprocess.PIPE,timeout=180)
  if result.returncode:raise RuntimeError(f'command failed({result.returncode}): {args[0]}\n{result.stderr.decode(errors="replace")[-8000:]}')
  if stdout.tell()>8<<20:raise RuntimeError('native response exceeds8MiB')
  stdout.seek(0);return stdout.read()
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--prepared',action='store_true');parser.add_argument('--check',action='store_true');parser.add_argument('--profile',default='.toolchain/oracles/rustc-span-reference/build-profile.json');args=parser.parse_args()
 env=os.environ.copy();rustc=reference.executable(None,'FERRUGO_RUSTC',pathlib.Path('.toolchain/rust-nightly-2026-11-04/bin/rustc'))
 cargo=reference.executable(None,'FERRUGO_CARGO',pathlib.Path('.toolchain/rust-nightly-2026-11-04/bin/cargo'));rustdoc=reference.executable(None,'FERRUGO_RUSTDOC',pathlib.Path(rustc).parent/'rustdoc')
 selection=json.loads((REFERENCE/'toolchain.json').read_text());graph=json.loads((REFERENCE/'sources.json').read_text())
 for executable,expected in [(rustc,selection['rustc']),(cargo,selection['cargo']),(rustdoc,selection['rustc'].replace('rustc ','rustdoc ',1))]:
  if run([executable,'--version'],env).decode().strip()!=expected:raise RuntimeError('selected executable differs from exact whole-span nightly profile:'+executable)
 for name in ['FERRUGO_RUST_SOURCE','CARGO_HOME']:
  if name in env and not reference.path_option(env[name],name).is_dir():raise RuntimeError(name+' directory unavailable')
 # --profile selects arithmetic/debug flags only. Cached commands and artifact
 # paths are never executable authority, including with legacy --prepared.
 path=pathlib.Path(args.profile);path=path if path.is_absolute()else ROOT/path
 requested=json.loads(path.read_text())if path.exists()else{}
 debug=requested.get('debug_assertions',True);overflow=requested.get('overflow_checks',True)
 if type(debug)is not bool or type(overflow)is not bool:raise RuntimeError('requested profile flags must be booleans')
 profile,env,_=verified.load(debug,overflow,rustc)
 if profile['rustc_version']!=selection['rustc']or profile['source_revision']!=graph['revision']:raise RuntimeError('prepared source/compiler pin differs')
 provenance=pathlib.Path(profile['provenance_path']).read_bytes()
 if sha(provenance)!=profile['provenance_sha256']:raise RuntimeError('reference provenance changed')
 proof=json.loads(provenance)
 if proof['source_graph_sha256']!=sha((REFERENCE/'sources.json').read_bytes()):raise RuntimeError('source graph differs from committed verified whole-span graph')
 for file in graph['files']:
  base=pathlib.Path(profile['workspace'])/('original'if file['path'].startswith('compiler/')else'upstream-root');data=(base/file['path']).read_bytes()
  if len(data)!=file['bytes']or sha(data)!=file['sha256']:raise RuntimeError('compiled source differs:'+file['path'])
 output=ROOT/'.toolchain/oracles/rust-tracing';marker=output/'reference-cache.json';ownership={'tool':'port-rust-tracing','revision':graph['revision']}
 if output.exists()and any(output.iterdir())and not marker.is_file():raise RuntimeError('refusing to overwrite unowned native output directory')
 output.mkdir(parents=True,exist_ok=True)
 if marker.exists()and json.loads(marker.read_text())!=ownership:raise RuntimeError('native output ownership differs')
 marker.write_text(json.dumps(ownership,indent=2)+'\n');binary=output/'dispatch-oracle';arguments=profile['link_arguments'][:]
 for filename in profile['package_artifacts']['tracing'][0]['filenames']:arguments+=['--extern','tracing='+filename]
 env['RUSTC_BOOTSTRAP']='1';run([rustc,*arguments,HERE/'oracle.rs','-o',binary],env)
 rows=[]
 for case in itertools.product(range(3),[-1,0,1,2],range(2),range(2),range(4),range(2)):
  rows.append({'args':case,'output':run([binary,*case],env).decode()})
 raw=b''.join((json.dumps(row,ensure_ascii=False,separators=(',',':'))+'\n').encode()for row in rows)
 bound=list((ROOT/'port/support/rusttracing').glob('*.go'))+list((ROOT/'port/support/rustfmt').glob('*.go'))+list((ROOT/'port/rust/library/core/src/unicode').glob('*.go'))
 bindings={str(p.relative_to(ROOT)):sha(p.read_bytes())for p in sorted(bound)if not p.name.endswith('_test.go')or p.parent.name=='rusttracing'}
 manifest={'schema':1,'scope':'Scoped used-contract adapter only, no tracing dependency-file credit. Original register/filter/hint/callsite/typed visitor/borrowed formatting/span lifecycle and reentrancy dictionary observations. Defaults, failure/owned/abort and full source-file callers remain under validation.','source_revision':graph['revision'],'rustc':selection['rustc'],'dependencies':[p for p in proof['registry_dependencies']if p['name']in ['tracing','tracing-core','tracing-attributes']],'cases':len(rows),'uncompressed_sha256':sha(raw),'oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'go_bindings':bindings}
 metadata=(json.dumps(manifest,indent=2,ensure_ascii=False)+'\n').encode();manifest_path=HERE/'dispatch-manifest.json'
 if args.check:
  if sha(gzip.decompress(FIXTURE.read_bytes()))!=sha(raw)or manifest_path.read_bytes()!=metadata:raise RuntimeError('native tracing corpus or source bindings differ')
 else:
  FIXTURE.parent.mkdir(parents=True,exist_ok=True)
  if not FIXTURE.exists()or gzip.decompress(FIXTURE.read_bytes())!=raw:FIXTURE.write_bytes(gzip.compress(raw,mtime=0))
  manifest_path.write_bytes(metadata)
 print(f'{len(rows)} unchanged-dependency tracing states match retained corpus')
if __name__=='__main__':main()
