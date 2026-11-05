# SPDX-License-Identifier: MIT OR Apache-2.0
"""Verified complete original span reference for these scoped source tools."""
import importlib.util,json,os,subprocess,sys
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2]
def module(name,path):
 spec=importlib.util.spec_from_file_location(name,path);value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value
PREPARE=module('ferrugo_owned_span_prepare',ROOT/'tools/port-rust-span-reference/prepare.py')
PROFILE=module('ferrugo_owned_span_profile',ROOT/'tools/port-rust-symbol/native_profile.py')
def load(debug=True,overflow=True,rustc=None):
 env=os.environ.copy()
 if rustc is not None:env['FERRUGO_RUSTC']=rustc
 selected,env,metadata=PROFILE.compiler_profile(env)
 # Apply the same non-resolving executable selection as the real reference
 # helper, preserving Rustup multicall argv[0] for explicit/PATH overrides.
 selected=PREPARE.executable(selected,'FERRUGO_RUSTC',Path('.toolchain/rust-nightly-2026-11-04/bin/rustc'))
 command=[sys.executable,str(ROOT/'tools/port-rust-span-reference/prepare.py'),'--rustc',selected,'--debug-assertions',str(debug).lower(),'--overflow-checks',str(overflow).lower()]
 # Always revalidate Git blobs, all108 archives/expanded sources, compiler
 # selection, all43 original tests and linkage. A profile's existence alone
 # never makes its artifacts trusted. The helper serializes shared-cache work.
 result=subprocess.run(command,cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=240)
 if result.returncode:raise RuntimeError('original span verification failed:\n'+result.stderr.decode(errors='replace')[-12000:])
 path=ROOT/f'.toolchain/oracles/rustc-span-reference/build-profile-d{int(debug)}o{int(overflow)}.json'
 profile=json.loads(path.read_text())
 if profile['rustc']!=selected or profile['debug_assertions']!=debug or profile['overflow_checks']!=overflow:raise RuntimeError('selected reference profile differs')
 if profile['source_revision']!=PROFILE.REVISION or profile['cfg_bootstrap'] or profile['unicode']!=[18,0,0]:raise RuntimeError('reference source/runtime selection differs')
 env['RUSTC_BOOTSTRAP']='1'
 return profile,env,metadata

def source_bytes(relative):return PROFILE.source_bytes(relative)
