#!/usr/bin/env python3
"""Compare guard forwarding with the unchanged official dependency and real std."""
import argparse, hashlib, importlib.util, json, os, pathlib, resource, signal, subprocess, sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}: exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--check',action='store_true');ap.add_argument('--offline',action='store_true');args=ap.parse_args();root=args.root.resolve();env=os.environ.copy();here=root/'tools/port-rust-stable-hash/panic-guards'
 builder=root/'tools/port-rust-def-id/reference-build';spec=importlib.util.spec_from_file_location('stable_panic_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1'
 run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'stable-hash-panic-guards'
 argv=[env['FERRUGO_RUSTC'],'--edition=2024',str(here.relative_to(root)/'oracle.rs'),'-L','dependency='+str(deps)]
 for name in ['rustc_stable_hash','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root)
 cases=[];defaults={}
 for op,fail in [('write',False),('clone',False),('finish',False),('finish',True)]:
  for context in ['normal','inherited','processing']:
   p=subprocess.run([native,op,context,str(fail).lower()],cwd=root,capture_output=True,timeout=10,env=env)
   if p.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native exit {p.returncode}: {p.stderr}')
   events=[];infos={}
   for line in p.stdout.decode().splitlines():
    if line.startswith('E:'):events.append(line[2:])
    elif line.startswith('H:'):
     info=json.loads(line[2:]);events.append(info);infos[info['Payload']]=info
    else:raise RuntimeError('unexpected native stdout:'+line)
   defaults.update({key:value for key,value in infos.items()if key!='outer'})
   cases.append({'Request':{'Operation':op,'Context':context,'DropFail':fail,'Infos':infos},'Exit':134 if p.returncode else 0,'Events':events})
 for c in cases:c['Request']['Infos']={**defaults,**c['Request']['Infos']}
 fixture=root/'port/dependencies/rustc-stable-hash/v0_1_2/src/testdata/panic_guards.json';data=(json.dumps(cases,indent=2)+'\n').encode()
 paths=[here/'oracle.rs',here/'generate.py',*sorted((here/'subject').glob('*.go')),root/'port/dependencies/rustc-stable-hash/v0_1_2/src/ownership.go',root/'port/dependencies/rustc-stable-hash/v0_1_2/src/panic_guard_test.go']
 # Local Cargo output paths are build inputs, not portable provenance. Their
 # source archives/expanded bytes are already verified by the reference builder;
 # retain those source identities and the actual compiler/profile instead.
 portable_profile={k:v for k,v in profile.items()if k not in ['library','link_arguments']}
 report={'dependency':'rustc-stable-hash 0.1.2','archive_sha256':'781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08','native_profile':portable_profile,'cases':len(cases),'aborts':sum(c['Exit']!=0 for c in cases),'fixture_sha256':sha(data),'scope':'Actual borrowed non-Copy H write/clone forwarding, consumed H finish cleanup, caught nested/inherited panic, immediate processing-panic abort, actual compiler cleanup-abort metadata. Source hooks/location/type/flags and exact observable local/state Drop prefixes.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in paths]}
 manifest=here/'validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=data or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('panic guard evidence differs')
 else:fixture.write_bytes(data);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/dependencies/rustc-stable-hash/v0_1_2/src','-run','^Test(OriginalStableHashPanicGuards|StableHashPanicGuardWorker)$','-count=1'],env,root)
 print(f'Stable-hash panic guards: {len(cases)} native cases / {report["aborts"]} aborts passed')
if __name__=='__main__':main()
