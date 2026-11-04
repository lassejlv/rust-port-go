#!/usr/bin/env python3
"""Build the whole unchanged def_id.rs against actual pinned macro/hash/codec sources."""
import argparse,hashlib,importlib.util,json,os,pathlib,shutil,subprocess,sys,tomllib
sys.dont_write_bytecode=True
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parents[2]
PIN='2d20b42b883dcfc014f3c2c56081eeb6091c2179'
CRATES=('rustc_serialize','rustc_hashes','rustc_macros','rustc_index_macros')
PATHS=['compiler/rustc_span/src/def_id.rs','compiler/rustc_span/src/lib.rs','compiler/rustc_data_structures/src/fingerprint.rs','compiler/rustc_data_structures/src/fingerprint/tests.rs','compiler/rustc_data_structures/src/unord.rs','compiler/rustc_data_structures/src/unhash.rs','compiler/rustc_data_structures/src/fx.rs','compiler/rustc_data_structures/src/atomic_ref.rs','compiler/rustc_data_structures/src/stable_hash.rs','compiler/rustc_index/src/idx.rs','Cargo.lock','src/version','src/stage0']
def digest(data):return hashlib.sha256(data).hexdigest()
def write(path,data):
 path.parent.mkdir(parents=True,exist_ok=True)
 if not path.exists()or path.read_bytes()!=data:path.write_bytes(data)
def module(name,path):
 spec=importlib.util.spec_from_file_location(name,path);result=importlib.util.module_from_spec(spec);spec.loader.exec_module(result);return result
def selected(root,variable,local,fallback):
 if variable in os.environ:name=os.environ[variable]
 else:name=str(root/local)if(root/local).is_file()else fallback
 if not name:raise RuntimeError(variable+' explicitly empty')
 if '/'in name and not pathlib.Path(name).is_absolute():name=str(root/name)
 result=shutil.which(name)
 if not result:raise RuntimeError(variable+' executable unavailable: '+name)
 return os.path.abspath(result)
def command(args,env,cwd):
 p=subprocess.run([str(x)for x in args],cwd=cwd,env=env,capture_output=True,timeout=240)
 if p.returncode:raise RuntimeError(f'{args}: exit{p.returncode}\n{p.stdout.decode(errors="replace")}\n{p.stderr.decode(errors="replace")}')
 return p.stdout
