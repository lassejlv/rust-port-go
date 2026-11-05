#!/usr/bin/env python3
"""Run all original symbol proc-macro tests without changing source/helper files."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import native_profile
import symbol_reference

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,**kwargs):
 result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180,**kwargs)
 if result.returncode:raise SystemExit(f'command failed: {command}\n{result.stderr.decode(errors="replace")[-4096:]}')
 return result
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--prepared',action='store_true');parser.add_argument('--offline',action='store_true');args=parser.parse_args()
 profile,env,compiler=symbol_reference.load();rustc=profile['rustc']
 cargo=native_profile.executable(env.get('FERRUGO_CARGO','.toolchain/rust-nightly-2026-11-04/bin/cargo'))
 cargo_version=run([cargo,'--version'],env=env).stdout.decode().strip()
 if cargo_version!='cargo 1.101.0-nightly (31d3246e0 2026-10-27)':raise SystemExit('original macro test Cargo profile mismatch')
 rustdoc=native_profile.executable(env.get('FERRUGO_RUSTDOC',str(Path(rustc).parent/'rustdoc')))
 for name in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','CARGO_BUILD_TARGET','UPDATE_EXPECT']:env.pop(name,None)
 work=ROOT/'.toolchain/oracles/rustc-span-reference'
 for key,value in {'RUSTC':rustc,'RUSTDOC':rustdoc,'RUSTC_BOOTSTRAP':'1','CFG_RELEASE':'1.101.0','CARGO_HOME':env.get('CARGO_HOME',str(ROOT/'.toolchain/port-cargo-home')),'CARGO_TARGET_DIR':work/'target/d1o1','CARGO_PROFILE_DEV_DEBUG_ASSERTIONS':'true','CARGO_PROFILE_DEV_OVERFLOW_CHECKS':'true','CARGO_PROFILE_DEV_OPT_LEVEL':'0','CARGO_PROFILE_TEST_DEBUG_ASSERTIONS':'true','CARGO_PROFILE_TEST_OVERFLOW_CHECKS':'true','CARGO_PROFILE_TEST_OPT_LEVEL':'0'}.items():env[key]=str(value)
 command=[cargo,'test','--locked','--offline','-p','rustc_macros','symbols::tests','--lib']
 with (work/'.prepare.lock').open('a+b')as lock:
  fcntl.flock(lock.fileno(),fcntl.LOCK_EX)
  result=run(command,cwd=work,env=env)
 cache=ROOT/'.toolchain/oracles/symbol-macro-tests';cache.mkdir(parents=True,exist_ok=True);(cache/'stdout').write_bytes(result.stdout);(cache/'stderr').write_bytes(result.stderr)
 tests_path='compiler/rustc_macros/src/symbols/tests.rs';source=(work/'original'/tests_path).read_bytes();declarations=re.findall(r'#\[test\]\s*fn ([A-Za-z_0-9]+)',source.decode())
 statuses=re.findall(r'^test symbols::tests::([A-Za-z_0-9]+) \.\.\. (\w+)$',result.stdout.decode(),re.M)
 if len(statuses)!=4 or sorted(statuses)!=sorted((name,'ok')for name in declarations)or b'4 passed; 0 failed;'not in result.stdout:raise SystemExit('original symbol macro tests did not all pass')
 metadata={'rust_revision':native_profile.REVISION,**compiler,'cargo':cargo_version,'cfg_release':'1.101.0','debug_assertions':True,'overflow_checks':True,'scope':'Development dependency proof only: all four unchanged original Rust symbol proc-macro tests, including exact duplicate diagnostics and full input parse. Does not claim a Go Rust parser/proc-macro crate port.','reference_inputs':symbol_reference.inputs(),'command':['cargo',*command[1:]],'source_test_file':tests_path,'source_test_sha256':sha(source),'test_declaration_order':declarations,'results':[{'test':name,'result':'ok'}for name in declarations],'shared_original_span_tests':profile['upstream_tests_passed'],'helpers':[{'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())}for path in[Path(__file__).resolve(),HERE/'native_profile.py']]}
 path=ROOT/'port/rust/compiler/rustc_span/src/symbol_macro_tests_manifest.json';encoded=(json.dumps(metadata,indent=2)+'\n').encode()
 if args.check:
  if not path.exists()or path.read_bytes()!=encoded:raise SystemExit('original symbol macro test manifest differs')
 else:path.write_bytes(encoded)
 print('original Rust symbol macro tests:4 passed, exact sources unchanged')
if __name__=='__main__':main()
