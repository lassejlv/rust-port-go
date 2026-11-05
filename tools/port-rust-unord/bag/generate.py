#!/usr/bin/env python3
"""Genuine whole-original UnordBag core, Debug and StableHash observations."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile,collections
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('sort_tools',HERE.parent/'sort/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(b):return hashlib.sha256(b).hexdigest()
def validate_counts(raw,expected):
 counts=dict(collections.Counter(row.split(b'\t')[0].decode()for row in raw.splitlines()))
 if counts!=expected:raise ValueError('Bag coverage differs:'+str(counts))
 return counts
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args()
 profile=json.loads(pathlib.Path(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json')).read_text())
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or profile['upstream_tests_passed']!=43 or not profile['debug_assertions']or not profile['overflow_checks']:raise SystemExit('Shared reference profile differs')
 provenance=pathlib.Path(profile['provenance_path']).read_bytes()
 if sha(provenance)!=profile['provenance_sha256']:raise SystemExit('Source graph provenance differs')
 for item in json.loads(provenance)['source_files']:
  path=(pathlib.Path(profile['original_root'])if item['path'].startswith('compiler/')else pathlib.Path(profile['workspace'])/'upstream-root')/item['path']
  if path.is_symlink()or not path.is_file()or sha(path.read_bytes())!=item['sha256']:raise SystemExit('Source graph differs:'+item['path'])
 if subprocess.check_output([profile['rustc'],'--version'],text=True).strip()!=profile['rustc_version']:raise SystemExit('SDK differs')
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-bag-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');subprocess.run([profile['rustc'],str((HERE/'oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,check=True);raw=subprocess.check_output([executable])
 counts=validate_counts(raw,{'C':36,'H':375,'D':849})
 fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_bag.tsv.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 names=['tools/port-rust-unord/bag/'+n for n in ['oracle.rs','generate.py','README.md','test_tools.py']]+['port/rust/compiler/rustc_data_structures/src/'+n for n in ['unord_bag.go','unord_bag_oracle_test.go','unord_traits_test.go']]
 manifest={'scope':'Actual UnordBag storage/traits and fixed item count/get_only foundations; full generic UnordItems algebra/sorting/codecs remain pending','cases':1260,'counts':counts,'source_revision':profile['source_revision'],'compiler':profile['rustc_version'],'provenance_sha256':profile['provenance_sha256'],'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{n:sha((ROOT/n).read_bytes())for n in names}}
 metadata=HERE/'manifest.json'
 if a.check:
  if metadata.read_text()!=json.dumps(manifest,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Bag proof differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordBag$','-count=1'],cwd=ROOT,check=True)
 print('1,260 actual original Bag core/StableHash/Debug records')
if __name__=='__main__':main()
