#!/usr/bin/env python3
"""Original whole-file Cargo graph contracts; Rust is a development oracle only."""
import argparse,concurrent.futures,gzip,hashlib,json,os,pathlib,resource,shutil,signal,stat,subprocess,sys,tarfile,tomllib
ROOT=pathlib.Path(__file__).resolve().parents[3]
HERE=pathlib.Path(__file__).resolve().parent
TARGET=ROOT/'port/cargo/src/util'
PIN='24de4721844c7a4d839072bab6fb66d41c9bc1f4'
BLOB='74cdc51a0c36f21ce64f291540be571c05b12d63'
SOURCE_SHA='238123cc4c8300921008db5bcec1bbdf0946a023e0acafe9997e6917b5fada51'
MAX=(1<<64)-1
TAGS={'key-cmp','key-eq','key-ne','key-lt','key-clone','key-drop','edge-clone','edge-default','edge-drop','edge-eq','edge-ne','info-drop','borrow','display'}
def sha(data):return hashlib.sha256(data).hexdigest()
def run(command,env,timeout=180,**kwargs):
 result=subprocess.run([str(x)for x in command],env=env,cwd=ROOT,capture_output=True,timeout=timeout,**kwargs)
 if result.returncode:raise RuntimeError(f'{command[0]} failed: '+result.stderr.decode(errors='replace')[-8192:])
 return result.stdout
def executable(name,env_key,default):
 if name is None:
  if env_key in os.environ:name=os.environ[env_key]
  else:name=str(ROOT/default)if(ROOT/default).is_file()else pathlib.Path(default).name
 if not name:raise RuntimeError('explicit empty '+env_key)
 if '/'in name and not os.path.isabs(name):name=str(ROOT/name)
 found=shutil.which(name)
 if not found:raise RuntimeError('missing executable: '+name)
 return str(pathlib.Path(found).absolute())
def archive_source(archive,source,name,version):
 prefix=name+'-'+version;files={};directories=set()
 with tarfile.open(archive,'r:gz')as package:
  for member in package:
   if member.isdir()and member.name.rstrip('/')==prefix:continue
   if not member.name.startswith(prefix+'/'):raise RuntimeError('archive prefix differs')
   relative=member.name[len(prefix)+1:].rstrip('/');path=pathlib.PurePosixPath(relative)
   if not relative or path.is_absolute()or'..'in path.parts or str(path)!=relative:raise RuntimeError('unsafe archive path')
   if member.isfile():
    if relative in files:raise RuntimeError('duplicate archive file')
    files[relative]=package.extractfile(member).read()
   elif not member.isdir():raise RuntimeError('non-regular archive entry')
   for parent in path.parents:
    if str(parent)!='.':directories.add(str(parent))
   if member.isdir():directories.add(relative)
 seen=set()
 for current,dirs,names in os.walk(source,followlinks=False):
  current=pathlib.Path(current)
  for child in dirs:
   p=current/child
   if p.is_symlink()or not stat.S_ISDIR(p.lstat().st_mode)or p.relative_to(source).as_posix()not in directories:raise RuntimeError('unexpected registry directory '+str(p))
  for child in names:
   p=current/child;relative=p.relative_to(source).as_posix()
   if p.is_symlink()or not stat.S_ISREG(p.lstat().st_mode):raise RuntimeError('non-regular registry source')
   if relative=='.cargo-ok'and relative not in files:
    if p.read_bytes()not in[b'{"v":1}',b'ok',b'']:raise RuntimeError('unexpected cargo marker')
    continue
   if relative not in files or p.read_bytes()!=files[relative]:raise RuntimeError('compiled registry source differs: '+str(p))
   seen.add(relative)
 if seen!=set(files):raise RuntimeError('missing expanded registry files: '+name)
 return files
