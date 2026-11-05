#!/usr/bin/env python3
"""Additional unchanged-source lazy Diff equality shapes; no production Rust."""
import argparse,concurrent.futures,gzip,json,os,pathlib,resource,sys
import generate_owned as ref
ROOT,HERE,TARGET=ref.ROOT,ref.HERE,ref.TARGET

def requests():
 result=[]
 for scenario in [0,3]:
  for size in [0,1,3,65,130]:
   for case in range(8):
    if size==0 and case in [4,5]:continue
    result.append(ref.request(scenario,size,case))
 for scenario in [5,6]:
  for size in [3,65,130]:
   for case in [8,9]:result.append(ref.request(scenario,size,case))
 return result

def profile(config,work,cargo,rustc,env):
 name,debug,checked=config;target=work/'target'/name;build=env.copy()
 build.update(CARGO_TARGET_DIR=str(target),CARGO_PROFILE_DEV_DEBUG_ASSERTIONS=str(debug).lower(),CARGO_PROFILE_DEV_OVERFLOW_CHECKS=str(checked).lower(),CARGO_PROFILE_DEV_OPT_LEVEL='0',CARGO_PROFILE_TEST_DEBUG_ASSERTIONS=str(debug).lower(),CARGO_PROFILE_TEST_OVERFLOW_CHECKS=str(checked).lower(),CARGO_PROFILE_TEST_OPT_LEVEL='0')
 ref.run([cargo,'build','--locked','--offline','--manifest-path',work/'Cargo.toml'],build)
 outputs=list((target/'debug/build').glob('im-rc-*/output'))
 if not outputs or any(b'cargo:rustc-cfg=has_specialisation'in p.read_bytes()for p in outputs):raise RuntimeError('unexpected specialization')
 tests=ref.run([cargo,'test','--locked','--offline','--manifest-path',work/'Cargo.toml','--bin','ferrugo-cargo-graph-oracle'],build)
 if b'3 passed; 0 failed'not in tests:raise RuntimeError('original tests failed')
 binary=target/'debug/ferrugo-cargo-graph-oracle';rows=[];seen=set()
 def record(req):
  if req in seen:return None
  seen.add(req);out=ref.observe(binary,req,build)
  if out['abort']:raise RuntimeError('unexpected single-failure equality abort')
  rows.append({'request':req,'want':out});return out
 for req in requests():
  out=record(req);values=list(map(int,req.split('\t')));size,case=values[1],values[2]
  if (size==3 and case in [0,2,4,8,9])or(size==130 and case in [2,4,8,9]):
   for index in ref.selected_indices(out):record(ref.mutate(req,fail1=index))
 print(f'Graph equality {name}: {len(rows)} observations',flush=True)
 return rows,{'name':name,'debug_assertions':debug,'overflow_checks':checked,'opt_level':0,'has_specialisation':False,'original_tests_passed':3,'records':len(rows)}

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--cargo');p.add_argument('--rustc');p.add_argument('--cargo-home');p.add_argument('--jobs',type=int,default=2);args=p.parse_args();resource.setrlimit(resource.RLIMIT_CORE,(0,0))
 if args.jobs not in range(1,5):raise RuntimeError('jobs must be1..4')
 cargo=ref.executable(args.cargo,'FERRUGO_CARGO','.toolchain/rust/bin/cargo');rustc=ref.executable(args.rustc,'FERRUGO_RUSTC','.toolchain/rust/bin/rustc')
 env=os.environ.copy();env.update(RUSTC=rustc,RUSTC_BOOTSTRAP='1',GIT_NO_LAZY_FETCH='1');env.pop('RUSTFLAGS',None);env.pop('CARGO_ENCODED_RUSTFLAGS',None)
 if ref.run([rustc,'--version'],env).decode().strip()!='rustc 1.99.1 (58cf8f9c3 2026-10-13)':raise RuntimeError('exact stable1.99 comparison profile required')
 if b'target_pointer_width="64"'not in ref.run([rustc,'--print','cfg'],env):raise RuntimeError('explicit native64 profile required')
 cargo_home=pathlib.Path(args.cargo_home or env.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home')).absolute();env['CARGO_HOME']=str(cargo_home)
 upstream=pathlib.Path(env.get('FERRUGO_CARGO_SOURCE',ROOT/'.toolchain/upstream-cargo'))
 source=ref.run(['git','-C',upstream,'cat-file','blob',ref.BLOB],env)
 if ref.sha(source)!=ref.SOURCE_SHA:raise RuntimeError('source identity differs')
 deps=ref.dependencies(cargo_home,upstream,env)
 work=ROOT/'.toolchain/oracles/cargo-graph-equality';work.mkdir(parents=True,exist_ok=True)
 for name in ['Cargo.toml','Cargo.lock']:(work/name).write_bytes((HERE.parent/name).read_bytes())
 original_client=(HERE/'owned_oracle.rs').read_bytes();marker=b'fn operation('
 if original_client.count(marker)!=1:raise RuntimeError('external type/client prefix boundary differs')
 prefix=original_client.split(marker)[0];client=prefix+(HERE/'equality_oracle.rs').read_bytes()
 (work/'oracle.rs').write_bytes(client);(work/'graph.rs').write_bytes(source)
 index=next(d['source'].parent for d in deps if d['name']=='im-rc')
 env['CARGO_GRAPH_SOURCE']=str(work/'graph.rs');env['CARGO_ENCODED_RUSTFLAGS']='\x1f'.join([f'--remap-path-prefix={work/"graph.rs"}=src/util/graph.rs',f'--remap-path-prefix={index}/='])
 configs=[(f'd{int(d)}o{int(o)}',d,o)for d in[False,True]for o in[False,True]]
 with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs)as pool:results=list(pool.map(lambda c:profile(c,work,cargo,rustc,env),configs))
 grouped={}
 for rows,meta in results:
  for row in rows:
   k=json.dumps(row,separators=(',',':'))
   if k not in grouped:grouped[k]={**row,'profiles':[]}
   grouped[k]['profiles'].append(meta['name'])
 raw=b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in grouped.values())
 if len(raw)>96<<20:raise RuntimeError('equality raw corpus exceeds96MiB')
 fixture=TARGET/'testdata/graph_equality.jsonl.gz';old=fixture.read_bytes()if fixture.is_file()else b'';compressed=old if old and gzip.decompress(old)==raw else gzip.compress(raw,mtime=0)
 files=[pathlib.Path(__file__),HERE/'audit_sort_sources.py',HERE/'std-sort-reference.json',ROOT/'tools/port-rust-unord/sort/SOURCES.json',HERE/'owned-validation.json',HERE/'equality_oracle.rs',HERE/'owned_oracle.rs',HERE/'generate_owned.py',TARGET/'graph_owned_equality_test.go',TARGET/'graph_owned_contract_test.go',TARGET/'graph_owned_oracle_test.go',TARGET/'graph_owned.go',TARGET/'graph_owned_traversal.go',TARGET/'graph_btree.go']+list((ROOT/'port/dependencies/im-rc/v15_1_0').rglob('*.go'))
 m={'format':1,'cargo_revision':ref.PIN,'source_git_blob':ref.BLOB,'source_sha256':ref.SOURCE_SHA,'compiler':ref.run([rustc,'-vV'],env).decode().strip(),'pointer_bits':64,'profiles':[m for _,m in results],'native_observations':sum(len(r)for r,_ in results),'retained_rows':len(grouped),'raw_bytes':len(raw),'raw_sha256':ref.sha(raw),'fixture_sha256':ref.sha(compressed),'external_client_prefix_sha256':ref.sha(prefix),'compiled_client_sha256':ref.sha(client),'registry_dependencies':[{k:v for k,v in d.items()if k not in['source','files']}for d in deps],'bindings':{f.relative_to(ROOT).as_posix():ref.sha(f.read_bytes())for f in sorted(files)},'scope':'Supplemental actual-source equality proof: different insertion shapes, equal-sized distinct keys in both directions, shared-prefix COW before final/middle difference, equal cloned edge value and Eq-default fallback. No original source edits, no dependency/std completion credit.','bounds':{'case_seconds':5,'stdout_bytes':8<<20,'stderr_bytes':1<<20,'raw_fixture_bytes':96<<20,'core_dumps':False}}
 manifest=HERE/'equality-validation.json';data=(json.dumps(m,indent=2)+'\n').encode()
 if args.check:
  if fixture.read_bytes()!=compressed or manifest.read_bytes()!=data:raise RuntimeError('equality evidence/bindings differ')
 else:fixture.write_bytes(compressed);manifest.write_bytes(data)
 print(f'Graph equality: {m["native_observations"]} observations /{len(grouped)} rows; all4 profiles and original tests pass.')
if __name__=='__main__':
 try:main()
 except Exception as e:print(e,file=sys.stderr);sys.exit(1)
