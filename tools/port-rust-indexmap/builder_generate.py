#!/usr/bin/env python3
"""Unchanged IndexMap generic constructor/state/conditional trait comparison."""
import argparse,gzip,hashlib,importlib.util,itertools,json,os,pathlib,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 qs=[]
 for cap,scap,scope in itertools.product([0,3,7,40],[0,7,40],['none','A','B']):
  for kind in ['opaque','building']:qs.append(dict(capacity=cap,source_capacity=scap,scope=scope,kind=kind,mode='construct',failure='none'))
  for kind in ['default','custom']:
   for mode in ['clone','clone_from']:
    fails=['none','clone']if mode=='clone'else(['none','clone','drop']if kind=='default'else['none','clone_from','clone'])
    for failure in fails:qs.append(dict(capacity=cap,source_capacity=scap,scope=scope,kind=kind,mode=mode,failure=failure))
 return qs
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('indexmap_builder_states',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'indexmap-builders';subject=cache/'indexmap-builders-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here/'builder_oracle.rs').relative_to(root)),'-L','dependency='+str(deps)]
 artifact={}
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  artifact[name]=found[0];argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/builder-replay'],env,root)
 ui=subprocess.run([env['FERRUGO_RUSTC'],'--edition=2024',str((here/'builder_no_clone.rs').relative_to(root)),'--extern','indexmap='+str(artifact['indexmap']),'-L','dependency='+str(deps),'-o',str(cache/'builder-invalid')],cwd=root,env=env,capture_output=True)
 if ui.returncode==0 or b'E0599'not in ui.stderr or b'Clone'not in ui.stderr:raise RuntimeError('native unavailable Clone bound was not rejected')
 qs=requests();data=''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode();raw=run([native],env,root,data);actual=run([subject],env,root,data);a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('builder record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'builder mismatch case{n}\n{json.dumps(expected)}\n{json.dumps(got)}')
 fixture=root/'port/support/rustindexmap/testdata/builder.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'builder_generate.py',here/'builder_oracle.rs',here/'builder_no_clone.rs',*sorted((here/'subject').glob('*.go')),*sorted((here/'builder-replay').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go')),root/'port/support/rustvec/replace.go']
 report={'status':'partial-supporting-IndexMap-generic-builder','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'cases':len(qs),'native_ui_rejections':1,'request_sha256':sha(data),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Actual owned arbitrary non-Copy S with independently conditional BuildHasher/Clone/CloneFrom: noHash/noEq keys and nonBuildHasher/nonClone S constructors, noClone H, deep clone and real shared-state interior mutation, default replacement and custom partial CloneFrom panic, A/B/no-observer ambient metadata. Existing core/Entry/borrowed iterator scope remains partial.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'builder_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('builder evidence differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','-run','^Test(OriginalIndexMapBuilderStates|IndexMapUnconstrainedConstructor)','-count=1'],env,root);print(f'IndexMap owned builder: {len(qs)} original cases +native Clone-bound rejection passed')
if __name__=='__main__':main()
