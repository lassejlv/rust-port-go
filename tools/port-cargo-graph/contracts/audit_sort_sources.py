#!/usr/bin/env python3
"""Read-only algorithm-identity audit; never rewrites or compiles Rust bodies."""
import argparse,hashlib,json,pathlib,sys,tarfile,urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[3];HERE=pathlib.Path(__file__).resolve().parent
COMMIT='58cf8f9c35b355199727627d07ae34a4e2846d67'
def sha(b):return hashlib.sha256(b).hexdigest()
def function(body,marker):
 start=body.index(marker);begin=body.index(b'{',start);level=0
 for i in range(begin,len(body)):
  if body[i]==123:level+=1
  elif body[i]==125:
   level-=1
   if level==0:return body[start:i+1]
 raise RuntimeError('unclosed source item')
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--fetch',action='store_true');p.add_argument('--sdk-archive',type=pathlib.Path);p.add_argument('--reference-cache',type=pathlib.Path,default=ROOT/'.toolchain/oracles/cargo-graph-owned/reference-stdlib-sort');args=p.parse_args()
 catalog=ROOT/'tools/port-rust-unord/sort/SOURCES.json';sdk=json.loads(catalog.read_text());cache=args.reference_cache
 candidates=[args.sdk_archive]if args.sdk_archive else[cache/'sdk-source.tar.xz',ROOT/'.toolchain/ruststr-sdk-1.101/rust-src-nightly.tar.xz',pathlib.Path('/tmp/ferrugo-unord-rust-src-1.101-nightly.tar.xz')]
 archive=next((p for p in candidates if p and p.is_file()),None)
 if archive is None:
  if not args.fetch:raise RuntimeError('missing exact1.101 SDK source archive; use --sdk-archive or explicit --fetch')
  archive=cache/'sdk-source.tar.xz';archive.parent.mkdir(parents=True,exist_ok=True);archive.write_bytes(urllib.request.urlopen(sdk['official_archive_url'],timeout=60).read())
 if archive.is_symlink()or sha(archive.read_bytes())!=sdk['official_archive_sha256']:raise RuntimeError('SDK source archive checksum differs')
 newer={}
 with tarfile.open(archive,'r:xz')as package:
  for member in package:
   for item in sdk['files']:
    if member.name.endswith('/'+item['path']):
     if not member.isfile()or item['path']in newer:raise RuntimeError('unexpected SDK source member')
     data=package.extractfile(member).read()
     if sha(data)!=item['sha256']:raise RuntimeError('SDK source file checksum differs')
     newer[item['path']]=data
 if len(newer)!=len(sdk['files']):raise RuntimeError('SDK source selection incomplete')
 rows=[];wrappers=[]
 for item in sdk['files']:
  path=item['path'];target=cache/path;url='https://raw.githubusercontent.com/rust-lang/rust/'+COMMIT+'/'+path
  if not target.is_file():
   if not args.fetch:raise RuntimeError('missing1.99 reference source; explicit --fetch needed: '+path)
   target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(urllib.request.urlopen(url,timeout=40).read())
  if target.is_symlink():raise RuntimeError('reference source symlink rejected')
  old,new=target.read_bytes(),newer[path]
  row={'path':path,'reference_url':url,'reference_bytes':len(old),'reference_sha256':sha(old),'reference_git_blob':hashlib.sha1(('blob '+str(len(old))+'\0').encode()+old).hexdigest(),'shared_sdk_bytes':len(new),'shared_sdk_sha256':sha(new),'byte_identical':old==new}
  if path.startswith('library/core/src/slice/sort/'):
   def canonical(b):return b.replace(b'core::intrinsics::abort()',b'intrinsics::abort()').replace(b'intrinsics::abort_immediate()',b'intrinsics::abort()')
   if canonical(old)!=canonical(new):raise RuntimeError('actual sorting algorithm difference: '+path)
   row['canonical_sha256']=sha(canonical(old));row['sdk_abort_immediate_calls']=new.count(b'intrinsics::abort_immediate()')
  else:
   for marker in [b'pub fn sort(&mut self)',b'fn stable_sort<T, F>(',b'impl<T> sort::stable::BufGuard<T> for Vec<T>']:
    a,b=function(old,marker),function(new,marker)
    if a!=b:raise RuntimeError('actual sort wrapper differs: '+marker.decode())
    wrappers.append({'item':marker.decode(),'sha256':sha(a),'bytes':len(a),'byte_identical':True})
  rows.append(row)
 metadata={'scope':'Read-only exact Rust1.99.1 versus official1.101 source audit for Graph printer reference-key stable sort; no std file completion credit. Original bytes remain unchanged and are never compiled from canonicalized text.','reference_compiler_commit':COMMIT,'shared_source_catalog_sha256':sha(catalog.read_bytes()),'shared_sdk_archive_url':sdk['official_archive_url'],'shared_sdk_archive_sha256':sdk['official_archive_sha256'],'source_files':rows,'alloc_slice_sort_items':wrappers,'only_sort_algorithm_text_differences':'Internal invariant abort intrinsic spellings: core::intrinsics::abort()/intrinsics::abort() versus intrinsics::abort_immediate(). Both selected public sorts have the same input/type dispatch, comparisons, moves and cleanup; invalid internal length/scratch guards remain aborts. Other alloc::slice APIs are excluded.','audit_tool_sha256':sha(pathlib.Path(__file__).read_bytes())}
 output=HERE/'std-sort-reference.json';data=(json.dumps(metadata,indent=2)+'\n').encode()
 if args.check:
  if output.read_bytes()!=data:raise RuntimeError('sort source audit differs')
 else:output.write_bytes(data)
 print('Graph sort source audit: all7 algorithm files identical except abort-intrinsic spelling; all3 alloc sort/buffer items byte-identical.')
if __name__=='__main__':
 try:main()
 except Exception as e:print(e,file=sys.stderr);sys.exit(1)
