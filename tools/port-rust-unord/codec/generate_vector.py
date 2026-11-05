#!/usr/bin/env python3
"""Observe actual vector codecs, opaque-byte dispatch/capacity/independence."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile,collections
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
spec=importlib.util.spec_from_file_location('index_context_tools',HERE.parent/'index-context/generate.py');TOOLS=importlib.util.module_from_spec(spec);spec.loader.exec_module(TOOLS)
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--skip-go',action='store_true');a=p.parse_args();profile=json.loads(pathlib.Path(os.environ.get('FERRUGO_SPAN_REFERENCE_PROFILE',ROOT/'.toolchain/oracles/rustc-span-reference/build-profile.json')).read_text());TOOLS.verify_sources(profile)
 if profile['source_revision']!='2d20b42b883dcfc014f3c2c56081eeb6091c2179'or profile['rustc_version']!='rustc 1.101.0-nightly (708c94ef0 2026-11-03)'or profile['upstream_tests_passed']!=43 or not profile['debug_assertions']or not profile['overflow_checks']:raise SystemExit('Shared reference profile differs')
 with tempfile.TemporaryDirectory(prefix='ferrugo-unord-vectors-')as temporary:
  executable=str(pathlib.Path(temporary)/'oracle');result=subprocess.run([profile['rustc'],str((HERE/'vector_oracle.rs').relative_to(ROOT)),*profile['link_arguments'],'-o',executable],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:raise SystemExit(result.stderr.decode(errors='replace'))
  raw=subprocess.check_output([executable])
 counts=dict(collections.Counter(row.split(b'\t')[0].decode()for row in raw.splitlines()))
 if counts!={'G':236,'O':36,'X':20}:raise SystemExit('Vector coverage differs:'+str(counts))
 locations=set()
 for row in raw.decode().splitlines():
  f=row.split('\t')
  if f[0]=='X'and f[-1]:
   info=bytes.fromhex(f[-1]).decode().split(':')
   if info[0]!='static_str'or info[-3:]!=['true','true','false']:raise SystemExit('Opaque decoder panic contract differs')
   locations.add((bytes.fromhex(info[1]).decode(),int(info[2]),int(info[3])))
 if len(locations)!=1:raise SystemExit('Opaque decoder source location varies')
 location=next(iter(locations));fixture=ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_vector_codec.tsv.gz';compressed=gzip.compress(raw,mtime=0)
 if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw:compressed=fixture.read_bytes()
 names=['tools/port-rust-unord/codec/'+n for n in ['vector_oracle.rs','generate_vector.py']]+['tools/port-rust-unord/index-context/generate.py']+['port/rust/compiler/rustc_data_structures/src/'+n for n in ['unord_bag.go','unord_bag_codec.go','unord_vector_codec_test.go']]+['port/rust/compiler/rustc_serialize/src/'+n for n in ['container_vec.go','opaque_memory.go']]
 manifest={'scope':'Actual generic and opaque slice/Vec/UnordBag encoding/decoding; bulk-dispatch capacity, copy independence, precise exhaustion hook/location/position; no std/full-module credit','cases':292,'counts':counts,'source_revision':profile['source_revision'],'compiler':profile['rustc_version'],'provenance_sha256':profile['provenance_sha256'],'location':{'File':location[0],'Line':location[1],'Column':location[2]},'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'files':{n:sha((ROOT/n).read_bytes())for n in names}};metadata=HERE/'vector_manifest.json'
 if a.check:
  if metadata.read_text()!=json.dumps(manifest,indent=2)+'\n'or fixture.read_bytes()!=compressed:raise SystemExit('Vector codec proof differs')
 else:fixture.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 if not a.skip_go:subprocess.run([TOOLS.TOOLS.go_compiler(),'test','./port/rust/compiler/rustc_data_structures/src','-run','^TestOriginalUnordVectorCodec$','-count=1'],cwd=ROOT,check=True)
 print('292 actual generic/bulk vector codec records')
if __name__=='__main__':main()
