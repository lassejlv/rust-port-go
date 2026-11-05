#!/usr/bin/env python3
"""Original borrowed cursor, mutable-borrow and Debug text comparisons."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,random,subprocess,sys
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[1]
def sha(data):return hashlib.sha256(data).hexdigest()
def run(args,env,root,data=None):
 p=subprocess.run(list(map(str,args)),cwd=root,env=env,input=data,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}:exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def requests():
 rng=random.Random(0x1eaf);qs=[]
 for n in [0,1,2,3,7,8,15,16,64,100]:
  for kind in ['iter','keys','values','iter_mut','values_mut']:
   for seed in range(8):
    mutable=kind.endswith('_mut');names=['next','back','nth','nth_back','debug','len']+([]if mutable else['clone_from','count','last'])+(['slice']if kind in ['iter','iter_mut']else[]);steps=[]
    for i in range(20):steps.append(dict(op=rng.choice(names),n=rng.choice([0,1,2,5,12,(1<<64)-1]),mode=rng.randrange(4),delta=rng.randrange(-50,50)))
    if mutable:steps.append(dict(op='into_slice'if kind=='iter_mut'and seed%3==2 else'count'if seed%2==0 else'last',delta=3))
    qs.append(dict(n=n,kind=kind,steps=steps))
   if kind.endswith('_mut'):
    for terminal in ['count','last','into_slice'if kind=='iter_mut'else'count']:qs.append(dict(n=n,kind=kind,steps=[dict(op='next',delta=1),dict(op=terminal,delta=7)]))
 return qs+[{**q,'default':True}for q in qs if q['n']==7]
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--check',action='store_true');args=ap.parse_args();root=args.root.resolve();here=root/'tools/port-rust-indexmap';builder=root/'tools/port-rust-def-id/reference-build';env=os.environ.copy()
 spec=importlib.util.spec_from_file_location('indexmap_cursor_builder',builder/'prepare.py');support=importlib.util.module_from_spec(spec);spec.loader.exec_module(support)
 for key,local,fallback in [('FERRUGO_RUSTC','.toolchain/rust/bin/rustc','rustc'),('FERRUGO_CARGO','.toolchain/rust/bin/cargo','cargo'),('FERRUGO_GO','.toolchain/go/bin/go','go')]:env[key]=support.selected(root,key,pathlib.Path(local),fallback)
 env.setdefault('GOCACHE','/tmp/ferrugo-go-cache');env['RUSTC_BOOTSTRAP']='1';run([sys.executable,builder/'prepare.py','--root',root,'--debug','true','--overflow','true',*(['--offline']if args.offline else[])],env,root)
 cache=root/'.toolchain/oracles/def-id-reference';profile=json.loads((cache/'build-profile.json').read_text());deps=pathlib.Path(profile['library']).parent/'deps';native=cache/'indexmap-borrowed-iterator';subject=cache/'indexmap-borrowed-iterator-go';argv=[env['FERRUGO_RUSTC'],'--edition=2024',str((here/'iterator_oracle.rs').relative_to(root)),'-L','dependency='+str(deps)]
 for name in ['indexmap','serde_json']:
  found=list(deps.glob('lib'+name+'-*.rlib'))
  if len(found)!=1:raise RuntimeError('ambiguous native artifact:'+name)
  argv+=['--extern',name+'='+str(found[0])]
 run([*argv,'-o',native],env,root);run([env['FERRUGO_GO'],'build','-o',subject,'./tools/port-rust-indexmap/iterator-replay'],env,root)
 qs=requests();data=''.join(json.dumps(q,separators=(',',':'))+'\n'for q in qs).encode();raw=run([native],env,root,data);actual=run([subject],env,root,data);a=list(map(json.loads,raw.splitlines()));b=list(map(json.loads,actual.splitlines()))
 if len(a)!=len(qs)or len(b)!=len(qs):raise RuntimeError('cursor record count differs')
 for n,(expected,got)in enumerate(zip(a,b)):
  if expected!=got:raise RuntimeError(f'cursor mismatch case{n}\n{json.dumps(expected)}\n{json.dumps(got)}')
 fixture=root/'port/support/rustindexmap/testdata/borrowed_iterator.jsonl.gz';compressed=fixture.read_bytes()if fixture.exists()and gzip.decompress(fixture.read_bytes())==raw else gzip.compress(raw,mtime=0,compresslevel=9)
 files=[here/'iterator_generate.py',here/'iterator_oracle.rs',*sorted((here/'subject').glob('*.go')),*sorted((here/'iterator-replay').glob('*.go')),*sorted((root/'port/support/rustindexmap').glob('*.go'))]
 report={'status':'partial-supporting-IndexMap-borrowed-iterators','whole_dependency_port':False,'native_profile':{k:v for k,v in profile.items()if k not in ['library','link_arguments']},'cases':len(qs),'operations':sum(len(q['steps'])for q in qs),'request_sha256':sha(data),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'scope':'Iter/IterMut/Keys/Values/ValuesMut next/back/nth/nth_back, source count/last overrides, exact hints, borrowed Clone/provided CloneFrom/Default, remaining slices, consumed mutable slice and mutations through real references, numeric formatting flags. Generic fold/collect and additional inherited APIs are outside this borrowed-iterator corpus.','files':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())}for p in files]};manifest=here/'iterator_validation.json';encoded=(json.dumps(report,indent=2)+'\n').encode()
 if args.check:
  if not fixture.exists()or fixture.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('cursor evidence differs')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 run([env['FERRUGO_GO'],'test','./port/support/rustindexmap','-run','^TestOriginalIndexMapBorrowedIterators$','-count=1'],env,root);print(f'IndexMap borrowed cursors: {len(qs)} cases /{report["operations"]} operations passed; no whole dependency credit')
if __name__=='__main__':main()
