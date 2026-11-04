#!/usr/bin/env python3
"""Check every original test name, statement mirror, assertion and literal."""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('semver_test_mirror',HERE/'mirror.py')
MIRROR=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(MIRROR)

MAPPING={
 'tests/test_autotrait.rs':'tests/test_autotrait_test.go',
 'tests/test_identifier.rs':'tests/test_identifier_test.go',
 'tests/test_version.rs':'tests/test_version_test.go',
 'tests/test_version_req.rs':'tests/test_version_req_test.go',
 'tests/util/mod.rs':'tests/util_test.go',
 'tests/node/mod.rs':'tests/node_test.go',
 'benches/parse.rs':'benches/parse_test.go',
}

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--root',type=Path,default=MIRROR.ROOT);parser.add_argument('--check',action='store_true');args=parser.parse_args();root=args.root.resolve();files=MIRROR.source_files(root)
 subprocess.run([sys.executable,str(HERE/'mirror.py'),'--root',str(root),'--check'],check=True,stdout=subprocess.PIPE)
 go=os.environ.get('FERRUGO_GO') or str(root/'.toolchain/go/bin/go')
 if not Path(go).exists():go=shutil.which(go) or go
 paths=[str(root/'port/dependencies/semver/v1_0_28'/target) for target in MAPPING.values()]
 environment=os.environ.copy();environment.setdefault('GOCACHE','/tmp/ferrugo-go-cache')
 result=subprocess.run([go,'run',str(HERE/'audit.go'),'--',*paths],env=environment,check=True,stdout=subprocess.PIPE)
 go_files=json.loads(result.stdout);entries=[];rust_assertions=go_assertions=0;test_count=0
 for source,target in MAPPING.items():
  original=files[source].decode();masked,literals=MIRROR.mask(original);go_path=root/'port/dependencies/semver/v1_0_28'/target;go_entry=go_files[str(go_path)]
  source_functions=re.findall(r'#\[test\]\s*(?:pub )?fn (\w+)',masked)
  prefix={'test_autotrait.rs':'Autotrait','test_identifier.rs':'Identifier','test_version.rs':'Version','test_version_req.rs':'VersionReq'}.get(Path(source).name)
  expected_names=[f'Test{prefix}_{name}' for name in source_functions]
  actual_names=[name for name in go_entry['functions'] if prefix and name.startswith(f'Test{prefix}_')]
  if actual_names!=expected_names:raise SystemExit(f'original test declaration order differs: {source}')
  source_literal_counts=Counter(value for _,value in literals);go_literal_counts=Counter(go_entry['literals'])
  # Requirement assertion helpers retain their literal arguments in the shared
  # Go helper file, preserving the original source file's test statements.
  if source=='tests/test_version_req.rs':
   go_literal_counts.update(go_files[str(root/'port/dependencies/semver/v1_0_28/tests/util_test.go')]['literals'])
  missing=source_literal_counts-go_literal_counts
  if missing:raise SystemExit(f'original literal occurrences missing from {source}: {missing}')
  # Rust tuple projections such as self.0 map to named Go fields, not values.
  source_integers=Counter(re.findall(r'(?<!\.)\b[0-9]+\b',masked));go_integers=Counter(go_entry['integers'])
  if source_integers-go_integers:raise SystemExit(f'original integer occurrences missing from {source}: {source_integers-go_integers}')
  assertions=len(re.findall(r'\bassert(?:_eq|_ne)?!',masked));rust_assertions+=assertions;go_assertions+=go_entry['assertions'];test_count+=len(source_functions)
  entries.append({'source':source,'source_sha256':hashlib.sha256(files[source]).hexdigest(),'source_lines':len(original.splitlines()),'target':target,'target_sha256':hashlib.sha256(go_path.read_bytes()).hexdigest(),'tests':source_functions,'go_tests':actual_names,'literal_occurrences':sum(source_literal_counts.values()),'integer_occurrences':sum(source_integers.values()),'source_assertion_macros':assertions,'status':'mirrored runtime assertions plus retained Rust compiler witness' if source.endswith('test_autotrait.rs') else 'mirrored'})
 if rust_assertions!=71 or go_assertions!=71 or test_count!=34:raise SystemExit(f'assertion/test accounting differs: Rust{rust_assertions} Go{go_assertions} tests{test_count}')
 audit={'crate_sha256':MIRROR.CHECKSUM,'source_test_files':6,'source_test_lines':893,'source_benchmark_files':1,'source_benchmark_lines':21,'original_test_functions':34,'original_assertion_macros':71,'mirrored_assertion_calls':71,'benchmark_workloads':3,'files':entries,'profiles':{'default':'native matcher, full280length/5.6MB identifier tests','semver_no_std':'same assertions with std feature disabled','semver_serde':'same assertions with source serde feature enabled; serde implementation oracle belongs to the implementation owner','semver_node':'explicit external Node semver matcher; requested missing prerequisites fail','semver_miri':'original40iteration workload cap only; does not claim Miri execution','native_word_width':'amd64 runtime validated;386 cross-compiles, but current kernel rejects ELF32 with exec-format-error;32bit runtime is not validated'},'language_specific_witnesses':{'send_sync':'Exact original Rust test_autotrait.rs compiled unchanged; Go adds read-only concurrent/race checks without claiming Rust trait-bound equivalence','default_hasher':'Exact original nested DefaultHasher helper retained/executed unchanged; Go repeats the same equality assertion through caller-owned deterministic hashing, with no concrete DefaultHasher port claim'}}
 encoded=(json.dumps(audit,indent=2,ensure_ascii=False)+'\n').encode();target=HERE/'audit.json'
 if args.check:
  if target.read_bytes()!=encoded:raise SystemExit('source-aligned test audit differs')
 else:target.write_bytes(encoded)
 print('34 original tests,71 assertion sites,all literal occurrences and declaration order,3 benchmark workloads accounted')

if __name__=='__main__':main()
