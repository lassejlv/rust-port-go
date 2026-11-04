#!/usr/bin/env python3
"""Retain real abort prefixes, never a fabricated post-abort map state."""
import argparse,gzip,hashlib,importlib.util,io,json,os,pathlib,resource,subprocess,tarfile,tempfile,tomllib
HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('owned_support',HERE/'ownership_generate.py');OWNED=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(OWNED)
SUPPORT=OWNED.SUPPORT;ROOT=SUPPORT.ROOT;FIXTURE=ROOT/'port/support/rusthashmap/testdata/aborts.jsonl.gz'
def sha(b):return hashlib.sha256(b).hexdigest()
def no_core():resource.setrlimit(resource.RLIMIT_CORE,(0,0))
def parse_response(raw):
 f=raw.decode().strip().split('\t')
 if len(f)!=9:raise SystemExit('Malformed completed abort-reference response')
 return {'abort':False,'action':f[4],'final':f[5],'counter':int(f[6]),'state':bytes.fromhex(f[7]).decode(),'events':bytes.fromhex(f[8]).decode().split('|')}
def request_line(r):return f'{r["scenario"]}\t{r["ambient"]}\t{r["fail"]}\t{r["second"]}\n'.encode()
def invoke(exe,request):
 result=subprocess.run([str(exe)],input=request_line(request),stdout=subprocess.PIPE,stderr=subprocess.PIPE,preexec_fn=no_core,timeout=10)
 events=[bytes.fromhex(line.split(b'\t',1)[1].decode()).decode()for line in result.stderr.splitlines()if line.startswith(b'PROBE_EVENT\t')]
 if result.returncode==-6:
  if result.stdout:raise SystemExit('Aborted source unexpectedly emitted completed state')
  return {'abort':True,'events':events,'reference_exit':'SIGABRT'}
 if result.returncode:raise SystemExit('Unexpected original process failure:'+str(result.returncode))
 response=parse_response(result.stdout)
 if response['events']!=events:raise SystemExit('Completed source telemetry differs')
 return response
def requests_for(exe):
 baseline_requests=''.join(f'{s}\tnormal\t-1\n'for s in range(28)).encode()
 baseline=OWNED.execute(exe,baseline_requests)
 requests=[];seen=set()
 for row in baseline.decode().splitlines():
  fields=row.split('\t');scenario=int(fields[0]);count=int(fields[5])
  requests.extend({'scenario':scenario,'ambient':'processing_panic','fail':fail,'second':-1}for fail in [-1,*range(count)])
  if scenario==24:
   requests.append({'scenario':24,'ambient':'normal','fail':0,'second':-1});continue
  singles=''.join(f'{scenario}\tnormal\t{fail}\n'for fail in range(count)).encode()
  for one in OWNED.execute(exe,singles).decode().splitlines():
   f=one.split('\t');first=int(f[2]);events=[e for e in bytes.fromhex(f[7]).decode().split('|')if e!='final']
   for second in range(first+1,len(events)):
    if not events[second].endswith(':true'):continue
    identity=(scenario,events[first].split(':')[0],events[second].split(':')[0])
    if identity in seen:continue
    seen.add(identity);requests.append({'scenario':scenario,'ambient':'normal','fail':first,'second':second})
 return requests
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 lock=tomllib.loads(subprocess.check_output(['git','-C',str(ROOT/'.toolchain/upstream-rust'),'show',SUPPORT.PIN+':Cargo.lock'],env=dict(os.environ,GIT_NO_LAZY_FETCH='1'),text=True))
 expected=next(p['checksum']for p in lock['package']if p['name']=='rustc-hash'and p['version']=='2.1.1')
 archives=list((pathlib.Path(os.environ.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home'))/'registry/cache').glob('*/rustc-hash-2.1.1.crate'))
 if len(archives)!=1 or archives[0].is_symlink()or sha(archives[0].read_bytes())!=expected:raise SystemExit('Pinned source archive differs')
 proof=None;requests=None;profiles=[]
 with tempfile.TemporaryDirectory(prefix='ferrugo-map-abort-')as temporary:
  tmp=pathlib.Path(temporary)
  with tarfile.open(fileobj=io.BytesIO(archives[0].read_bytes()))as archive:
   for member in archive.getmembers():
    p=pathlib.PurePosixPath(member.name)
    if p.is_absolute()or'..'in p.parts or not(member.isdir()or member.isfile()):raise SystemExit('Invalid archive member')
   archive.extractall(tmp,filter='data')
  for index,(name,fallback,version)in enumerate(SUPPORT.PROFILES):
   rustc=SUPPORT.compiler(os.environ.get('FERRUGO_RUSTC'if index==0 else'FERRUGO_RUSTC_NIGHTLY'),fallback)
   if subprocess.check_output([rustc,'--version'],text=True).strip()!=version:raise SystemExit('Reference compiler differs')
   dest=tmp/name;dest.mkdir();exe=OWNED.build(rustc,tmp/'rustc-hash-2.1.1',dest)
   current_requests=requests_for(exe)
   records=[{'request':request,'response':invoke(exe,request)}for request in current_requests]
   current=b''.join((json.dumps(record,separators=(',',':'),sort_keys=True)+'\n').encode()for record in records)
   if proof is not None and(proof!=current or requests!=current_requests):raise SystemExit('Actual abort profiles differ')
   proof,requests=current,current_requests
   profiles.append({'name':name,'compiler':subprocess.check_output([rustc,'-vV'],text=True).strip(),'cases':len(records),'aborts':sum(r['response']['abort']for r in records),'raw_sha256':sha(current)})
 compressed=gzip.compress(proof,mtime=0)
 if FIXTURE.exists()and gzip.decompress(FIXTURE.read_bytes())==proof:compressed=FIXTURE.read_bytes()
 manifest={'scope':'Actual std process abort prefixes: in-progress panic-hook failures and escaping second panics. Aborted responses contain no fabricated postmortem state. Scoped support only; no std/Unord wholefile completion credit.','profiles':profiles,'cases':len(requests),'fixture_sha256':sha(compressed),'raw_sha256':sha(proof),'rustc_hash_archive_sha256':expected,'oracle_sha256':sha((HERE/'ownership_oracle.rs').read_bytes()),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'owned_generator_sha256':sha((HERE/'ownership_generate.py').read_bytes()),'support_generator_sha256':sha((HERE/'generate.py').read_bytes())}
 metadata=HERE/'abort_manifest.json'
 if args.check:
  if json.loads(metadata.read_text())!=manifest or FIXTURE.read_bytes()!=compressed:raise SystemExit('Actual abort proof differs')
 else:FIXTURE.parent.mkdir(parents=True,exist_ok=True);FIXTURE.write_bytes(compressed);metadata.write_text(json.dumps(manifest,indent=2)+'\n')
 print(str(manifest['cases'])+' actual process cases;'+str(profiles[0]['aborts'])+' observed SIGABRT prefixes per profile')
if __name__=='__main__':main()
