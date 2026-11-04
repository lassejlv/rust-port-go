#!/usr/bin/env python3
"""Run all unchanged original semver tests in an isolated Cargo workspace."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tomllib

sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('semver_original_mirror',HERE/'mirror.py')
MIRROR=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(MIRROR)
REGISTRY_SPEC=importlib.util.spec_from_file_location('semver_original_registry',HERE/'registry_source.py')
REGISTRY=importlib.util.module_from_spec(REGISTRY_SPEC);REGISTRY_SPEC.loader.exec_module(REGISTRY)

def write(path,data):
 path.parent.mkdir(parents=True,exist_ok=True)
 if not path.exists() or path.read_bytes()!=data:path.write_bytes(data)

def verify_registry(destination,cargo_home):
 entries=[]
 for package in tomllib.loads((destination/'Cargo.lock').read_text())['package']:
  if 'source' not in package:continue
  archives=list((cargo_home/'registry/cache').glob(f"*/{package['name']}-{package['version']}.crate"))
  if len(archives)!=1 or archives[0].is_symlink() or hashlib.sha256(archives[0].read_bytes()).hexdigest()!=package['checksum']:raise SystemExit('missing or mismatched registry archive: '+package['name'])
  entry={'name':package['name'],'version':package['version'],'sha256':package['checksum']}
  entry.update(REGISTRY.verify_registry_source(archives[0],cargo_home,package['name'],package['version']));entries.append(entry)
 return entries

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--root',type=Path,default=MIRROR.ROOT);parser.add_argument('--fetch',action='store_true');parser.add_argument('--offline',action='store_true');parser.add_argument('--write-lock',action='store_true');parser.add_argument('--node',action='store_true');parser.add_argument('--target');args=parser.parse_args();root=args.root.resolve();files=MIRROR.source_files(root)
 destination=root/'.toolchain/oracles/semver-original-tests'
 for path,data in files.items():write(destination/'original'/path,data)
 write(destination/'Cargo.toml',(HERE/'Cargo.toml').read_bytes())
 if (HERE/'Cargo.lock').exists():write(destination/'Cargo.lock',(HERE/'Cargo.lock').read_bytes())
 elif not(destination/'Cargo.lock').exists():write(destination/'Cargo.lock',(root/'.toolchain/upstream-rust/Cargo.lock').read_bytes())
 cargo=os.environ.get('FERRUGO_CARGO') or str(root/'.toolchain/rust/bin/cargo');rustc=os.environ.get('FERRUGO_RUSTC') or str(root/'.toolchain/rust/bin/rustc')
 environment=os.environ.copy();environment['CARGO_HOME']=str(Path(environment.get('CARGO_HOME',root/'.toolchain/port-cargo-home')).resolve());environment['RUSTC']=shutil.which(rustc) or str(Path(rustc).resolve())
 environment['RUSTDOC']=environment.get('FERRUGO_RUSTDOC') or environment.get('RUSTDOC') or str(Path(environment['RUSTC']).with_name('rustdoc'))
 tail=['--manifest-path',str(destination/'Cargo.toml')]
 if args.offline:tail.append('--offline')
 if args.fetch:
  command=[cargo,'fetch',*tail]
  if(HERE/'Cargo.lock').exists():command.append('--locked')
  subprocess.run(command,env=environment,check=True)
 if args.write_lock:write(HERE/'Cargo.lock',(destination/'Cargo.lock').read_bytes())
 registry=verify_registry(destination,Path(environment['CARGO_HOME']))
 profiles=[];logs=destination/'logs';logs.mkdir(exist_ok=True)
 configurations=[('std',[]),('no_std',['--no-default-features']),('serde',['--features','serde']),('no_std_serde',['--no-default-features','--features','serde'])]
 if args.node:configurations.append(('node',[]))
 for name,flags in configurations:
  current=environment.copy()
  if name=='node':current['RUSTFLAGS']='--cfg test_node_semver'
  command=[cargo,'test',*tail,'--locked','--tests',*flags]
  if args.target:command.extend(['--target',args.target])
  command.extend(['--','--test-threads=1'])
  result=subprocess.run(command,env=current,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  write(logs/(name+'.stdout'),result.stdout);write(logs/(name+'.stderr'),result.stderr)
  if result.returncode:raise SystemExit(f'original semver profile {name} failed:\n{result.stdout.decode(errors="replace")}\n{result.stderr.decode(errors="replace")}')
  passed=[]
  for line in result.stdout.decode().splitlines():
   if line.startswith('test ') and line.endswith(' ... ok'):passed.append(line[5:-7])
  if len(passed)!=34:raise SystemExit(f'original profile {name} ran {len(passed)} tests, expected44')
  doc_command=[cargo,'test',*tail,'--locked','--doc',*flags]
  if args.target:doc_command.extend(['--target',args.target])
  doc_result=subprocess.run(doc_command,env=current,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  write(logs/(name+'.doc.stdout'),doc_result.stdout);write(logs/(name+'.doc.stderr'),doc_result.stderr)
  if doc_result.returncode:raise SystemExit(f'original semver doc profile {name} failed:\n{doc_result.stdout.decode(errors="replace")}\n{doc_result.stderr.decode(errors="replace")}')
  doc_passed=[line[5:-7] for line in doc_result.stdout.decode().splitlines() if line.startswith('test ') and line.endswith(' ... ok')]
  if len(doc_passed)!=4:raise SystemExit(f'original doc profile {name} ran {len(doc_passed)} doc tests, expected4')
  profiles.append({'name':name,'tests_passed':len(passed),'test_names':passed,'doc_tests_passed':len(doc_passed),'doc_test_names':doc_passed,'doc_assertion_sites':4,'flags':flags,'rustflags':current.get('RUSTFLAGS',''),'target':args.target or 'host','stdout_sha256':hashlib.sha256(result.stdout).hexdigest(),'stderr_sha256':hashlib.sha256(result.stderr).hexdigest(),'doc_stdout_sha256':hashlib.sha256(doc_result.stdout).hexdigest(),'doc_stderr_sha256':hashlib.sha256(doc_result.stderr).hexdigest()})
  print(name,': all34 unchanged original Rust tests passed')
  print(name,': all4 unchanged original Rust doc tests passed')
 if verify_registry(destination,Path(environment['CARGO_HOME']))!=registry:raise SystemExit('registry source changed during test execution')
 evidence={'crate_sha256':MIRROR.CHECKSUM,'source_files':[{'path':path,'sha256':hashlib.sha256(data).hexdigest()} for path,data in sorted(files.items()) if path.startswith('tests/') or path.startswith('benches/')],'compiler':subprocess.check_output([environment['RUSTC'],'-vV'],text=True).strip(),'profiles':profiles,'registry_dependencies':registry,'scope':'Original tests/support bodies unchanged, including Send+Sync compile witness and nested DefaultHasher helper. Criterion benchmark input workloads are mirrored separately in Go; this helper does not replace the implementation owner oracle.'}
 write(destination/'evidence.json',(json.dumps(evidence,indent=2)+'\n').encode())

if __name__=='__main__':main()