def dependencies(cargo_home,upstream,env):
 locked=tomllib.loads((HERE.parent/'Cargo.lock').read_text())['package']
 original=tomllib.loads(run(['git','-C',upstream,'show',PIN+':Cargo.lock'],env).decode())['package']
 selected=[]
 for pkg in locked:
  if 'checksum'not in pkg:continue
  if not any(x.get('name')==pkg['name']and x.get('version')==pkg['version']and x.get('checksum')==pkg['checksum']for x in original):raise RuntimeError('dependency is not the actual Cargo lock selection')
  archives=list((cargo_home/'registry/cache').glob('*/'+pkg['name']+'-'+pkg['version']+'.crate'))
  if len(archives)!=1 or sha(archives[0].read_bytes())!=pkg['checksum']:raise RuntimeError('registry archive checksum/missing: '+pkg['name'])
  source=cargo_home/'registry/src'/archives[0].parent.name/(pkg['name']+'-'+pkg['version'])
  for path in [source,source.parent,source.parent.parent]:
   if not path.is_dir()or path.is_symlink():raise RuntimeError('missing or symlink registry source')
  files=archive_source(archives[0],source,pkg['name'],pkg['version'])
  digest=sha(b''.join((name+'\0'+sha(data)+'\n').encode()for name,data in sorted(files.items())))
  selected.append({'name':pkg['name'],'version':pkg['version'],'archive_sha256':pkg['checksum'],'expanded_tree_sha256':digest,'archive_files':len(files),'source':source,'files':files})
 if len(selected)!=7:raise RuntimeError('expected all7 exact registry dependencies')
 return selected
def request(scenario=1,size=3,op=0,mode=0,fail1=MAX,fail2=MAX,format=0,write_fail=MAX,chars=0,reuse=0):return '\t'.join(map(str,[scenario,size,op,mode,fail1,fail2,format,write_fail,chars,reuse]))
def mutate(req,**fields):
 values=req.split('\t');positions={'scenario':0,'size':1,'op':2,'mode':3,'fail1':4,'fail2':5,'format':6,'write_fail':7,'chars':8,'reuse':9}
 for key,value in fields.items():values[positions[key]]=str(value)
 return '\t'.join(values)
def baseline_requests():
 result=[]
 for scenario in range(9):
  for size in [0,1,3,12]:
   for op in [*range(25),26,27,28,29]:result.append(request(scenario,size,op))
 for scenario in [0,1,3,4,6,7,8]:
  for size in [65,130]:
   for op in [*range(25),26,27,28,29]:result.append(request(scenario,size,op))
 for bits in range(512):
  for op in [19,20,22]:result.append(request(100+bits,3,op))
 return list(dict.fromkeys(result))
def parse_hook(event):
 p=event.split(':');return {'kind':p[1],'payload':bytes.fromhex(p[2]).decode(),'can_unwind':p[3]=='true','force_no_backtrace':p[4]=='true','file':bytes.fromhex(p[5]).decode(),'line':int(p[6]),'column':int(p[7])}
def printer_stderr(raw):
 if raw.startswith(b'Graph for tests = Graph {\n'):
  end=raw.find(b'}\n')
  if end>=0:return raw[:end+2].decode()
 return ''
def observe(binary,req,env,hooks=False,behavior='observe'):
 child=env.copy();child['FERRUGO_GRAPH_ABORT_TRACE']='1'
 if hooks:child['FERRUGO_GRAPH_HOOKS']='1'
 else:child.pop('FERRUGO_GRAPH_HOOKS',None)
 child['FERRUGO_GRAPH_HOOK_BEHAVIOR']=behavior
 p=subprocess.run([str(binary)],input=(req+'\n').encode(),env=child,capture_output=True,timeout=5)
 if len(p.stdout)>8<<20 or len(p.stderr)>1<<20:raise RuntimeError('native observation exceeds bound')
 lines=p.stdout.decode().splitlines()
 if p.returncode==-signal.SIGABRT:return {'abort':True,'trace':lines,'stderr':printer_stderr(p.stderr)}
 if p.returncode:raise RuntimeError('unexpected native status: '+str(p.returncode)+' '+p.stderr.decode(errors='replace'))
 if not lines or len(lines[-1].split('\t'))!=6:raise RuntimeError('native schema differs: '+str(lines[-1:]))
 fields=lines[-1].split('\t');trace=bytes.fromhex(fields[5]).decode().split('|')if fields[5]else[]
 if trace!=lines[:-1]:raise RuntimeError('streamed native events disagree')
 return {'abort':False,'result':bytes.fromhex(fields[0]).decode(),'len':int(fields[1]),'after':fields[2],'stored':fields[3],'alive':fields[4],'trace':trace,'stderr':p.stderr.decode()}
