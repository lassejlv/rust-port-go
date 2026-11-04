#!/usr/bin/env python3
"""Native whole-file BaseN formatter/borrow contracts; no production Rust fallback."""
import argparse,gzip,hashlib,json,os,pathlib,shutil,subprocess,sys
ROOT=pathlib.Path(__file__).resolve().parents[3]
HERE=pathlib.Path(__file__).resolve().parent
TARGET=ROOT/'port/rust/compiler/rustc_data_structures/src'
PIN='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
SOURCE='compiler/rustc_data_structures/src/base_n.rs'
BLOB='0c3d7613d4f1ff69a31c4458bd571074bbd5c618'
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,**kwargs):
 result=subprocess.run([str(x)for x in args],env=env,cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120,**kwargs)
 if result.returncode:raise RuntimeError(result.stderr.decode(errors='replace'))
 return result.stdout
def executable(value):
 result=shutil.which(value)
 if not result:raise RuntimeError('executable unavailable:'+value)
 return os.path.abspath(result)
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();env=os.environ.copy();env.update(RUSTC_BOOTSTRAP='1',GIT_NO_LAZY_FETCH='1')
 rustc=executable(env.get('FERRUGO_RUSTC',str(ROOT/'.toolchain/rust/bin/rustc')))
 version=run([rustc,'--version'],env).decode().strip()
 if version!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('BaseN reference compiler differs')
 upstream=pathlib.Path(env.get('FERRUGO_RUST_SOURCE',ROOT/'.toolchain/upstream-rust'))
 if run(['git','-C',upstream,'rev-parse',PIN+':'+SOURCE],env).decode().strip()!=BLOB:raise RuntimeError('BaseN original Git object differs')
 original=run(['git','-C',upstream,'cat-file','blob',BLOB],env)
 work=ROOT/'.toolchain/oracles/base-n-contracts';work.mkdir(parents=True,exist_ok=True);(work/'base_n.rs').write_bytes(original)
 test=run(['git','-C',upstream,'show',PIN+':compiler/rustc_data_structures/src/base_n/tests.rs'],env);(work/'base_n').mkdir(exist_ok=True);(work/'base_n/tests.rs').write_bytes(test)
 source=HERE/'format_oracle.rs';(work/'oracle.rs').write_bytes(source.read_bytes());raw=None;profiles=[]
 for debug in [False,True]:
  for checked in [False,True]:
   profile=f'd{int(debug)}o{int(checked)}';flags=['-C','debug-assertions='+str(debug).lower(),'-C','overflow-checks='+str(checked).lower()]
   binary=work/('format-'+profile);run([rustc,'--edition=2024',*flags,work/'oracle.rs','-o',binary],env)
   observed=run([binary],env)
   if raw is not None and observed!=raw:raise RuntimeError('finite valid formatter cases changed across independent profiles')
   raw=observed;test_binary=work/('tests-'+profile);run([rustc,'--edition=2024',*flags,'--test',work/'oracle.rs','-o',test_binary],env);result=run([test_binary],env)
   if b'2 passed; 0 failed' not in result:raise RuntimeError('both original BaseN tests did not pass')
   profiles.append({'name':profile,'debug_assertions':debug,'overflow_checks':checked,'original_tests_passed':2})
 fixture=TARGET/'testdata/base_n_formats.tsv.gz';fixture.parent.mkdir(exist_ok=True)
 compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0)
 info={'format':1,'revision':PIN,'source_git_blob':BLOB,'source_sha256':sha(original),'original_tests_sha256':sha(test),'compiler':version,'profiles':profiles,'records':len(raw.splitlines()),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'oracle_sha256':sha(source.read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'scope':'Unchanged whole BaseN source; actual Display flags, borrowed buffer identity, all selected write failures and mutable formatter state. Owned conversion evidence is separate.'}
 manifest=HERE/'format-manifest.json';encoded=(json.dumps(info,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or manifest.read_bytes()!=encoded:raise RuntimeError('BaseN native format observations/bindings differ')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'BaseN full borrowed Display: {info["records"]} native records; four profiles and all original tests pass.')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
