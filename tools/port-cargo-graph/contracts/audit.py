#!/usr/bin/env python3
"""Bounded whole-Graph and partial dependency provenance, never parent ledger."""
import argparse,hashlib,json,os,pathlib,re,sys
import generate_owned as reference
ROOT=reference.ROOT
HERE=pathlib.Path(__file__).resolve().parent
BASE=ROOT/'port/dependencies/im-rc/v15_1_0'
def sha(data):return hashlib.sha256(data).hexdigest()
def binding(path):return {'path':path.relative_to(ROOT).as_posix(),'sha256':sha(path.read_bytes())}
def write_or_check(path,data,check):
 if check:
  if not path.is_file()or path.read_bytes()!=data:raise RuntimeError('audit binding differs: '+str(path))
 else:path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();env=os.environ.copy();env['GIT_NO_LAZY_FETCH']='1'
 upstream=pathlib.Path(env.get('FERRUGO_CARGO_SOURCE',ROOT/'.toolchain/upstream-cargo'));cargo_home=pathlib.Path(env.get('CARGO_HOME',ROOT/'.toolchain/port-cargo-home'))
 source=reference.run(['git','-C',upstream,'cat-file','blob',reference.BLOB],env)
 if sha(source)!=reference.SOURCE_SHA:raise RuntimeError('original graph source differs')
 deps=reference.dependencies(cargo_home,upstream,env)
 mapped={'src/fakepool.rs':['src/fakepool.go'],'src/config.rs':['src/nodes/btree.go'],'src/nodes/btree.rs':['src/nodes/btree.go'],'src/ord/map.rs':['src/ord/map.go']}
 im=next(d for d in deps if d['name']=='im-rc');archive=[]
 for name,data in sorted(im['files'].items()):
  role='reference_only_unported';targets=[]
  if name in mapped:role='partial_required_graph_operations';targets=[binding(BASE/target)for target in mapped[name]]
  elif name=='LICENCE.md':role='retained_license';targets=[binding(BASE/name)]
  elif name in ['Cargo.toml','Cargo.toml.orig','build.rs','src/lib.rs','src/util.rs']:role='verified_original_build_or_profile_reference'
  archive.append({'path':name,'bytes':len(data),'sha256':sha(data),'role':role,'targets':targets,'whole_file_go_credit':False})
 notices=[]
 for dep in deps:
  for name,data in sorted(dep['files'].items()):
   if '/'not in name and ('licen' in name.lower()or'copying' in name.lower()or name.lower().startswith('notice')):
    target=BASE/'reference-dependencies'/(dep['name']+'-'+dep['version'])/name
    write_or_check(target,data,args.check);notices.append(binding(target))
 inventory={'format':1,'package':'im-rc','version':'15.1.0','archive_sha256':im['archive_sha256'],'archive_files':archive,'supported_profile':'im-rc Rc, no pool, stable no has_specialisation, capacity64/median32','scope':'Partial production dependency operations needed by complete Cargo graph.rs. No entire dependency file, crate, tests or archive completion credit.','unsupported':['unused OrdMap/OrdSet/other collections APIs','pool','threadsafe im variant','has_specialisation including Copy-key search','optional serialization/parallel/property-testing features'],'notices':notices,'original_registry_dependencies':[{k:v for k,v in d.items()if k not in['source','files']}for d in deps],'sized_chunks_operations':'Chunk clone initialized-prefix panic guard and element-drop/move order; used through source-compatible Go node storage, no whole sized-chunks source-file credit.'}
 write_or_check(BASE/'source-manifest.json',(json.dumps(inventory,indent=2)+'\n').encode(),args.check)
 declarations=re.findall(r'#\[test\]\s*fn (\w+)',source.decode())
 if declarations!=['path_to_case','path_to_self','reverse']:raise RuntimeError('original test declaration order differs')
 go_names=re.findall(r'func (TestGraphUpstream\w+)\(', (reference.TARGET/'graph_test.go').read_text())
 if go_names!=['TestGraphUpstreamPathToCase','TestGraphUpstreamPathToSelf','TestGraphUpstreamReverse']:raise RuntimeError('mapped Go test declarations differ')
 validation=json.loads((HERE/'owned-validation.json').read_text())
 std=json.loads((HERE/'std-reference.json').read_text())
 targets=[binding(reference.TARGET/name)for name in ['graph.go','graph_owned.go','graph_owned_traversal.go','graph_btree.go']]
 method_map={'Graph<N,E>':'OwnedGraph[N,E]','new/default':'NewOwnedGraph[WithConfig]','add':'Add[WithContext] consumed N','link':'Link[WithContext] consumed N,N ->GraphEdgeMut[E]','reversed':'Reversed[WithContext], RHS E clone before LHS key clones/default/assignment','contains<Borrow<Q>>':'Contains[WithContext]/OwnedGraphContainsBorrowed','edge':'Edge[WithContext] shared actual E','edges':'Edges[WithContext] eager lookup +lazy borrowed GraphEdgesIter','sort/sort_inner_visit':'Sort[WithContext] ->GraphOwnedNodes with actual N ownership, std BTree marks and return-place cleanup','iter/len':'Iter[WithContext]/Len; exact size hints and source iterator comparison order','is_path_from_to':'IsPathFromTo[WithContext], std BTreeSet shared keys','path_to_bottom/path_to_top/private path_to':'PathToBottom/Top[WithContext], actual query/graph cells and std BTreeMap backlinks','fmt::Debug':'WriteDebugFormatter, borrowed DisplayNode and fallible writer/character calls','print_for_test cfg(debug_assertions)':'private printOwnedForTest, actual stable PartialOrd::lt sort and overridden Ne dedup, last equal reference retained','PartialEq/Eq':'Equal[WithContext], lazy persistent DiffIter with shared subtree skips','Clone/Clone::clone_from default':'Clone/CloneFrom[WithContext], Rc sharing and assignment cleanup','implicit field Drop':'Close[WithContext] owned tree/value destructors'}
 metadata={'format':1,'cargo_revision':reference.PIN,'source':{'path':'src/util/graph.rs','git_blob':reference.BLOB,'sha256':reference.SOURCE_SHA,'lines':len(source.splitlines()),'bytes':len(source)},'primary_targets':targets,'method_mapping':method_map,'original_tests':{'declaration_order':declarations,'mapped_go_declarations':go_names,'mapped_target':binding(reference.TARGET/'graph_test.go'),'primary_replay':binding(reference.TARGET/'graph_owned_oracle_test.go'),'upstream_tests_per_native_profile':3,'extra_upstream_declaration_credit':0},'contract_tests':binding(reference.TARGET/'graph_owned_contract_test.go'),'native_evidence':binding(HERE/'owned-validation.json'),'native_observations':validation['native_observations'],'profiles':[{k:v for k,v in profile.items()if k!='panic_sites'}for profile in validation['profiles']],'partial_dependency_inventory':binding(BASE/'source-manifest.json'),'partial_std_reference':std,'shared_sort_compatibility':'Pinned1.101 scoped rustsort.Stable, reference layout, compared here against actual1.99.1 std sort; no std source-file credit.','panic_locations':'Exact profile metadata is supplied through caller-owned PanicOperations.Locations semantic site keys; no Go stack locations or implicit fabricated Rust build filenames.','source_preconditions':['Ord/Eq/Clone laws and fixed type dictionaries','Rc profile is not Send/Sync; operations remain on the valid serialized source owner','source shared/exclusive borrow and affine ownership rules','no physical Rust allocator/stack/layout identity claim','source callback panics can retain partial mutations/leak prepared return places; no rollback guarantee is invented'],'legacy_value_api':'graph.go/GraphOptions remain effect-free value/trivial-Drop conveniences; all former sorted-slice/eager-clone observations are historical adapter evidence. Full owned/borrowed source contracts use OwnedGraph.','inventory':'Only parent main ledger controls whole Graph file credit. All im-rc/std support slices stay explicitly partial and excluded from whole-file/project counts.'}
 write_or_check(reference.TARGET/'graph_source_manifest.json',(json.dumps(metadata,indent=2)+'\n').encode(),args.check)
 print(f'Graph source audit: {len(declarations)} original tests, {len(archive)} im-rc archive files accounted; no dependency whole-file credit.')
if __name__=='__main__':
 try:main()
 except Exception as e:print(e,file=sys.stderr);sys.exit(1)