def panic_sites(binary,env,debug,checked):
 sites={}
 def add(site,req,message):
  out=observe(binary,req,env,True)
  info=next(parse_hook(e)for e in out['trace']if e.startswith('hook:')and message in bytes.fromhex(e.split(':')[2]).decode())
  sites[site]=info;return out
 missing=request(7,3,18);out=add('cargo.graph.sort.index',missing,'OrdMap::index')
 if debug:
  add('cargo.graph.path_edge',request(110,3,19),'assertion failed:')
  add('cargo.graph.path_end',request(139,3,20),'internal error:')
  add('rust.std.stdio.stderr.print_error',request(1,3,17,write_fail=1),'failed printing to stderr:')
 add('graph.client.trip',request(0,3,0,fail1=0),'selected external')
 add('graph.client.outer1',request(0,0,6,mode=1),'outer')
 add('graph.client.outer3',request(0,0,6,mode=3),'outer')
 cleanup_index=next(i for i,e in enumerate(out['trace'])if e.startswith('key-drop:')and e.endswith(':true:op'))
 cleanup=observe(binary,mutate(missing,fail1=cleanup_index),env,True)
 if not cleanup['abort']:raise RuntimeError('native cleanup did not abort')
 sites['graph.runtime.cleanup']=next(parse_hook(e)for e in cleanup['trace']if e.startswith('hook:')and bytes.fromhex(e.split(':')[2]).decode()=='panic in a destructor during cleanup')
 if checked:
  base=request(6,130,29);baseline=observe(binary,base,env,True)
  for i in selected_indices(baseline):
   failed=observe(binary,mutate(base,fail1=i),env,True)
   found=[parse_hook(e)for e in failed['trace']if e.startswith('hook:')and bytes.fromhex(e.split(':')[2]).decode()=='attempt to subtract with overflow']
   if found:sites['im-rc.btree.iter.remaining_sub']=found[0];break
  if 'im-rc.btree.iter.remaining_sub'not in sites:raise RuntimeError('checked partial-mutation iterator continuation was not observed')
 return sites
