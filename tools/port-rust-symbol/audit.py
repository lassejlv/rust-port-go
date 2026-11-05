#!/usr/bin/env python3
"""Retain bounded file-to-file provenance; never changes parent inventory."""
import argparse
import hashlib
import json
from pathlib import Path
import re

import native_profile
import generate_predefined

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
BASE='port/rust/compiler/rustc_span/src/'
def sha(data):return hashlib.sha256(data).hexdigest()
def source(path):
 data=native_profile.source_bytes(path)
 return data,{'path':path,'git_blob':hashlib.sha1(f'blob {len(data)}\0'.encode()+data).hexdigest(),'sha256':sha(data),'lines':len(data.splitlines())}
def target(path):return {'path':path,'sha256':sha((ROOT/path).read_bytes())}
def string_literals(body):
 pattern=re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"',re.S)
 return [generate_predefined.rust_string(m.group())for m in pattern.finditer(body)if m.group().startswith('"')]
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 files=[]
 for origin,targets in [
  ('symbol.rs',['symbol.go','symbol_predefined.go','symbol_ident.go','symbol_format.go','symbol_context_ord.go','symbol_panic.go']),
  ('edit_distance.rs',['edit_distance.go','symbol_panic.go']),
 ]:
  data,record=source('compiler/rustc_span/src/'+origin);record.update(scope='complete source implementation',targets=[target(BASE+name)for name in targets]);files.append(record)
 tests=[]
 for origin,go,names,go_names,assertions in [
  ('symbol/tests.rs','symbol/tests_test.go',['interner_tests','interner_get','without_first_quote_test'],['TestInternerTests','TestInternerGet','TestWithoutFirstQuote'],10),
  ('edit_distance/tests.rs','edit_distance/tests_test.go',['test_edit_distance','test_edit_distance_limit','test_method_name_similarity_score','test_find_best_match_for_name','test_precise_algorithm'],['TestEditDistance','TestEditDistanceLimit','TestMethodNameSimilarityScore','TestFindBestMatchForName','TestPreciseAlgorithm'],26),
 ]:
  data,record=source('compiler/rustc_span/src/'+origin);declarations=re.findall(r'#\[test\]\s*fn ([A-Za-z_0-9]+)',data.decode())
  if declarations!=names:raise SystemExit('source test declarations changed: '+origin)
  if len(re.findall(r'assert_(?:eq|ne)!',data.decode()))!=assertions:raise SystemExit('source assertion count changed: '+origin)
  body=(ROOT/BASE/go).read_text();found=re.findall(r'func (Test[A-Za-z_0-9]+)\(',body)
  if found!=go_names:raise SystemExit('Go mirrored declaration order differs: '+go)
  if origin.startswith('symbol'):
   actual=len(re.findall(r'assertSymbol\(t,',body))
  else:actual=len(re.findall(r'assertDistance\(t,',body))+len(re.findall(r'assertMatch\(span\.',body))+len(re.findall(r'if span.EditDistance\(',body))
  if actual!=assertions:raise SystemExit(f'Go source assertion mapping differs: {go}: {actual}')
  rust_literals=string_literals(data.decode());go_literals=iter(string_literals(body))
  for literal in rust_literals:
   if not any(candidate==literal for candidate in go_literals):raise SystemExit(f'original literal order/text changed: {go}: {literal!r}')
  record.update(target=target(BASE+go),test_declaration_order=names,go_declaration_order=go_names,assertion_sites=assertions,string_literal_sequence=rust_literals);tests.append(record)
 runtime=json.loads((ROOT/BASE/'symbol_oracle_manifest.json').read_text());distance=json.loads((ROOT/BASE/'symbol_edit_distance_manifest.json').read_text());unicode=json.loads((ROOT/BASE/'symbol_unicode18_manifest.json').read_text());macro=json.loads((ROOT/BASE/'symbol_macro_tests_manifest.json').read_text());predef=json.loads((ROOT/BASE/'symbol_predefined_manifest.json').read_text())
 borrows=json.loads((HERE/'borrow-validation.json').read_text())
 references=[]
 for path in ['compiler/rustc_macros/src/symbols.rs','compiler/rustc_macros/src/symbols/tests.rs','compiler/rustc_index_macros/src/newtype.rs']:
  _,record=source(path);record.update(scope='unchanged development reference dependency; no whole-file Go port or inventory credit');references.append(record)
 metadata={'schema':1,'rust_revision':native_profile.REVISION,'source_version':native_profile.SOURCE_VERSION,'source_files':files,'source_test_files':tests,'reference_only':references,'partial_std_dependencies':unicode['source_dependencies'],'go_only_support':[target(BASE+'symbol_source_test_witness.go'),{**target(BASE+'lib_bug_runtime_cells.go'),'scope':'borrowed source-cell bridge for used macros.rs bug_impl; no macros/std file credit'},{'path':BASE+'symbol_ident.go','api':'IdentKey/CanonicalKey equality bridge','upstream_api_credit':False}],'unicode_go_bindings':unicode['go_bindings'],'validation':{'compiler_commit':runtime['compiler_commit'],'nightly_date':runtime['nightly_date'],'unicode_version':runtime['unicode_version'],'cfg_release':runtime['cfg_release'],'whole_crate_records':runtime['records'],'operations':runtime['operations'],'original_rust_span_tests':runtime['original_rust_span_tests'],'owned_context_abort_cases':len(runtime['owned_context_aborts']),'edit_distance_records':distance['records'],'edit_distance_profiles':len(distance['profiles']),'unicode_scalars':unicode['scalar_values'],'unicode_writer_casing_blocks':len(unicode['blocks']),'generated_named_constants':predef['profiles'][0]['named_constants'],'predefined_profiles':len(predef['profiles']),'original_macro_tests':len(macro['results'])},'public_trait_boundaries':['ContextOrd<E> consumes arbitrary owned E and field glue under current caller metadata before returning noncapturing comparison function; active session supplied at invocation.','Hash supports required raw-write/finish contract plus every inherited/overridden write_u32 path; hash_slice retains the default element loop.','Symbol/ByteSymbol stable hashing uses complete caller context trait and actual concrete stable hasher.','Ident codecs delegate exact Name then Span operations through complete SpanEncoder/SpanDecoder.','Ident primary AtLocationCell and selected internal cells preserve the same immutable caller-location cell through diagnostic callback and panic hook; scalar At remains a convenience.','Source callbacks use typed Context.PanicAt/PanicFormattedAt for Rust processing-panic timing; raw Go callback-local defers/recover retain host semantics.'],'representation':'Symbol/ByteSymbol are checked four-byte indices; generic Go Option is not advertised as the Rust niche ABI. Ident builtin Go equality is not source PartialEq; Equal/Hash/CanonicalKey preserve the source key.','license':'Rust project MIT OR Apache-2.0; existing core Unicode data retains its Unicode3 license and notice.','inventory':'Parent main ledger owns completion credit; this bounded manifest does not grant whole macro/std/dependency/project credit.'}
 metadata['storage_borrow_followup']={'manifest':target('tools/port-rust-symbol/borrow-validation.json'),'tests':target(BASE+'symbol_borrow_test.go'),'fixture':target(BASE+'testdata/symbol_borrows.txt'),'observations_per_profile':borrows['observations_per_profile'],'profiles':len(borrows['profiles']),'scope':borrows['scope'],'current_runtime_comparator':'No-context entry scrubs borrowed Runtime metadata; explicit entry uses current caller scope, preserving inherited panic flags.'}
 panic=json.loads((HERE/'panic/manifest.json').read_text())
 metadata['ordinary_panic_contract']={'manifest':target('tools/port-rust-symbol/panic/manifest.json'),'tests':target(BASE+'symbol_panic_contract_test.go'),'constructor_profile_tests':target(BASE+'symbol_panic_location_test.go'),'borrowed_bug_impl':target(BASE+'lib_bug_runtime_cells.go'),'fixture':target(BASE+'testdata/symbol_panic.jsonl.gz'),'native_observations':panic['native_observations'],'retained_rows':panic['retained_rows'],'abort_observations':sum(profile['aborts']for profile in panic['profiles']),'arithmetic_profiles':len({(p['debug_assertions'],p['overflow_checks'])for p in panic['profiles']}),'runtime_profile_variants':len(panic['profiles']),'locations':'Verbatim locations observed from the actual selected whole-crate/native caller; semantic dictionaries remain unlocated when absent. Root Ident diagnostic projection profile is retained separately.','payloads':'Generated index/clamp/TLS/arithmetic are static_str; prefill/indexing/RawLifetime/ToString fallback are owned_string; emitted compiler bugs retain ExplicitBug opaque marker.','context_ord':'Owned E borrows current operation metadata and actual mutable cell; field Drop observes body mutations before returning the noncapturing comparator, invoked under two later actual sessions.','constructor_cleanup_reference':'tools/port-rust-span-lib/native_session_inputs.rs validates actual consumed SourceMapInputs cleanup after duplicate prefill panic; owner proof stays separate and receives no extra credit here.'}
 metadata['ordinary_panic_contract']['producer_portability']={'reference_adapter':target('tools/port-rust-symbol/symbol_reference.py'),'selector_tests':target(BASE+'symbol_panic_evidence_test.go'),'verification_tests':[target('tools/port-rust-symbol/test_reference.py'),target('tools/port-rust-symbol/panic/test_portability.py')],'selection':'FERRUGO_SYMBOL_PANIC_PROFILE_DIR explicitly selects a bounded, source-bound native producer bundle in parent and abort children; missing or corrupt selections fail without fallback.','verification':'All whole-crate Symbol clients invoke complete verification for every needed profile, including --prepared; requested cached profiles select flags only.','filenames':'Historical and fresh producer filenames remain verbatim; CI generates and retains an environment-specific profile instead of cross-host absolute-path comparison.'}
 encoded=(json.dumps(metadata,ensure_ascii=False,indent=2)+'\n').encode();path=ROOT/BASE/'symbol_source_manifest.json'
 if args.check:
  if not path.exists()or path.read_bytes()!=encoded:raise SystemExit('Symbol source manifest differs')
 else:path.write_bytes(encoded)
 print(json.dumps({'source_files':len(files),'mirrored_test_files':len(tests),'test_declarations':sum(len(row['test_declaration_order'])for row in tests),'assertion_sites':sum(row['assertion_sites']for row in tests),'native_records':runtime['records']}))
if __name__=='__main__':main()
