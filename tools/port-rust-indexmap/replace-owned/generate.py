#!/usr/bin/env python3
"""Native std drop-and-replace/CloneFrom cleanup with actual source metadata."""
import argparse,hashlib,importlib.util,json,os,pathlib,resource,signal,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-indexmap/replace-owned';builder=root/'tools/port-rust-def-id/reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('replace_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';found=list(deps.glob('libserde_json-*.rlib'))
 if len(found)!=1:raise RuntimeError('ambiguous serde_json artifact')
 native=cache/'replace-owned-rust';run([env['FERRUGO_RUSTC'],'--edition=2024',str((here.parent/'replace_owned_oracle.rs').relative_to(root)),'--extern','serde_json='+str(found[0]),'-L','dependency='+str(deps),'-o',native],env,root)
 cases=[];defaults={};context_defaults={}
 for operation in ['assign','clone_from']:
  for context in ['normal','inherited','processing']:
   for observer in ['none','A','B']:
    for failure in ['none','body','field','both']:
     p=subprocess.run([native,operation,context,observer,failure],env=env,cwd=root,capture_output=True,timeout=10)
     if p.returncode not in [0,-signal.SIGABRT]:raise RuntimeError(f'unexpected native status {p.returncode}: {p.stderr}')
     events=[];infos={}
     for line in p.stdout.decode().splitlines():
      if line.startswith('E:'):events.append(line[2:])
      elif line.startswith('H:'):
       info=json.loads(line[2:]);events.append(info);infos[info['Payload']]=info
      else:raise RuntimeError('unexpected native stdout:'+line)
     defaults.update(infos);context_defaults.setdefault(context,{}).update(infos)
     cases.append({'Request':{'Operation':operation,'Context':context,'Observer':observer,'Failure':failure,'Infos':infos},'Exit':134 if p.returncode else 0,'Events':events})
 for row in cases:row['Request']['Infos']={**defaults,**context_defaults[row['Request']['Context']],**row['Request']['Infos']}
 fixture=root/'port/support/rustvec/testdata/replace_owned.json';data=(json.dumps(cases,indent=2)+'\n').encode();files=[here/'generate.py',here.parent/'replace_owned_oracle.rs',*sorted((here/'subject').glob('*.go')),*[root/'port/support/rustvec'/name for name in ['replace.go','replace_test.go','replace_oracle_test.go']]]
 report={'status':'supporting-used-drop-and-replace','native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'cases':len(cases),'actual_native_sigabrt':sum(row['Exit']!=0 for row in cases),'fixture_sha256':sha(data),'scope':'Actual assignment and Clone::clone_from provided-default in the selected std profile: same destination cell, complete partial old Drop/field cleanup, installation after old Drop panic, immediate processing-panic abort, second cleanup panic, inherited context and A/B/no-observer ambient metadata. Supplemental Go dynamic destructor transfer/consumed-owner invariants; not an entire std assignment/ownership/ABI port.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=data or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('replace-owned evidence differs')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(data);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustvec','-run','^Test(OriginalReplaceOwned|ReplaceOwned)','-count=1'],env,root)
 print(f'Drop-and-replace: {len(cases)} native cases /{report["actual_native_sigabrt"]} actual SIGABRT passed')
if __name__=='__main__':main()
