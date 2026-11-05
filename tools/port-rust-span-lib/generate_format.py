#!/usr/bin/env python3
"""Retain original whole-crate formatter streams and mutable state."""
import argparse,gzip,hashlib,io,json,os,pathlib,subprocess,tempfile
import reference
ROOT=pathlib.Path(__file__).resolve().parents[2];HERE=pathlib.Path(__file__).resolve().parent
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--profile',type=pathlib.Path,default=ROOT/'.toolchain/oracles/rustc-span-reference/build-profile-d1o1.json');p.add_argument('--check',action='store_true');a=p.parse_args();profile,environment=reference.load_profile(a.profile);assert profile['source_revision']=='2d20b42b883dcfc014f3c2c56081eeb6091c2179'and profile['unicode']==[18,0,0]
 source=HERE/'native_format.rs';destination=ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-format.tsv.gz';manifest=HERE/'format-manifest.json'
 with tempfile.TemporaryDirectory(prefix='ferrugo-span-lib-format-')as directory:
  executable=pathlib.Path(directory)/'native';subprocess.run([profile['rustc'],str(source),*profile['link_arguments'],'-o',str(executable)],check=True,cwd=ROOT,env=environment)
  output=io.BytesIO();native_hash=hashlib.sha256();native_bytes=0;pending=[];key=None
  def flush():
   if not pending:return
   full=pending[-1];calls=full[7].split(',')if full[7]else[]
   assert full[4]=='true'and int(full[3])==len(calls)
   for row in pending:
    cut=int(row[3])
    # Compact only after independently checking the ACTUAL native stream and
    # accepted prefix against the retained success record. No result/state is
    # inferred, and non-prefix behavior remains an explicit full observation.
    if cut<len(calls)and row[7]==','.join(calls[:cut+1])and row[6]==''.join(call.split(':',1)[1]for call in calls[:cut]):row[6]=row[7]='@'
    output.write(('\t'.join(row)+'\n').encode())
  process=subprocess.Popen([str(executable)],stdout=subprocess.PIPE,cwd=ROOT)
  for line in process.stdout:
   native_hash.update(line);native_bytes+=len(line)
   if native_bytes>512*1024*1024:process.kill();raise SystemExit('native formatter stream exceeds bound')
   row=line.decode().rstrip('\n').split('\t');assert len(row)==8;current=tuple(row[:3])
   if key is not None and current!=key:flush();pending=[]
   key=current;pending.append(row)
  status=process.wait()
  if status:raise SystemExit(f'native formatter exited with status {status}')
  flush();raw=output.getvalue()
 assert len(raw)<64*1024*1024;compressed=reference.compressed_fixture(raw,destination,64*1024*1024)
 proof={'schema':1,'status':'partial-lib-validation','source_revision':profile['source_revision'],'rustc_version':profile['rustc_version'],'unicode':profile['unicode'],'reference_provenance_sha256':profile['provenance_sha256'],'client_sha256':sha(source.read_bytes()),'tool_inputs_sha256':reference.tool_inputs(),'generator_sha256':sha(pathlib.Path(__file__).read_bytes()),'records':len(raw.splitlines()),'native_raw_sha256':native_hash.hexdigest(),'native_raw_bytes':native_bytes,'raw_sha256':sha(raw),'fixture_sha256':sha(compressed)};encoded=(json.dumps(proof,indent=2,sort_keys=True)+'\n').encode()
 if a.check:
  if not destination.exists()or destination.read_bytes()!=compressed or not manifest.exists()or manifest.read_bytes()!=encoded:raise SystemExit('lib format evidence is stale')
 else:destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f"formatter native records: {proof['records']}")
if __name__=='__main__':
 try:main()
 except subprocess.CalledProcessError as error:raise SystemExit(f'native formatter command failed with status {error.returncode}')
