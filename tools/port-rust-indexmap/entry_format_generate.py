#!/usr/bin/env python3
"""Original entry format mutation/cleanup/clone/consumption comparisons."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 return [dict(kind=kind,format=f,chars=c,fail=-1)for kind in ['entry_occupied','entry_vacant','occupied','vacant','indexed']for f in range(12)for c in[False,True]]
def encode(qs):return ''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode()
def compare(native,subject,qs,env,root):
 raw=run([native],env,root,encode(qs));actual=run([subject],env,root,encode(qs));a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('entry format record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'entry format mismatch case{n}\n{json.dumps(expected)}\n{json.dumps(got)}')
 return raw,a
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('indexmap_entry_format',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'indexmap-entry-format';subject=cache/'indexmap-entry-format-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here/'entry_format_oracle.rs').relative_to(root)),'-L','dependency='+str(deps)]
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/entry-format-replay'],env,root)
 ui=cache/'indexmap-entry-format-bounds';run([env['FERRUGO_RUSTC'],'--edition=2024',str((here/'entry_format_bounds.rs').relative_to(root)),'-L','dependency='+str(deps),'--extern','indexmap='+str(next(deps.glob('libindexmap-*.rlib'))),'-o',ui],env,root);run([ui],env,root)
 rejected=subprocess.run([env['FERRUGO_RUSTC'],'--edition=2024','--cfg','entry_enum',str((here/'entry_format_bounds.rs').relative_to(root)),'-L','dependency='+str(deps),'--extern','indexmap='+str(next(deps.glob('libindexmap-*.rlib'))),'-o',ui],cwd=root,env=env,capture_output=True)
 if rejected.returncode==0 or b'`Undebug`'not in rejected.stderr or b'Debug'not in rejected.stderr:raise RuntimeError('source Entry enum Debug bound was not rejected')
 qs=requests();raw,rows=compare(native,subject,qs,env,root);failures=[{**row['request'],'fail':cut}for row in rows for cut in range(len(row['result']['calls'])+1)];failure_raw,observed=compare(native,subject,failures,env,root);raw+=failure_raw
 fixture=root/'port/support/rustindexmap/testdata/entry_format.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'entry_format_generate.py',here/'entry_format_oracle.rs',here/'entry_format_bounds.rs',*sorted((here/'subject').glob('*.go')),*sorted((here/'entry-format-replay').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go')),root/'port/support/rustvec/replace.go']
 report={'status':'partial-supporting-IndexMap-entry-format','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'baseline_cases':len(qs),'positive_trait_bound_programs':1,'rejected_trait_bound_programs':1,'one_shot_failures':len(failures),'caught_panics':sum(row['result']['failed']for row in observed),'request_sha256':sha(encode(qs+failures)),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Actual Entry enum, OccupiedEntry, VacantEntry and IndexedEntry Debug struct/tuple shapes through unchanged crate; full compact/pretty/radix/sign/zero/alignment/fill/precision options, independent string/character writes, every writer failure and mutable caller formatter post-state. Trait bounds checked separately. Partial supporting API.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'entry_format_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('entry format evidence differs')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','-run','^Test(OriginalIndexMapEntryFormatting|VacantEntryDebugDoesNotRequireValueDebug)$','-count=1'],env,root);print(f'IndexMap entry format: {len(qs)} original baseline +{len(failures)} failure cases passed; no whole dependency credit')
if __name__=='__main__':main()
