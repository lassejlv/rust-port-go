#!/usr/bin/env python3
"""Run unchanged Cargo graph printer against actual SDK capture/stderr state."""
import argparse,hashlib,importlib.util,json,os,pathlib,resource,shutil,subprocess,sys
ROOT=pathlib.Path(__file__).resolve().parents[3]
def sha(data):return hashlib.sha256(data).hexdigest()
def module(path):
 spec=importlib.util.spec_from_file_location('cargo_graph_stdio_original_support',path);value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value
def run(command,env,root,timeout=240):
 p=subprocess.run(list(map(str,command)),cwd=root,env=env,capture_output=True,timeout=timeout)
 if p.returncode:raise RuntimeError(f'command failed:{command[0]} exit{p.returncode}: '+p.stderr.decode(errors='replace')[-6000:])
 return p.stdout
def executable(value,key,fallback,root):
 name=value if value is not None else os.environ.get(key,fallback)
 if not name:raise RuntimeError(key+' explicitly empty')
 if '/'in name and not os.path.isabs(name):name=str(root/name)
 selected=shutil.which(name)
 if not selected:raise RuntimeError(key+' unavailable:'+name)
 return os.path.abspath(selected)
def requests():
 return [dict(Nodes=n,Capture=c,Preheld=p,Repeats=r,InitialCapacity=capacity,Error='none')for n in[0,1,3,12]for c in[False,True]for p in[False,True]for r in[1,2]for capacity in[0,16]]+[dict(Nodes=n,Capture=False,Preheld=False,Repeats=r,InitialCapacity=0,Error='eio')for n in[0,1,3,12]for r in[1,2]]
def observe(binary,observer,q,env,root):
 child=dict(env,LD_PRELOAD=str(observer),FERRUGO_STDIO_TRACE='1',FERRUGO_STDIO_SCRIPT='e5'if q['Error']=='eio'else'')
 p=subprocess.run([str(binary),str(q['Nodes']),str(int(q['Capture'])),str(int(q['Preheld'])),str(q['Repeats']),str(q['InitialCapacity']),q['Error']],cwd=root,env=child,capture_output=True,timeout=10)
 if p.returncode:raise RuntimeError(f'unexpected native exit {p.returncode}, no post-panic data fabricated: '+p.stderr.decode(errors='replace')[-2000:])
 if len(p.stdout)>1<<20 or len(p.stderr)>1<<20:raise RuntimeError('native output exceeds bound')
 row=dict(Request=q,NativeExit=p.returncode,Exit=0,StderrHex=p.stderr.hex(),Writes=[],Snapshots=[],CaptureSlotReturned=False,Hooks=[],HookOtherThreadLocked=None,PrintResults=[])
 for line in p.stdout.decode().splitlines():
  if line.startswith('write:'):
   value=line[6:];bytes.fromhex(value);row['Writes'].append(value);continue
  fields=line.split('\t');tag=fields[0]
  if tag=='S':
   if len(fields)!=6:raise RuntimeError('native snapshot schema differs')
   row['Snapshots'].append(dict(Stage=fields[1],Length=int(fields[2]),Capacity=int(fields[3]),ArcStrong=int(fields[4]),CaptureHex=fields[5]));bytes.fromhex(fields[5])
  elif tag=='H':
   if len(fields)!=8:raise RuntimeError('native hook schema differs')
   row['Hooks'].append(dict(Payload=bytes.fromhex(fields[2]).decode(),Kind=fields[1],Location=dict(File=bytes.fromhex(fields[3]).decode(),Line=int(fields[4]),Column=int(fields[5])),CanUnwind=fields[6]=='true',ForceNoBacktrace=fields[7]=='true'))
  elif tag=='L':
   if row['HookOtherThreadLocked']is not None:raise RuntimeError('duplicate native lock probe')
   row['HookOtherThreadLocked']=fields[1]=='true'
  elif tag=='P':row['PrintResults'].append(dict(Failed=fields[2]=='true',Payload=bytes.fromhex(fields[3]).decode()if fields[2]=='true'else None))
  elif tag=='R':row['CaptureSlotReturned']=fields[1]=='true'
  else:raise RuntimeError('unexpected native stdout line:'+line)
 stages=['before','set',*['print'+str(n)for n in range(1,q['Repeats']+1)],'popped','dropped']
 if [s['Stage']for s in row['Snapshots']]!=stages or len(row['PrintResults'])!=q['Repeats']:raise RuntimeError('native snapshot/print order differs')
 if q['Error']=='eio':
  if len(row['Hooks'])!=1 or row['HookOtherThreadLocked']is not True or row['PrintResults'][0]['Failed']is not True or any(x['Failed']for x in row['PrintResults'][1:]):raise RuntimeError('actual EIO release/control-flow proof failed')
 elif row['Hooks']or any(x['Failed']for x in row['PrintResults']):raise RuntimeError('unexpected normal printer panic')
 return row
