#!/usr/bin/env python3
"""Original whole-crate UnordSet/derived Debug/stable-hash proof."""
import argparse,gzip,hashlib,json,os,pathlib,subprocess,tempfile
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-prepare',action='store_true');a=p.parse_args()
 profile_path=pathlib.Path(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json'))
 if not profile_path.is_file():raise SystemExit('Shared whole-crate reference profile is unavailable')
 profile=json.loads(profile_path.read_text())
 if not a.skip_prepare:
  subprocess.run(['python3',str(ROOT/'tools/port-rust-span-reference/prepare.py'),'--output',profile['workspace'],'--rustc',profile['rustc'],'--rustdoc',profile['rustdoc'],'--cargo',profile['cargo']],cwd=ROOT,check=True)
  profile=json.loads(profile_path.read_text())
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or not profile['debug_assertions']or not profile['overflow_checks']or profile['upstream_tests_passed']!=43:raise SystemExit('Shared whole-crate reference profile differs')
 provenance_bytes=pathlib.Path(profile['provenance_path']).read_bytes()
 if sha(provenance_bytes)!=profile['provenance_sha256']:raise SystemExit('Shared original-source provenance differs')
 provenance=json.loads(provenance_bytes)
 for source in provenance['source_files']:
  path=(pathlib.Path(profile['original_root'])if source['path'].startswith('compiler/')else pathlib.Path(profile['workspace'])/'upstream-root')/source['path']
  if path.is_symlink()or not path.is_file()or sha(path.read_bytes())!=source['sha256']:raise SystemExit('Original source graph changed:'+source['path'])
 original=next(s for s in provenance['source_files']if s['path']=='compiler/rustc_data_structures/src/unord.rs')
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-traits-')as temp:
  executable=str(pathlib.Path(temp)/'oracle')
  subprocess.run([profile['rustc'],str(HERE/'oracle.rs'),*profile['link_arguments'],'-o',executable],cwd=ROOT,check=True)
  raw=subprocess.check_output([executable])
 counts={tag:sum(line.startswith(tag.encode()+b'\t')for line in raw.splitlines())for tag in ['S','H','D']}
 if counts!={'S':2400,'H':1083,'D':2179}:raise SystemExit('Original trait proof coverage differs:'+str(counts))
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_traits.tsv.gz'
 compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 manifest={'scope':'Actual UnordSet foundation, derived UnordMap/Set Debug, and commutative StableHash including partial caller state on callback panic; full Unord iterator/sort/codec contracts remain pending','source_revision':profile['source_revision'],'compiler':profile['rustc_version'],'counts':counts,'cases':len(raw.splitlines()),'original_unord':original,'provenance_sha256':profile['provenance_sha256'],'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'oracle_sha256':sha((HERE/'oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'reference_tools':{name:sha((ROOT/'tools/port-rust-span-reference'/name).read_bytes())for name in ['prepare.py','verify.py','sources.json','Cargo.lock','toolchain.json']}}
 metadata=HERE/'manifest.json'
 if a.check:
  if fixture.read_bytes()!=compressed or json.loads(metadata.read_text())!=manifest:raise SystemExit('Original Unord trait proof differs')
 else:fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print(f'{len(raw.splitlines()):,} actual original Unord trait records match: {counts}')
if __name__=='__main__':main()
