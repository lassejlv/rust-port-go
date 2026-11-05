#!/usr/bin/env python3
"""Retain exact original Fingerprint decode panic metadata without changing its main corpus."""
import argparse,gzip,hashlib,io,json,os,resource,shutil,subprocess,sys,tempfile
from pathlib import Path

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
PIN='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
COMPILER='58cf8f9c35b355199727627d07ae34a4e2846d67'
LIMIT=1<<20
DECODE_SITE='rustc_data_structures::Fingerprint::decode.raw_length'

def sha(data):return hashlib.sha256(data).hexdigest()
def executable(flag,env_name,default,root):
 value=flag if flag is not None else os.environ.get(env_name)
 if value is None:value=str(root/default)if(root/default).is_file()else Path(default).name
 if not value:raise RuntimeError('empty executable override: '+env_name)
 chosen=os.path.abspath(root/value)if'/'in value else shutil.which(value)
 if not chosen or not os.path.isfile(chosen)or not os.access(chosen,os.X_OK):raise RuntimeError('unavailable executable: '+value)
 return chosen

def run(command,**kwargs):
 result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180,**kwargs)
 if result.returncode:raise RuntimeError(f'command failed: {command}\n{result.stderr.decode(errors="replace")}')
 return result.stdout

def compressed(raw,path):
 if len(raw)>LIMIT:raise RuntimeError('decode metadata exceeds bound')
 if path.is_symlink():raise RuntimeError('symlink decode metadata fixture')
 if path.exists()and path.stat().st_size<=LIMIT:
  previous=path.read_bytes()
  try:
   with gzip.GzipFile(fileobj=io.BytesIO(previous))as stream:decoded=stream.read(LIMIT+1)
   if decoded==raw:return previous
  except(OSError,EOFError):pass
 return gzip.compress(raw,mtime=0)

def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--root',type=Path,default=ROOT);p.add_argument('--rustc');p.add_argument('--cargo');p.add_argument('--offline',action='store_true');p.add_argument('--fetch',action='store_true');p.add_argument('--check',action='store_true');a=p.parse_args();root=a.root.resolve()
 rustc=executable(a.rustc,'FERRUGO_RUSTC','.toolchain/rust/bin/rustc',root);cargo=executable(a.cargo,'FERRUGO_CARGO','.toolchain/rust/bin/cargo',root)
 environment=dict(os.environ,FERRUGO_RUSTC=rustc,FERRUGO_CARGO=cargo,RUSTC_BOOTSTRAP='1',FERRUGO_FP_METADATA_ROOT=str(root));environment.setdefault('CARGO_HOME',str(root/'.toolchain/port-cargo-home'))
 compiler=run([rustc,'-vV'],env=environment).decode().strip()
 if 'commit-hash: '+COMPILER not in compiler:raise RuntimeError('selected compiler differs from pinned1.99.1 development profile')
 reference=root/'tools/port-rust-fingerprint/reference-build';run([sys.executable,str(reference/'prepare.py'),'--root',str(root),*(['--offline']if a.offline else[]),*(['--fetch']if a.fetch else[])],env=environment)
 cache=root/'.toolchain/oracles/fingerprint-reference';profile=json.loads((cache/'build-profile.json').read_text());provenance=json.loads((cache/'source-provenance.json').read_text())
 if profile['rust_revision']!=PIN or profile['compiler']!=compiler:raise RuntimeError('original Fingerprint reference selection differs')
 rows=[];locations={};aborts=0
 with tempfile.TemporaryDirectory(prefix='ferrugo-fingerprint-decode-')as directory:
  binary=Path(directory)/'native';run([rustc,'--edition=2024','-C','opt-level=1',str((HERE/'oracle.rs').relative_to(root)),*profile['link_arguments'],'-o',str(binary)],env=environment,cwd=root)
  for ambient in ['normal','inherited','processing']:
   for packed in [False,True]:
    for length in [0,15,16,17]:
     child=subprocess.run([str(binary),ambient,str(packed).lower(),str(length)],stdout=subprocess.PIPE,stderr=subprocess.PIPE,cwd=root,env=environment,timeout=10)
     if child.returncode not in [0,-6]:raise RuntimeError('unexpected native decode status: '+str(child.returncode))
     if len(child.stderr)>LIMIT:raise RuntimeError('native decode observation exceeds bound')
     abort=child.returncode==-6;aborts+=abort;events=[]
     for line in child.stderr.decode().splitlines():
      if not line.startswith('EVENT:'):continue
      event=line.removeprefix('EVENT:');events.append(event)
      if event.startswith('hook:'):
       parts=event.split(':')
       if len(parts)!=10:raise RuntimeError('unexpected original panic hook record')
       file=bytes.fromhex(parts[3]).decode();site=DECODE_SITE if file=='original/compiler/rustc_data_structures/src/fingerprint.rs'else'decode-metadata::ambient'
       location={'File':file,'Line':int(parts[4]),'Column':int(parts[5])}
       if site in locations and locations[site]!=location:raise RuntimeError('native caller location changed within one source site')
       locations[site]=location
     rows.append('\t'.join([ambient,str(packed).lower(),str(length),str(abort).lower(),'|'.join(events)])+'\n')
 raw=''.join(rows).encode();fixture=HERE/'cases.tsv.gz';gz=compressed(raw,fixture)
 proof={'schema':1,'scope':'Original complete Fingerprint/PackedFingerprint decode via safe caller-defined Decoder raw-byte override; main55,292-row corpus remains unchanged.','source_revision':PIN,'compiler':compiler,'reference_provenance_sha256':sha((cache/'source-provenance.json').read_bytes()),'registry_dependencies':profile['registry_dependencies'],'locations':locations,'location_profile':'Only known Fingerprint reference wrapper path is mapped to original/compiler/...; the oracle repository prefix is stripped. Raw native filenames remain in every hook event. Production locations are caller-supplied and absent by default.','records':len(rows),'actual_aborts':aborts,'raw_sha256':sha(raw),'fixture_sha256':sha(gz),'client_sha256':sha((HERE/'oracle.rs').read_bytes()),'helpers_sha256':{name:sha((HERE/name).read_bytes())for name in ['generate.py']},'reference_helpers_sha256':{str(path.relative_to(root)):sha(path.read_bytes())for path in [reference/'prepare.py',root/'tools/port-rust-edition/reference-build/prepare.py',reference/'lib.rs',reference/'Cargo.toml',reference/'Cargo.lock']},'go_production_sha256':sha((root/'port/rust/compiler/rustc_data_structures/src/fingerprint.go').read_bytes()),'go_replay_sha256':sha((root/'port/rust/compiler/rustc_data_structures/src/fingerprint_decode_metadata_test.go').read_bytes())}
 encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode();manifest=HERE/'manifest.json'
 if a.check:
  if not fixture.exists()or fixture.read_bytes()!=gz or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('fingerprint decode metadata evidence differs')
 else:fixture.write_bytes(gz);manifest.write_bytes(encoded)
 print(f'Fingerprint decode metadata: {len(rows)} native records; {aborts} actual aborts')
if __name__=='__main__':
 try:main()
 except(Exception)as error:raise SystemExit(str(error))