def prepare(root,destination,env):
 expected={}
 def put(path,data):
  write(path,data);expected[path]=data
 source=pathlib.Path(os.environ.get('FERRUGO_RUST_SOURCE',str(root/'.toolchain/upstream-rust'))).resolve()
 def git(*args):return command(['git','-C',source,*args],env,root)
 if git('rev-parse','HEAD').decode().strip()!=PIN:raise RuntimeError('source revision differs')
 paths=set(PATHS)
 for crate in CRATES:paths.update(git('ls-tree','-r','--name-only',PIN,'compiler/'+crate).decode().splitlines())
 originals={};provenance=[]
 for path in sorted(paths):
  oid=git('rev-parse',PIN+':'+path).decode().strip();data=git('show',oid)
  if hashlib.sha1(f'blob {len(data)}\0'.encode()+data).hexdigest()!=oid:raise RuntimeError('Git blob differs:'+path)
  originals[path]=data;put(destination/'original'/path,data);provenance.append({'path':path,'git_blob':oid,'sha256':digest(data),'bytes':len(data)})
  for crate in CRATES:
   prefix='compiler/'+crate+'/'
   if path.startswith(prefix)and path[len(prefix):]!='Cargo.toml':put(destination/'crates'/crate/path[len(prefix):],data)
 for crate in CRATES:put(destination/'crates'/crate/'Cargo.toml',(HERE/(crate+'.Cargo.toml')).read_bytes())
 for name in ['Cargo.toml','lib.rs']:put(destination/('wrapper/lib.rs'if name=='lib.rs'else name),(HERE/name).read_bytes())
 for name,path in [('def_id.rs','compiler/rustc_span/src/def_id.rs'),('fingerprint.rs','compiler/rustc_data_structures/src/fingerprint.rs'),('fingerprint/tests.rs','compiler/rustc_data_structures/src/fingerprint/tests.rs'),('unord.rs','compiler/rustc_data_structures/src/unord.rs'),('unhash.rs','compiler/rustc_data_structures/src/unhash.rs'),('fx.rs','compiler/rustc_data_structures/src/fx.rs'),('atomic_ref.rs','compiler/rustc_data_structures/src/atomic_ref.rs'),('idx.rs','compiler/rustc_index/src/idx.rs')]:put(destination/'wrapper'/name,originals[path])
 support=module('def_id_span_support',root/'tools/port-rust-span-encoding/prepare.py')
 fragments=[]
 def block(path,needle,start=0):
  a,b,data=support.rust_block(originals[path],needle,start);fragments.append({'path':path,'start':a,'end':b,'sha256':digest(data),'bytes':len(data)});return data
 stablepath='compiler/rustc_data_structures/src/stable_hash.rs';stable=originals[stablepath]
 parts=[b'use crate::fingerprint::Fingerprint;',b'pub use rustc_stable_hash::{FromStableHash,SipHasher128Hash as StableHasherHash,StableSipHasher128 as StableHasher};']
 for marker,simple in [(b'pub trait StableHashCtxt {',False),(b'pub struct DummyStableHashCtxt;',True),(b'impl StableHashCtxt for DummyStableHashCtxt {',False),(b'pub struct RawSpan(pub u64);',True),(b'pub struct RawDefId(pub u32, pub u32);',True),(b'pub trait StableHash {',False),(b'pub trait ToStableHashKey {',False),(b'pub trait StableOrd: Ord {',False),(b'pub trait StableCompare {',False),(b'impl<T: StableOrd> StableCompare for T {',False),(b'macro_rules! impl_stable_traits_for_trivial_type {',False),(b'#[derive(Clone, Copy, Hash, Eq, PartialEq, Debug)]\npub struct StableHashControls {',False),(b'impl<T1: StableHash, T2: StableHash> StableHash for (T1, T2) {',False),(b"impl<'a, T> StableHash for &'a T",False)]:
  if simple:
   a=stable.index(marker);data=stable[a:a+len(marker)];fragments.append({'path':stablepath,'start':a,'end':a+len(marker),'sha256':digest(data),'bytes':len(data)});parts.append(data)
  else:parts.append(block(stablepath,marker))
 parts +=[b'pub(crate) use impl_stable_traits_for_trivial_type;',b'impl_stable_traits_for_trivial_type!(u64);',b'impl_stable_traits_for_trivial_type!(usize);',b'impl_stable_traits_for_trivial_type!(rustc_hashes::Hash64);']
 put(destination/'wrapper/stable_hash.rs',b'\n\n'.join(parts)+b'\n')
 libpath='compiler/rustc_span/src/lib.rs';lib=originals[libpath];start=lib.index(b'pub trait SpanEncoder:')
 traits=b'pub trait SpanEncoder: rustc_serialize::Encoder {\n'+block(libpath,b'fn def_path_hasher(',start)+b'\nfn encode_def_id(&mut self,id:DefId);\nfn encode_def_index(&mut self,id:DefIndex);\nfn encode_crate_num(&mut self,id:CrateNum);\n'+block(libpath,b'fn encode_local_def_id(',start)+b'\n}\npub trait BlobDecoder: rustc_serialize::Decoder {fn decode_local_def_id(&mut self)->LocalDefId;fn decode_def_index(&mut self)->DefIndex;}\npub trait SpanDecoder: BlobDecoder {fn decode_def_id(&mut self)->DefId;fn decode_crate_num(&mut self)->CrateNum;}\n'
 for marker in [b'impl<E: SpanEncoder> Encodable<E> for CrateNum',b'impl<E: SpanEncoder> Encodable<E> for DefIndex',b'impl<E: SpanEncoder> Encodable<E> for DefId',b'impl<D: SpanDecoder> Decodable<D> for CrateNum',b'impl<D: BlobDecoder> Decodable<D> for DefIndex',b'impl<D: SpanDecoder> Decodable<D> for DefId']:traits+=block(libpath,marker)+b'\n'
 put(destination/'wrapper/span_contracts.rs',traits)
 meta={'revision':PIN,'source_version':originals['src/version'].decode().strip(),'sources':provenance,'exact_fragments':fragments,'scope':'Whole unchanged def_id/unord/fingerprint/unhash/fx/atomic_ref/Idx files and actual macro/codec/hash crates. Surrounding Symbol text and used span codec/context interfaces are functional providers, not full Symbol/lib source compilation or source credit.'}
 put(destination/'source-provenance.json',(json.dumps(meta,indent=2)+'\n').encode());return originals['Cargo.lock'],meta,expected

