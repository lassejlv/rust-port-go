#!/usr/bin/env python3
"""Original eager key/value projection and owning cursor comparisons."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 return [dict(kind=kind,n=n,front=front,back=back,other=other,op=op,nth=nth,fail='',inherited=inherit)for kind in ['keys','values']for n in [0,1,3,7,14]for front,back in [(0,0),(1,0),(0,1),(1,1)]for other in [0,3]for op in ['next','back','nth','nth_back','count','last','close','clone','clone_from','inspect']for nth in ([0,1,2,(1<<64)-1]if op in ['nth','nth_back']else[0])for inherit in [False,True]]
def encode(qs):return ''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode()
def compare(native,subject,qs,env,root):
 raw=run([native],env,root,encode(qs));actual=run([subject],env,root,encode(qs));a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('owning iterator record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'owning iterator mismatch case{n}\n{json.dumps(expected)}\n{json.dumps(got)}')
 return raw,a
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('indexmap_projected_iterator',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'indexmap-projected-iterator';subject=cache/'indexmap-projected-iterator-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here/'projection_oracle.rs').relative_to(root)),'-L','dependency='+str(deps)]
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root)
 ui=cache/'projection-clone-bounds';run([env['FERRUGO_RUSTC'],'--edition=2024',str((here/'projection_clone_bounds.rs').relative_to(root)),'-L','dependency='+str(deps),'--extern','indexmap='+str(list(deps.glob('libindexmap-*.rlib'))[0]),'-o',ui],env,root);run([ui],env,root)
 run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/projection-replay'],env,root)
 qs=requests();raw,rows=compare(native,subject,qs,env,root);failures=[{**row['request'],'fail':event}for row in rows for event in dict.fromkeys(row['result']['events'])];failure_raw,observed=compare(native,subject,failures,env,root);raw+=failure_raw
 fixture=root/'port/support/rustindexmap/testdata/projection.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'projection_generate.py',here/'projection_oracle.rs',here/'projection_clone_bounds.rs',*sorted((here/'subject').glob('*.go')),*sorted((here/'projection-replay').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go')),root/'port/support/rustvec/replace.go']
 report={'status':'partial-supporting-IndexMap-projected-iterators','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'baseline_cases':len(qs),'one_shot_failures':len(failures),'caught_panics':sum(row['result']['panic']is not None for row in observed),'request_sha256':sha(encode(qs+failures)),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Actual nonCopy K/V and nonClone S/H, eager opposite-component Drop, in-place MaybeUninit storage, prepared retained-field leak on discarded-field Drop panic, prefix-before-input-tail cleanup, typed Clone bounds without Clone on discarded field, cursor state, Count/Last leaks, Clone/default CloneFrom, normal/inherited contexts and one-shot callback failures. Generic fold/collect and additional inherited APIs are outside this projected-iterator corpus.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'projection_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('owning iterator evidence differs')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','-run','^TestOriginalIndexMapProjectedIterators$','-count=1'],env,root);print(f'IndexMap projected iterators: {len(qs)} original baseline +{len(failures)} failure cases passed; no whole dependency credit')
if __name__=='__main__':main()
