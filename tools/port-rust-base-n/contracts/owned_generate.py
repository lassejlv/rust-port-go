#!/usr/bin/env python3
"""Unchanged whole-source BaseN ownership/trait oracle, isolated aborts included."""
import argparse,gzip,hashlib,json,os,pathlib,resource,shutil,signal,subprocess,sys
ROOT=pathlib.Path(__file__).resolve().parents[3]
HERE=pathlib.Path(__file__).resolve().parent
TARGET=ROOT/'port/rust/compiler/rustc_data_structures/src'
PIN='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
BLOB='0c3d7613d4f1ff69a31c4458bd571074bbd5c618'
SOURCE='compiler/rustc_data_structures/src/base_n.rs'
MAX=(1<<64)-1
OBSERVATION_TIMEOUT=30
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,**kw):
 r=subprocess.run([str(x)for x in args],cwd=ROOT,env=env,capture_output=True,timeout=120,**kw)
 if r.returncode:raise RuntimeError(r.stderr.decode(errors='replace'))
 return r.stdout
def requests():
 result=[];seen=set()
 def add(kind=0,width=5,fixed=1,base=10,n=7,mask=0,disposition=0,mode=0):
  request='\t'.join(map(str,[kind,width,fixed,base,n>>64,n&MAX,mask,disposition,mode,'v1']))
  if request not in seen:seen.add(request);result.append(request)
 # Every panic combination, each independent override, both operations, all
 # ambient contexts. Drop glue and local cleanup are actual Rust impls.
 for kind in range(4):
  for fixed in [0,1]:
   for mode in range(4):
    for mask in range(64):add(kind=kind,fixed=fixed,mode=mode,mask=mask)
 # Release start wrapping, checked construction, empty/short/full borrowed
 # buffers and invalid bases; operation phase is retained separately.
 for kind in range(4):
  for width in [0,1,128,129,MAX]:
   for fixed in [0,1]:
    for base in [0,1,2,10,64,65,MAX]:
     for mode in range(4):add(kind,width,fixed,base,mode=mode)
 # The actual From/override can destroy, forget or move Self into longer-lived
 # storage. Later destruction receives a fresh scope, after outer catch.
 for disposition in [1,2,3]:
  for kind in [0,3]:
   for fixed in [0,1]:
    for mode in range(4):
     for mask in [0,1,2,4,8,16,24,32]:add(kind=kind,fixed=fixed,mode=mode,mask=mask,disposition=disposition)
 # Actual primitive implementations, including narrow u32 divisor casts.
 for kind in [32,64,128]:
  for fixed in [0,1]:
   for base in [0,1,2,10,64,65,1<<32,(1<<32)+2,MAX]:
    if fixed and base==1:continue # source's maximum is never reduced
    for n in [0,7,(1<<128)-1]:
     for mode in range(4):add(kind=kind,fixed=fixed,base=base,n=n,mode=mode)
 return result
