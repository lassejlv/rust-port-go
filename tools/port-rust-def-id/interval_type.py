#!/usr/bin/env python3
"""Canonical def-id Step type-name witnesses through unchanged whole crates."""
import argparse,hashlib,importlib.util,json,os,pathlib,subprocess,tempfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
def sha(value):return hashlib.sha256(value).hexdigest()
def module(name,path):
 spec=importlib.util.spec_from_file_location(name,path);value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--check',action='store_true');ap.add_argument('--offline',action='store_true',help='Reference preparation is always offline');a=ap.parse_args();root=a.root.resolve();here=root/'tools/port-rust-def-id'
 # Verify the complete original crate graph, archives, compiler and upstream tests.
 reference=module('def_id_interval_whole_reference',root/'tools/port-rust-fatal-error/reference.py');profile,env,_=reference.load(True,True)
 selected=module('def_id_interval_tool_selection',here/'reference-build/prepare.py');go=selected.selected(root,'FERRUGO_GO',pathlib.Path('.toolchain/go/bin/go'),'go');env.setdefault('GOCACHE','/tmp/ferrugo-go-cache')
 source=here/'interval_type_oracle.rs'
 with tempfile.TemporaryDirectory(prefix='ferrugo-def-id-interval-')as directory:
  executable=pathlib.Path(directory)/'original';p=subprocess.run([profile['rustc'],str(source.relative_to(root)),*profile['link_arguments'],'-o',str(executable)],cwd=root,env=env,capture_output=True,timeout=180)
  if p.returncode:raise RuntimeError('canonical original observer build failed:'+p.stderr.decode(errors='replace')[-4000:])
  p=subprocess.run([executable],cwd=root,env=env,capture_output=True,timeout=20)
  if p.returncode:raise RuntimeError('canonical original observer exited:'+str(p.returncode))
  raw=p.stdout
 rows=[]
 for line in raw.decode().splitlines():
  kind,domain,name,compact,pretty=line.split('\t');rows.append(dict(Kind=kind,Domain=int(domain),TypeName=name,Compact=bytes.fromhex(compact).decode(),Pretty=bytes.fromhex(pretty).decode()))
 if len(rows)!=9 or {r['TypeName']for r in rows}!={'rustc_span::def_id::DefIndex','rustc_span::def_id::CrateNum','rustc_span::AttrId'}:raise RuntimeError('canonical type names/count differ')
 fixture=root/'port/rust/compiler/rustc_span/src/testdata/def_id_interval_types.json';data=(json.dumps(rows,indent=2)+'\n').encode();files=[here/'interval_type.py',source,root/'port/rust/compiler/rustc_span/src/def_id_index.go',root/'port/rust/compiler/rustc_span/src/lib_attr_step.go',root/'port/rust/compiler/rustc_span/src/def_id_interval_type_test.go']
 proof={'status':'partial-def-id-canonical-Step-witness','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'unicode':profile['unicode'],'reference_provenance_sha256':profile['provenance_sha256'],'cases':len(rows),'raw_sha256':sha(raw),'fixture_sha256':sha(data),'scope':'Actual type_name::<DefIndex/CrateNum/AttrId> and inherited IntervalSet derived Debug through the whole unchanged canonical compiler crates. No facade crate-name substitution; no shared index algorithm edits. Full def_id remains pending its dependencies.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'interval_type_validation.json';encoded=(json.dumps(proof,indent=2)+'\n').encode()
 if a.check:
  if not fixture.exists()or fixture.read_bytes()!=data or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('canonical def-id interval evidence differs')
 else:fixture.write_bytes(data);manifest.write_bytes(encoded)
 p=subprocess.run([go,'test','./port/rust/compiler/rustc_span/src','-run','^TestDefIdCanonicalIntervalTypes$','-count=1'],cwd=root,env=env,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError('Go canonical interval regression failed:'+p.stdout.decode(errors='replace')[-4000:]+p.stderr.decode(errors='replace')[-4000:])
 print('Canonical def-id IntervalSet witnesses: 9 whole-crate original cases passed')
if __name__=='__main__':main()
