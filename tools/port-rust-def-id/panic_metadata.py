#!/usr/bin/env python3
"""Original DefID/macro/used-default fresh panic payload and location proof."""
import argparse,hashlib,importlib.util,json,os,pathlib,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-def-id';builder=here/'reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('defid_panic_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1'
 run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'def-id-panic-metadata';found=list(deps.glob('libserde_json-*.rlib'))
 if len(found)!=1:raise RuntimeError('ambiguous serde_json artifact')
 run([env['FERRUGO_RUSTC'],'--edition=2024',str(here.relative_to(root)/'panic_metadata_oracle.rs'),*profile['link_arguments'],'--extern','serde_json='+str(found[0]),'-o',native],env,root)
 names=['crate_u32','def_u32','crate_usize','def_usize','local_idx_limit','local_idx_overflow','crate_idx_overflow','def_idx_overflow','crate_idx_limit','def_idx_limit','crate_add_overflow','def_add_overflow','crate_add_limit','def_add_limit','crate_forward','def_forward','crate_backward','def_backward','expect_local','expect_partial','mod_expect_local','format_error'];requests=''.join(json.dumps({'op':op})+'\n'for op in names).encode();raw=run([native],env,root,requests);rows=list(map(json.loads,raw.splitlines()))
 if len(rows)!=len(names)or any(not row['failed']or len(row['hooks'])!=1 for row in rows):raise RuntimeError('incomplete native panic records')
 fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_panic_metadata.jsonl';production=root/'port/rust/compiler/rustc_span/src';files=[here/'panic_metadata.py',here/'panic_metadata_oracle.rs',*[production/name for name in ['def_id_panic.go','def_id_index.go','def_id_debug.go','def_id_panic_metadata_test.go']]]
 report={'status':'partial-DefID-fresh-panic-profile','native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'records':len(rows),'request_sha256':sha(requests),'fixture_sha256':sha(raw),'scope':'Actual whole pinned source/macros and used Idx/Step/alloc formatting defaults: panic payload text/dynamic type/unwind flags and actual selected profile locations. Explicit profiles preserve constructor macro locations and expect_local tracked callers; absent profiles retain nil locations. No fabricated Rust paths.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'panic_metadata_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=raw or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('DefID panic metadata evidence differs')
 else:fixture.write_bytes(raw);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/rust/compiler/rustc_span/src','-run','^TestDefIdOriginalPanicMetadata$','-count=1'],env,root)
 print(f'DefID fresh panic metadata: {len(rows)} original records passed')
if __name__=='__main__':main()