def main():
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));ap=argparse.ArgumentParser();ap.add_argument('--root',type=pathlib.Path,default=ROOT);ap.add_argument('--cargo');ap.add_argument('--rustc');ap.add_argument('--cc');ap.add_argument('--check',action='store_true');ap.add_argument('--offline',action='store_true');a=ap.parse_args();root=a.root.resolve();here=root/'tools/port-cargo-graph/contracts'
 if sys.platform!='linux':raise RuntimeError('retained child-only syscall observer profile requires Linux')
 support=module(here/'generate_owned.py');env=os.environ.copy()
 for key in['RUSTFLAGS','CARGO_ENCODED_RUSTFLAGS','RUSTC_WRAPPER','RUSTC_WORKSPACE_WRAPPER','CARGO_BUILD_TARGET']:env.pop(key,None)
 rustc=executable(a.rustc,'FERRUGO_RUSTC',str(root/'.toolchain/rust/bin/rustc')if(root/'.toolchain/rust/bin/rustc').is_file()else'rustc',root);cargo=executable(a.cargo,'FERRUGO_CARGO',str(root/'.toolchain/rust/bin/cargo')if(root/'.toolchain/rust/bin/cargo').is_file()else'cargo',root);cc=executable(a.cc,'FERRUGO_CC',env.get('CC','cc'),root)
 version=run([rustc,'--version'],env,root).decode().strip()
 if version!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('exact SDK1.99.1 reference required')
 cfg=run([rustc,'--print','cfg'],env,root)
 if b'target_pointer_width="64"'not in cfg.splitlines():raise RuntimeError('source profile requires64-bit native host')
 cargo_home=pathlib.Path(env.get('CARGO_HOME',str(root/'.toolchain/port-cargo-home'))).absolute();upstream=root/'.toolchain/upstream-cargo';deps=support.dependencies(cargo_home,upstream,env)
 source=run(['git','-C',upstream,'cat-file','blob',support.BLOB],env,root)
 if sha(source)!=support.SOURCE_SHA or len(source.splitlines())!=303:raise RuntimeError('original complete graph source differs')
 work=root/'.toolchain/oracles/cargo-graph-stdio'
 if work.is_symlink():raise RuntimeError('oracle cache leaf is a symlink')
 work.mkdir(parents=True,exist_ok=True);marker=work/'owner.json';identity=dict(Tool='port-cargo-graph-stdio',SourceSHA256=support.SOURCE_SHA)
 if marker.is_symlink()or not marker.exists()and any(work.iterdir()):raise RuntimeError('refusing unowned or symlink oracle cache')
 if marker.exists()and json.loads(marker.read_text())!=identity:raise RuntimeError('independent oracle cache ownership differs')
 marker.write_text(json.dumps(identity,indent=2)+'\n');copied=work/'graph.rs'
 if copied.exists()and copied.read_bytes()!=source:raise RuntimeError('copied original graph source was modified')
 copied.write_bytes(source);(work/'stdio_oracle.rs').write_bytes((here/'stdio_oracle.rs').read_bytes());(work/'Cargo.lock').write_bytes((here.parent/'Cargo.lock').read_bytes());(work/'Cargo.toml').write_text('[package]\nname="ferrugo-cargo-graph-oracle"\nversion="0.0.0"\nedition="2024"\npublish=false\n[[bin]]\nname="ferrugo-cargo-graph-stdio"\npath="stdio_oracle.rs"\n[dependencies]\nim-rc="=15.1.0"\n[workspace]\n')
 env.update(RUSTC=rustc,RUSTC_BOOTSTRAP='1',CARGO_HOME=str(cargo_home),CARGO_TARGET_DIR=str(work/'target'),CARGO_PROFILE_DEV_DEBUG_ASSERTIONS='true',CARGO_PROFILE_DEV_OVERFLOW_CHECKS='true',CARGO_PROFILE_DEV_OPT_LEVEL='0',CARGO_PROFILE_DEV_DEBUG='0',CARGO_PROFILE_TEST_DEBUG_ASSERTIONS='true',CARGO_PROFILE_TEST_OVERFLOW_CHECKS='true',CARGO_PROFILE_TEST_OPT_LEVEL='0',CARGO_PROFILE_TEST_DEBUG='0')
 run([cargo,'build','--locked','--offline','--manifest-path',work/'Cargo.toml'],env,root)
 outputs=list((work/'target/debug/build').glob('im-rc-*/output'))
 if not outputs or any(b'cargo:rustc-cfg=has_specialisation'in x.read_bytes()for x in outputs):raise RuntimeError('im-rc stable/no-has_specialisation profile differs')
 tests=run([cargo,'test','--locked','--offline','--manifest-path',work/'Cargo.toml','--bin','ferrugo-cargo-graph-stdio'],env,root)
 if b'3 passed; 0 failed'not in tests:raise RuntimeError('three original graph tests did not pass')
 observer_source=root/'tools/port-rust-stdio/write_observer.c';observer=work/'write_observer.so';run([cc,'-shared','-fPIC','-std=c11','-O2',observer_source,'-o',observer],env,root)
 rows=[observe(work/'target/debug/ferrugo-cargo-graph-stdio',observer,q,env,root)for q in requests()]
 if copied.read_bytes()!=source:raise RuntimeError('original graph changed during execution')
 after=support.dependencies(cargo_home,upstream,env)
 if [{k:v for k,v in p.items()if k not in['source','files']}for p in deps]!=[{k:v for k,v in p.items()if k not in['source','files']}for p in after]:raise RuntimeError('registry inputs changed during execution')
 fixture=root/'port/cargo/src/util/testdata/graph_stdio.json';data=(json.dumps(dict(Schema=1,Cases=rows),indent=2)+'\n').encode();files=[here/'stdio_oracle.rs',here/'generate_stdio.py',observer_source,here.parent/'Cargo.toml',here.parent/'Cargo.lock',here/'generate_owned.py'];proof=dict(Schema=1,Status='original-graph-real-stdio-composition',WholeDependencyPort=False,CargoRevision=support.PIN,GraphBlob=support.BLOB,GraphSHA256=support.SOURCE_SHA,Compiler=version,DebugAssertions=True,OverflowChecks=True,Specialisation=False,DebugSymbols=False,PointerBits=64,NativeCases=len(rows),NativeEIOCases=sum(x['Request']['Error']=='eio'for x in rows),OriginalTests=3,FixtureSHA256=sha(data),Registry=[{k:v for k,v in p.items()if k not in['source','files']}for p in deps],Files=[dict(Path=str(p.relative_to(root)),SHA256=sha(p.read_bytes()))for p in files]);manifest=here/'stdio-validation.json';encoded=(json.dumps(proof,indent=2)+'\n').encode()
 if a.check:
  if not fixture.exists()or fixture.read_bytes()!=data or not manifest.exists()or manifest.read_bytes()!=encoded:raise RuntimeError('original Graph stdio observations/bindings differ')
 else:fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(data);manifest.write_bytes(encoded)
 print(f'Original graph real stdio: {len(rows)} cases,8 EIO lock-release probes,all3 original tests passed')
if __name__=='__main__':main()
