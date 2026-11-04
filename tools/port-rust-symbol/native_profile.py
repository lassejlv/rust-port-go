"""Explicit matching source/compiler profile shared by new Symbol tools."""
import os
from pathlib import Path
import shutil
import subprocess

ROOT=Path(__file__).resolve().parents[2]
REVISION='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
SOURCE_VERSION='1.101.0'
NIGHTLY_DATE='2026-11-04'
COMPILER_COMMIT='708c94ef093ecc42fb8851e17d8383d999727abc'
UNICODE_VERSION=[18,0,0]

def source_bytes(relative):
    value=os.environ.get('FERRUGO_RUST_SOURCE',str(ROOT/'.toolchain/upstream-rust'))
    if not value:raise SystemExit('FERRUGO_RUST_SOURCE explicitly empty')
    source=Path(value);source=source if source.is_absolute() else ROOT/source
    env=os.environ.copy();env['GIT_NO_LAZY_FETCH']='1'
    result=subprocess.run(['git','-C',str(source),'show',REVISION+':'+relative],stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env,timeout=30)
    if result.returncode:raise SystemExit(f'unavailable pinned source object {relative}: {result.stderr.decode(errors="replace")[:1000]}')
    return result.stdout

def executable(value):
    if '/' in value:
        path=Path(value);result=str(path if path.is_absolute() else ROOT/path)
    else:result=shutil.which(value)
    if not result or not os.access(result,os.X_OK):raise SystemExit('unavailable executable: '+value)
    return result

def compiler_profile(environment=None):
    env=os.environ.copy() if environment is None else environment.copy()
    env['GIT_NO_LAZY_FETCH']='1'
    rustc=executable(env.get('FERRUGO_RUSTC',f'.toolchain/rust-nightly-{NIGHTLY_DATE}/bin/rustc'))
    result=subprocess.run([rustc,'--version','--verbose'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env,timeout=30)
    if result.returncode:raise SystemExit('selected Rust compiler did not report its profile')
    verbose=result.stdout.decode().strip()
    if f'commit-hash: {COMPILER_COMMIT}' not in verbose or 'release: 1.101.0-nightly' not in verbose:
        raise SystemExit(f'Symbol tools require official nightly-{NIGHTLY_DATE} ({COMPILER_COMMIT}), selected compiler reports:\n{verbose}')
    env['FERRUGO_RUSTC']=rustc
    return rustc,env,{'source_version':SOURCE_VERSION,'nightly_date':NIGHTLY_DATE,'compiler_commit':COMPILER_COMMIT,'compiler':verbose,'unicode_version':UNICODE_VERSION}
