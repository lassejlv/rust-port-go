#!/usr/bin/env python3
"""Use the real full compiler reference graph, never a reconstructed Unord."""
import argparse,gzip,hashlib,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-prepare',action='store_true');a=p.parse_args()
 profile_path=pathlib.Path(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
 if not str(profile_path)or not profile_path.is_file():raise SystemExit('Shared native profile is unavailable')
 profile=json.loads(profile_path.read_text())
 if not a.skip_prepare:
  subprocess.run(['python3',str(ROOT/'tools/port-rust-span-reference/prepare.py'),'--output',profile['workspace'],'--rustc',profile['rustc'],'--rustdoc',profile['rustdoc'],'--cargo',profile['cargo']],cwd=ROOT,check=True)
  profile=json.loads(profile_path.read_text())
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or not profile['debug_assertions']or not profile['overflow_checks']or profile['upstream_tests_passed']!=43:raise SystemExit('Shared native reference profile differs')
 provenance_path=pathlib.Path(profile['provenance_path']);provenance_bytes=provenance_path.read_bytes()
 if sha(provenance_bytes)!=profile['provenance_sha256']:raise SystemExit('Shared provenance digest differs')
 provenance=json.loads(provenance_bytes)
 for source in provenance['source_files']:
  path=(pathlib.Path(profile['original_root'])if source['path'].startswith('compiler/')else pathlib.Path(profile['workspace'])/'upstream-root')/source['path']
  if path.is_symlink()or not path.is_file()or sha(path.read_bytes())!=source['sha256']:raise SystemExit('Original source graph changed:'+source['path'])
 original=next(source for source in provenance['source_files']if source['path']=='compiler/rustc_data_structures/src/unord.rs')
 with tempfile.TemporaryDirectory(prefix='ferrugo-original-unord-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle')
  result=subprocess.run([profile['rustc'],str(HERE/'oracle.rs'),*profile['link_arguments'],'-o',executable],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  raw=subprocess.check_output([executable])
 if len(raw.splitlines())!=2000:raise SystemExit('Original Unord reference coverage differs')
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_map.tsv.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 manifest={'scope':'Partial genuine UnordMap foundation and actual read-items/filter/map/collect path; generic higher iterator/sort/codec/stablehash contracts remain pending and receive no wholefile credit','source_revision':profile['source_revision'],'compiler':profile['rustc_version'],'unicode':profile['unicode'],'cases':2000,'original_unord':original,'provenance_sha256':profile['provenance_sha256'],'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'reference_tools':{name:sha((ROOT/'tools/port-rust-span-reference'/name).read_bytes())for name in ['prepare.py','verify.py','sources.json','Cargo.lock','toolchain.json']}}
 metadata=HERE/'manifest.json'
 if a.check:
  if fixture.read_bytes()!=compressed or json.loads(metadata.read_text())!=manifest:raise SystemExit('Original UnordMap proof differs')
 else:fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print('2,000 actual whole-original UnordMap operation/pipeline cases match')
if __name__=='__main__':main()