def trip_indices(out):return [i for i,e in enumerate(out['trace'])if e.split(':',1)[0]in TAGS and e.endswith(':op')]
def selected_indices(out):
 grouped={}
 for i in trip_indices(out):grouped.setdefault(out['trace'][i].split(':',1)[0],[]).append(i)
 return sorted({values[x]for values in grouped.values()for x in [0,len(values)//2,len(values)-1]})
def profile(name,debug,checked,work,cargo,rustc,env):
 build=env.copy();target=work/'target'/name;build.update(CARGO_TARGET_DIR=str(target),CARGO_PROFILE_DEV_DEBUG_ASSERTIONS=str(debug).lower(),CARGO_PROFILE_DEV_OVERFLOW_CHECKS=str(checked).lower(),CARGO_PROFILE_DEV_OPT_LEVEL='0',CARGO_PROFILE_TEST_DEBUG_ASSERTIONS=str(debug).lower(),CARGO_PROFILE_TEST_OVERFLOW_CHECKS=str(checked).lower(),CARGO_PROFILE_TEST_OPT_LEVEL='0')
 run([cargo,'build','--locked','--offline','--manifest-path',work/'Cargo.toml'],build)
 outputs=list((target/'debug/build').glob('im-rc-*/output'))
 if not outputs or any(b'cargo:rustc-cfg=has_specialisation'in x.read_bytes()for x in outputs):raise RuntimeError('unexpected im-rc specialization profile')
 tests=run([cargo,'test','--locked','--offline','--manifest-path',work/'Cargo.toml','--bin','ferrugo-cargo-graph-oracle'],build)
 if b'3 passed; 0 failed'not in tests:raise RuntimeError('all original graph tests did not pass')
 binary=target/'debug/ferrugo-cargo-graph-oracle';sites=panic_sites(binary,build,debug,checked);cache={};records=[];seen=set()
 def record(req,hooks=False,behavior='observe'):
  key=(req,hooks,behavior)
  if key not in cache:cache[key]=observe(binary,req,build,hooks,behavior)
  if key not in seen:seen.add(key);records.append({'request':req,'hooks':hooks,'behavior':behavior,'want':cache[key]})
  return cache[key]
 for req in baseline_requests():record(req)
 # Every actual callback position in the complete small operation matrix.
 for scenario in [0,1,2,4,5,6,7,8,110,139]:
  for op in [*range(25),26,27,28,29]:
   base=request(scenario,3,op);out=record(base)
   for index in trip_indices(out):record(mutate(base,fail1=index))
 # All writer positions, independent char dispatch, ordinary Display errors,
 # repeated mutable formatter use, and all explicit ambient panic contexts.
 for mode in range(4):
  for pattern in range(8):
   for chars in [0,1]:
    for reuse in [0,1]:
     for fail in [MAX,*range(24)]:record(request(6,3,12,mode=mode,format=pattern,chars=chars,reuse=reuse,write_fail=fail))
 for mode in range(4):
  for failure in [0,1]:
   for hooks in [False,True]:record(request(1,3,17,mode=mode,write_fail=failure),hooks)
 # Context changes and selected first/middle/last sites of each independent
 # trait operation; ordinary-mode small tests above retain every position.
 for scenario in [0,1,6,7,110,139]:
  for op in [0,1,2,4,5,12,14,15,16,17,18,19,20,22,23,24,26,27,28,29]:
   for mode in [1,2,3]:
    base=request(scenario,3,op,mode=mode);out=record(base)
    for index in selected_indices(out):record(mutate(base,fail1=index))
 # Partial clone and original-tuple destructor failures beyond both B-tree
 # capacities, including continuation after a consumed-argument Drop panic.
 for scenario in [1,6]:
  for size in [65,130]:
   for op in [4,5,18,26,27,28,29]:
    base=request(scenario,size,op);out=record(base)
    for index in selected_indices(out):record(mutate(base,fail1=index))
 # Hook metadata/payload/location, forwarding without a second hook, real
 # cleanup-abort diagnostics, hook panic/resume/local-destructor aborts.
 hook_bases=[request(0,3,op,mode=mode)for op in [0,1,2,4,5,12,15,16,18]for mode in range(4)]
 hook_bases += [request(7,3,18),request(110,3,19),request(139,3,20)]
 hook_bases += [request(6,130,29)]
 for base in hook_bases:
  out=record(base,True)
  for index in selected_indices(out):
   failed=record(mutate(base,fail1=index),True)
   if not failed['abort']:
    for j,e in enumerate(failed['trace']):
     if e.split(':',1)[0]in ['key-drop','edge-drop','info-drop']and e.endswith(':true:op')and j>index:
      record(mutate(base,fail1=index,fail2=j),True);break
 for mode in range(4):
  base=request(7,3,18,mode=mode)
  for behavior in ['panic','resume','drop_panic']:record(base,True,behavior)
 print(f'Graph {name}: {len(records)} native observations, {sum(r["want"]["abort"]for r in records)} SIGABRTs',flush=True)
 return records,{'name':name,'debug_assertions':debug,'overflow_checks':checked,'opt_level':0,'original_tests_passed':3,'has_specialisation':False,'records':len(records),'aborts':sum(r['want']['abort']for r in records),'panic_sites':sites}
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--cargo');p.add_argument('--rustc');p.add_argument('--cargo-home');p.add_argument('--jobs',type=int,default=2);args=p.parse_args();resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 if args.jobs<1 or args.jobs>4:raise RuntimeError('jobs must be1..4')
 rustc=executable(args.rustc,'FERRUGO_RUSTC','.toolchain/rust/bin/rustc');cargo=executable(args.cargo,'FERRUGO_CARGO','.toolchain/rust/bin/cargo')
 env=os.environ.copy();env.update(RUSTC=rustc,RUSTC_BOOTSTRAP='1',GIT_NO_LAZY_FETCH='1');env.pop('RUSTFLAGS',None);env.pop('CARGO_ENCODED_RUSTFLAGS',None)
 version=run([rustc,'--version'],env).decode().strip()
 if version!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('Graph requires its exact stable comparison profile')
 if b'target_pointer_width="64"'not in run([rustc,'--print','cfg'],env):raise RuntimeError('retained Graph native profile is64-bit')
 cargo_home=pathlib.Path(args.cargo_home or env.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home')).absolute();env['CARGO_HOME']=str(cargo_home)
 upstream=pathlib.Path(env.get('FERRUGO_CARGO_SOURCE',ROOT/'.toolchain/upstream-cargo'))
 if run(['git','-C',upstream,'rev-parse',PIN+':src/util/graph.rs'],env).decode().strip()!=BLOB:raise RuntimeError('source Git identity differs')
 source=run(['git','-C',upstream,'cat-file','blob',BLOB],env)
 if sha(source)!=SOURCE_SHA:raise RuntimeError('source bytes differ')
 deps=dependencies(cargo_home,upstream,env);work=ROOT/'.toolchain/oracles/cargo-graph-owned';work.mkdir(parents=True,exist_ok=True)
 retained=json.loads((HERE.parent/'validation.json').read_text())['evidence_sha256']
 for path,expected in retained.items():
  if sha((ROOT/path).read_bytes())!=expected:raise RuntimeError('frozen historical evidence changed: '+path)
 for name in ['Cargo.toml','Cargo.lock']:(work/name).write_bytes((HERE.parent/name).read_bytes())
 (work/'graph.rs').write_bytes(source);(work/'oracle.rs').write_bytes((HERE/'owned_oracle.rs').read_bytes())
 index=next(d['source'].parent for d in deps if d['name']=='im-rc')
 remaps=[f'--remap-path-prefix={work/"graph.rs"}=src/util/graph.rs',f'--remap-path-prefix={work/"oracle.rs"}=tools/port-cargo-graph/contracts/owned_oracle.rs',f'--remap-path-prefix={index}/=']
 env['CARGO_GRAPH_SOURCE']=str(work/'graph.rs');env['CARGO_ENCODED_RUSTFLAGS']='\x1f'.join(remaps)
 configs=[(f'd{int(d)}o{int(o)}',d,o)for d in[False,True]for o in[False,True]]
 with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs)as pool:results=list(pool.map(lambda c:profile(*c,work,cargo,rustc,env),configs))
 packed={}
 for (rows,metadata)in results:
  for row in rows:
   key=json.dumps(row,separators=(',',':'))
   if key not in packed:packed[key]={**row,'profiles':[]}
   packed[key]['profiles'].append(metadata['name'])
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in packed.values())
 if len(raw)>192<<20:raise RuntimeError('retained raw corpus exceeds192MiB bound')
 fixture=TARGET/'testdata/graph_owned.jsonl.gz';old=fixture.read_bytes()if fixture.exists()else b'';compressed=old if old and gzip.decompress(old)==raw else gzip.compress(raw,mtime=0)
 files=[TARGET/name for name in ['graph_owned.go','graph_owned_traversal.go','graph_btree.go','graph_owned_contract_test.go','graph_owned_oracle_test.go']]
 files+=list((ROOT/'port/dependencies/im-rc/v15_1_0').rglob('*.go'));files+=[HERE/'owned_oracle.rs',pathlib.Path(__file__),HERE/'audit.py',HERE/'std-reference.json',HERE.parent/'Cargo.toml',HERE.parent/'Cargo.lock',ROOT/'port/support/rustfmt/panic.go',ROOT/'port/support/rustvec/ownership.go',ROOT/'port/support/rustsort/stable.go']
 files+=list((ROOT/'port/support/rusterrno').glob('*.go'))
 files+=[TARGET/'graph_stderr_binding_test.go',HERE/'generate_errno.py',HERE/'native_errno.rs',HERE/'errno-validation.json',ROOT/'port/support/rustfmt/formatter.go']
 files+=list((ROOT/'port/support/ruststdio').glob('*.go'))
 files+=[TARGET/'graph_stdio_oracle_test.go',TARGET/'testdata/graph_stdio.json',HERE/'generate_stdio.py',HERE/'stdio_oracle.rs',HERE/'stdio-validation.json',ROOT/'tools/port-rust-stdio/manifest.json']
 metadata={'format':1,'cargo_revision':PIN,'source_git_blob':BLOB,'source_sha256':SOURCE_SHA,'compiler':run([rustc,'-vV'],env).decode().strip(),'pointer_bits':64,'profiles':[m for _,m in results],'native_observations':sum(len(rows)for rows,_ in results),'retained_rows':len(packed),'raw_bytes':len(raw),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'retained_legacy_evidence':retained,'registry_dependencies':[{k:v for k,v in d.items()if k not in['source','files']}for d in deps],'bindings':{str(f.relative_to(ROOT)):sha(f.read_bytes())for f in sorted(files)},'scope':'Complete unchanged Cargo graph.rs selected stable1.99 no-specialisation dependency profile; actual non-Copy callbacks, Rc COW, field Drop/partial states, shared references, lazy iterators, fallible formatting and real panic hooks. No entire im-rc/std source-file credit or Rust ABI/allocator claim.','unsupported_dependency_profiles':['has_specialisation/nightly (including Copy-key fast search)','pool','threadsafe im variant','unreferenced im-rc collection APIs/features'],'bounds':{'native_seconds_per_case':5,'native_stdout_bytes':8<<20,'native_stderr_bytes':1<<20,'raw_fixture_bytes':192<<20,'core_dumps':False},'remap_contract':['graph source ->src/util/graph.rs','registry crate files ->crate-version/path','wrapper spelling is the actual Cargo rustc argument oracle.rs']}
 manifest=HERE/'owned-validation.json';encoded=(json.dumps(metadata,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or manifest.read_bytes()!=encoded:raise RuntimeError('Graph native contracts/bindings differ')
 else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
 print(f'Graph: {metadata["native_observations"]} native observations, {len(packed)} retained profile-grouped rows; all4 profiles/original tests pass.')
if __name__=='__main__':
 try:main()
 except Exception as e:print(e,file=sys.stderr);sys.exit(1)