def verify_local(destination,expected):
 # Registry packages are independently checked by verify_registry. Local
 # compiler inputs must also reject injected or stale source/build modules.
 for folder in ['original','crates','wrapper']:
  directory=destination/folder;wanted={p for p in expected if p.is_relative_to(directory)};actual=set()
  for path in directory.rglob('*'):
   if path.is_symlink():raise RuntimeError('symlink in local reference source:'+str(path))
   if path.is_file():actual.add(path)
  if actual!=wanted:raise RuntimeError('unexpected local reference file inventory:'+folder)
 for path,data in expected.items():
  if path.is_symlink()or not path.is_file()or path.read_bytes()!=data:raise RuntimeError('local reference source differs:'+str(path))
 for name in ['build.rs','src','.cargo']:
  if(destination/name).exists()or(destination/name).is_symlink():raise RuntimeError('unexpected Cargo auto-discovery input:'+name)

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--offline',action='store_true');ap.add_argument('--fetch',action='store_true');ap.add_argument('--write-lock',action='store_true');ap.add_argument('--debug',choices=['true','false'],default='true');ap.add_argument('--overflow',choices=['true','false'],default='true');args=ap.parse_args();root=args.root.resolve();destination=root/'.toolchain/oracles/def-id-reference'
 env=os.environ.copy()
 for key in ['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','CARGO_BUILD_TARGET','RUSTC_BOOTSTRAP','UPDATE_EXPECT']:env.pop(key,None)
 rustc=selected(root,'FERRUGO_RUSTC',pathlib.Path('.toolchain/rust/bin/rustc'),'rustc');cargo=selected(root,'FERRUGO_CARGO',pathlib.Path('.toolchain/rust/bin/cargo'),'cargo');env['RUSTC']=rustc;env['RUSTC_BOOTSTRAP']='1'
 if args.offline:env['GIT_NO_LAZY_FETCH']='1'
 if 'CARGO_HOME'in env and not env['CARGO_HOME']:raise RuntimeError('CARGO_HOME explicitly empty')
 env['CARGO_HOME']=str(pathlib.Path(env.get('CARGO_HOME',str(root/'.toolchain/port-cargo-home'))).resolve());env['RUSTFLAGS']=f'-Zcrate-attr=feature(never_type) -Cdebug-assertions={args.debug} -Coverflow-checks={args.overflow}'
 target=destination/f'target-d{int(args.debug=="true")}o{int(args.overflow=="true")}';env['CARGO_TARGET_DIR']=str(target)
 lock,provenance,local_inputs=prepare(root,destination,env);retained=HERE/'Cargo.lock';write(destination/'Cargo.lock',retained.read_bytes()if retained.exists()else lock)
 verify_local(destination,local_inputs)
 manifest=destination/'Cargo.toml';tail=['--manifest-path',manifest]
 if args.fetch or args.write_lock:command([cargo,'fetch',*tail,*(['--locked']if retained.exists()else[]),*(['--offline']if args.offline else[])],env,root)
 if args.write_lock:write(retained,(destination/'Cargo.lock').read_bytes())
 support=module('def_id_registry_support',root/'tools/port-rust-span-encoding/prepare.py');support.verify_lock((destination/'Cargo.lock').read_bytes(),lock)
 metadata=command([cargo,'metadata','--locked','--offline','--format-version=1',*tail],env,root);decoded=json.loads(metadata)
 allowed={'def_id_reference':destination/'Cargo.toml',**{name:destination/'crates'/name/'Cargo.toml'for name in CRATES}}
 for package in decoded['packages']:
  if package.get('source')is None and (package['name']not in allowed or pathlib.Path(package['manifest_path']).resolve()!=allowed[package['name']].resolve()):raise RuntimeError('unexpected local reference dependency:'+package['name'])
 deps=support.verify_registry(decoded,(destination/'Cargo.lock').read_bytes(),pathlib.Path(env['CARGO_HOME']))
 out=command([cargo,'build','--locked','--offline','--lib','--message-format=json',*tail],env,root);artifacts={}
 for line in out.splitlines():
  event=json.loads(line)
  if event.get('reason')=='compiler-artifact':artifacts[event['target']['name']]=event['filenames']
 if support.verify_registry(decoded,(destination/'Cargo.lock').read_bytes(),pathlib.Path(env['CARGO_HOME']))!=deps:raise RuntimeError('reference dependency changed during build')
 verify_local(destination,local_inputs)
 library=next(x for x in artifacts['def_id_reference']if x.endswith('.rlib'));profile={'revision':PIN,'compiler':command([rustc,'--version'],env,root).decode().strip(),'debug_assertions':args.debug=='true','overflow_checks':args.overflow=='true','library':library,'link_arguments':['--extern','def_id_reference='+library,'-L','dependency='+str(target/'debug/deps')],'registry_dependencies':deps,'source_provenance_sha256':digest((destination/'source-provenance.json').read_bytes())}
 write(destination/'build-profile.json',(json.dumps(profile,indent=2)+'\n').encode());print(destination)
if __name__=='__main__':main()