def observe(binary,request,env):
 try:
  r=subprocess.run([str(binary)],input=(request+'\n').encode(),env=env,capture_output=True,timeout=OBSERVATION_TIMEOUT)
 except subprocess.TimeoutExpired as error:
  raise RuntimeError(f'native observation exceeded {OBSERVATION_TIMEOUT}s: {binary.name}: {request}')from error
 if len(r.stdout)>65536 or len(r.stderr)>65536:raise RuntimeError('native observation exceeded bound')
 lines=r.stdout.decode().splitlines()
 if r.returncode==-signal.SIGABRT:return {'status':'abort','observation':'|'.join(lines)}
 if r.returncode:raise RuntimeError(f'native process failed {request}: {r.returncode}: {r.stderr.decode()}')
 if not lines or len(lines[-1].split('\t'))!=7:raise RuntimeError('native result schema differs')
 if lines[-1].split('\t')[-1]!='|'.join(lines[:-1]):raise RuntimeError('streamed native events differ')
 return {'status':'returned','observation':lines[-1]}
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args()
 resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 env=os.environ.copy();env.update(RUSTC_BOOTSTRAP='1',GIT_NO_LAZY_FETCH='1',FERRUGO_BASE_N_ABORT_TRACE='1')
 for key in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS']:env.pop(key,None)
 rustc=shutil.which(env.get('FERRUGO_RUSTC',str(ROOT/'.toolchain/rust/bin/rustc')))
 if not rustc:raise RuntimeError('BaseN Rust compiler unavailable')
 version=run([rustc,'--version'],env).decode().strip()
 if version!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('BaseN comparison compiler differs')
 if b'target_pointer_width="64"' not in run([rustc,'--print','cfg'],env):raise RuntimeError('BaseN owned fixture requires its explicit 64-bit native profile')
 frozen={
  'tools/port-rust-base-n/testdata/oracle.tsv.gz':'98c322602c43542644357be7b7c727c9e8f3904cbda75e117e06ad4d9485e65d',
  'port/rust/compiler/rustc_data_structures/src/testdata/base_n_formats.tsv.gz':'1bbcae68d43132928a820165954b10008fd2619e720960c16595f7c44a7ff843',
 }
 for path,expected in frozen.items():
  if sha((ROOT/path).read_bytes())!=expected:raise RuntimeError('retained numeric/formatter fixture changed: '+path)
 upstream=pathlib.Path(env.get('FERRUGO_RUST_SOURCE',ROOT/'.toolchain/upstream-rust'))
 if run(['git','-C',upstream,'rev-parse',PIN+':'+SOURCE],env).decode().strip()!=BLOB:raise RuntimeError('source Git blob differs')
 original=run(['git','-C',upstream,'cat-file','blob',BLOB],env)
 test=run(['git','-C',upstream,'show',PIN+':compiler/rustc_data_structures/src/base_n/tests.rs'],env)
 work=ROOT/'.toolchain/oracles/base-n-owned';work.mkdir(parents=True,exist_ok=True)
 (work/'base_n.rs').write_bytes(original);(work/'base_n').mkdir(exist_ok=True);(work/'base_n/tests.rs').write_bytes(test)
 (work/'main.rs').write_bytes((HERE/'owned_oracle.rs').read_bytes())
 inputs=requests();records=[];profiles=[]
 for debug in [False,True]:
  for checked in [False,True]:
   name=f'd{int(debug)}o{int(checked)}';flags=['-C','debug-assertions='+str(debug).lower(),'-C','overflow-checks='+str(checked).lower()]
   binary=work/('owned-'+name);run([rustc,'--edition=2024',*flags,work/'main.rs','-o',binary],env)
   test_binary=work/('tests-'+name);run([rustc,'--edition=2024',*flags,'--test',work/'main.rs','-o',test_binary],env)
   if b'2 passed; 0 failed' not in run([test_binary],env):raise RuntimeError('original tests did not pass')
   aborted=0
   for request in inputs:
    result=observe(binary,request,env);aborted+=result['status']=='abort';records.append(dict(profile=name,request=request,**result))
   profiles.append(dict(name=name,debug_assertions=debug,overflow_checks=checked,records=len(inputs),aborted=aborted,original_tests_passed=2))
   print(f'BaseN owned {name}: {len(inputs)} observations, {aborted} isolated aborts',flush=True)
 raw=b''.join((json.dumps(record,separators=(',',':'))+'\n').encode()for record in records)
 fixture=TARGET/'testdata/base_n_owned.jsonl.gz'
 old=fixture.read_bytes()if fixture.exists()else b''
 compressed=old if old and gzip.decompress(old)==raw else gzip.compress(raw,mtime=0)
 bindings=['base_n.go','base_n_owned.go','base_n_owned_contract_test.go']
 info=dict(format=1,revision=PIN,source_git_blob=BLOB,source_sha256=sha(original),original_tests_sha256=sha(test),compiler=version,pointer_bits=64,profiles=profiles,records=len(records),request_sha256=sha(('\n'.join(inputs)+'\n').encode()),raw_sha256=sha(raw),fixture_sha256=sha(compressed),retained_fixtures=frozen,bindings={str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [HERE/'owned_oracle.rs',pathlib.Path(__file__),*[TARGET/name for name in bindings],ROOT/'port/support/rustvec/ownership.go',ROOT/'port/support/rustfmt/formatter.go']},scope='Complete unchanged base_n.rs, actual non-Copy argument and field Drop glue; independent Into/ToBaseN overrides, static width, borrowed buffer identity, ambient/caught/escaping/hook panic states. std 1.99 comparison profile; no Rust ABI/layout claim.',bounds={'native_observation_seconds':OBSERVATION_TIMEOUT,'native_stdout_bytes':65536,'native_stderr_bytes':65536},excluded='Primitive encoded_len base one (or u32 cast-to-one): maximum never decreases; no finite runtime claim. 32-bit runtime is not executable on this host.')
 manifest=HERE/'owned-manifest.json';encoded=(json.dumps(info,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or manifest.read_bytes()!=encoded:raise RuntimeError('BaseN owned native fixture/bindings differ')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'BaseN ownership: {len(records)} original-source records; all four profiles verified.')
if __name__=='__main__':
 try:main()
 except Exception as error:print(error,file=sys.stderr);sys.exit(1)
