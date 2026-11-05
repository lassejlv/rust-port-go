#!/usr/bin/env python3
"""Scoped actual owning String methods, exact source SDKs1.99 and1.101."""
import argparse,gzip,hashlib,importlib.util,json,os,pathlib,random,subprocess,tempfile,tarfile
import prepare
ROOT=prepare.ROOT;HERE=prepare.HERE

def requests():
 rows=[]
 words=['','a','é','🦀','a\0b','x'*33,'ab\n\tΣ\U0001cfaf']
 rng=random.Random(0x535452199)
 for factory in ['from','with']:
  for cap in [0,1,3,4,7,8,9,16,31,32,63,64]:
   for word in words:
    live=[True,False,False];ops=[]
    for _ in range(32):
     active=[i for i,v in enumerate(live)if v];slot=rng.choice(active);choice=rng.choice(['push','char','reserve','exact','clone','clonefrom','move'])
     if choice in ['push','char','reserve','exact']:
      value=rng.choice(words).encode().hex()if choice=='push'else str(rng.choice([0,32,233,0x1cfaf,0x1f980]))if choice=='char'else str(rng.choice([0,1,2,7,8,13,31,64,128]))
      ops.append(f'{choice}:{slot}:{value}')
     else:
      dest=rng.choice([i for i in range(3)if i!=slot])
      if choice=='clonefrom'and not live[dest]:choice='clone'
      ops.append(f'{choice}:{dest}:{slot}');live[dest]=True
      if choice=='move':live[slot]=False
    slot=next(i for i,v in enumerate(live)if v);ops.append(f'reserve:{slot}:{(1<<64)-1}');ops.append(f'exact:{slot}:{(1<<64)-1}')
    rows.append({'factory':factory,'capacity':cap,'text':word,'ops':ops})
 return rows

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--check',action='store_true');p.add_argument('--rustc-199');p.add_argument('--rustc-1101');a=p.parse_args()
 selected199=a.rustc_199 if a.rustc_199 is not None else os.environ.get('FERRUGO_STRING_RUSTC_199',os.environ.get('FERRUGO_RUSTC',str(ROOT/'.toolchain/rust/bin/rustc')))
 old=os.environ.get('FERRUGO_RUSTC');os.environ['FERRUGO_RUSTC']=selected199
 try:profile,env,_=prepare.prepare()
 finally:
  if old is None:os.environ.pop('FERRUGO_RUSTC',None)
  else:os.environ['FERRUGO_RUSTC']=old
 c199=env['RUSTC']
 spec=importlib.util.spec_from_file_location('owned_string_sdk1101',ROOT/'tools/port-rust-str/prepare.py');sdk=importlib.util.module_from_spec(spec);spec.loader.exec_module(sdk);sdk_source,cache=sdk.verified()
 expected=json.loads((HERE/'sdk1101.json').read_text());archive=cache/'rust-src-nightly.tar.xz'
 if prepare.sha(archive.read_bytes())!=expected['archive_sha256']:raise RuntimeError('String1.101 archive differs')
 with tarfile.open(archive,'r:xz')as t:
  for record in expected['files']:
   matches=[m for m in t.getmembers()if m.isfile()and m.name.endswith('/'+record['path'])]
   if len(matches)!=1 or prepare.sha(t.extractfile(matches[0]).read())!=record['sha256']:raise RuntimeError('String1.101 used source differs')
 c1101,v1101=sdk.selected(a.rustc_1101 if a.rustc_1101 is not None else os.environ.get('FERRUGO_STRING_RUSTC_1101',str(ROOT/'.toolchain/rust-nightly-2026-11-04/bin/rustc')))
 versions=[(c199,profile['rustc_version'],'1.99'),(c1101,v1101,'1.101')]
 if subprocess.check_output([c199,'--version'],text=True).strip()!=profile['rustc_version']:raise RuntimeError('String1.99 selected compiler differs')
 qs=requests();wire=''.join(q['factory']+'\t'+str(q['capacity'])+'\t'+q['text'].encode().hex()+'\t'+';'.join(q['ops'])+'\n'for q in qs).encode();rows=[]
 for compiler,version,sdk_name in versions:
  for debug in [False,True]:
   for overflow in [False,True]:
    with tempfile.TemporaryDirectory(prefix='ferrugo-owning-string-')as directory:
     binary=pathlib.Path(directory)/'original'
     subprocess.run([compiler,str(HERE/'std_string.rs'),'-Cdebug-assertions='+str(debug).lower(),'-Coverflow-checks='+str(overflow).lower(),'-o',str(binary)],env=env,cwd=ROOT,check=True,timeout=60)
     actual=subprocess.run([str(binary)],input=wire,stdout=subprocess.PIPE,env=env,cwd=ROOT,check=True,timeout=30).stdout.decode().splitlines()
     if len(actual)!=len(qs):raise RuntimeError('String native case count differs')
     for q,line in zip(qs,actual):rows.append({'sdk':sdk_name,'profile':f'd{int(debug)}o{int(overflow)}','request':q,'want':bytes.fromhex(line).decode()})
 raw=b''.join((json.dumps(row,ensure_ascii=True,separators=(',',':'))+'\n').encode()for row in rows);dest=ROOT/'port/support/ruststr/testdata/owned-string.jsonl.gz';compressed=gzip.compress(raw,mtime=0)
 if dest.exists()and gzip.decompress(dest.read_bytes())==raw:compressed=dest.read_bytes()
 files=[HERE/name for name in ['generate_string.py','std_string.rs','sdk199.json','sdk1101.json','prepare.py']]+sorted((ROOT/'port/support/ruststr').glob('owned_string*.go'))
 proof={'schema':1,'scope':'Scoped genuine std String methods only; both exact SDKs; no whole String source-file credit or allocator/ABI identity','profiles':[{'sdk':name,'compiler':version.strip()}for _,version,name in versions],'records':len(rows),'operations':sum(len(q['ops'])for q in qs)*8,'source199':profile['owned_string_sdk'],'source1101':expected,'raw_sha256':prepare.sha(raw),'fixture_sha256':prepare.sha(compressed),'files':{str(p.relative_to(ROOT)):prepare.sha(p.read_bytes())for p in files}}
 encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode();manifest=HERE/'string-manifest.json'
 if a.check:
  if dest.read_bytes()!=compressed or manifest.read_bytes()!=encoded:raise RuntimeError('String evidence differs')
 else:dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(compressed);manifest.write_bytes(encoded)
 print(len(rows),'actual std String scripts;',proof['operations'],'operations; both SDKs/four flag profiles')
if __name__=='__main__':main()
