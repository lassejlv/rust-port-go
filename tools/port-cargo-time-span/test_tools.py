"""Fast fail-closed/source/archive/output checks; no compiler is started."""
import gzip,importlib.util,json,os,pathlib,sys,tempfile,unittest,shutil,re
from unittest import mock
HERE=pathlib.Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
import prepare
import generate

class SelectionTests(unittest.TestCase):
 def test_explicit_empty_compiler_does_not_fall_back(self):
  with mock.patch.dict(os.environ,{'FERRUGO_RUSTC':''}):
   with self.assertRaisesRegex(RuntimeError,'explicitly empty'):prepare.tool('FERRUGO_RUSTC','/valid/fallback')
 def test_bare_path_and_multicall_name_are_preserved(self):
  with tempfile.TemporaryDirectory()as directory:
   root=pathlib.Path(directory);mux=root/'rustup';mux.write_text('#!/bin/sh\nexit 0\n');mux.chmod(0o755);(root/'rustc').symlink_to(mux)
   with mock.patch.dict(os.environ,PATH=str(root),FERRUGO_RUSTC='rustc'):
    self.assertEqual(prepare.tool('FERRUGO_RUSTC','invalid'),str(root/'rustc'))
 def test_fresh_output_rejects_empty_files_symlinks_and_history_overlap(self):
  with tempfile.TemporaryDirectory()as directory:
   root=pathlib.Path(directory);file=root/'file';file.write_text('unowned');link=root/'link';link.symlink_to(root,target_is_directory=True)
   for path in ['',str(file),str(link),str(HERE),str(generate.output_paths(None)[1].parent)]:
    with self.subTest(path=path),self.assertRaises(RuntimeError):generate.output_paths(path)
   fresh=root/'fresh';self.assertFalse(fresh.exists());selected=generate.output_paths(str(fresh));self.assertEqual(selected[0],fresh/'manifest.json');self.assertFalse(fresh.exists())
 def test_bad_output_fails_before_any_reference_operation(self):
  with mock.patch.object(sys,'argv',['generate.py','--output-dir','']),mock.patch.object(prepare,'prepare',side_effect=AssertionError('must reject first')):
   with self.assertRaisesRegex(RuntimeError,'explicitly empty'):generate.main()
 def test_request_corpus_contains_overflow_unicode_and_all_options(self):
  rows=generate.requests();self.assertEqual({q['pattern']for q in rows if q['op']=='F'},set(range(12)))
  for text in ['18446744073709551615 months','18446744073709551615 seconds','18446744073709551616 seconds','1\u00a0second','١second','\0']:
   self.assertTrue(any(q['span']==text for q in rows),text)
 def test_preparation_bad_tool_fails_before_source_or_cache_access(self):
  with mock.patch.object(prepare,'tool',side_effect=RuntimeError('stop before compiler')),mock.patch.object(prepare.subprocess,'check_output',side_effect=AssertionError('must reject before git/version')):
   with self.assertRaisesRegex(RuntimeError,'stop before compiler'):prepare.prepare()
 def test_owned_cache_marker_and_descendant_symlinks_fail_without_writes(self):
  with tempfile.TemporaryDirectory()as directory:
   root=pathlib.Path(directory);actual=root/'actual';actual.mkdir();alias=root/'trusted-base';alias.symlink_to(actual,target_is_directory=True);cache=alias/'cache';cache.mkdir()
   outside=root/'outside';outside.write_bytes(b'unowned')
   with mock.patch.object(prepare,'CACHE',cache):
    # A trusted parent alias is valid; the owned leaf/children are not aliases.
    self.assertEqual(prepare.owned_cache_path(cache/'regular'),cache/'regular')
    for name in ['ownership.json','original','downloads']:
     link=cache/name;link.symlink_to(outside)
     with self.assertRaisesRegex(RuntimeError,'symlink'):prepare.owned_cache_path(link/'child'if name!='ownership.json'else link)
     link.unlink()
    with self.assertRaisesRegex(RuntimeError,'outside'):prepare.owned_cache_path(outside)
   self.assertEqual(outside.read_bytes(),b'unowned')
 def test_foreign_owner_rejects_before_any_child_creation_or_write(self):
  original=(prepare.CACHE/'original'/prepare.SOURCE).read_bytes()
  lock=('[[package]]\nname="anyhow"\nversion="1.0.103"\nchecksum="'+prepare.ANYHOW_SHA+'"\n').encode()
  def readonly(command,**kwargs):
   if command[-1]=='--version':return 'rustc 1.99.1 (58cf8f9c3 2026-10-13)\n'
   if command[-1].endswith(':Cargo.lock'):return lock
   if command[-1].endswith(':src/util/errors.rs'):return b'pub type CargoResult<T> = anyhow::Result<T>;'
   return original
  with tempfile.TemporaryDirectory()as directory:
   cache=pathlib.Path(directory)/'cache';cache.mkdir();marker=cache/'ownership.json';foreign=b'{"tool":"other-owner"}\n';marker.write_bytes(foreign)
   with mock.patch.object(prepare,'CACHE',cache),mock.patch.object(prepare,'tool',return_value='/read-only/compiler'),mock.patch.object(prepare.subprocess,'check_output',side_effect=readonly),mock.patch.object(pathlib.Path,'mkdir',side_effect=AssertionError('must reject before mkdir')):
    with self.assertRaisesRegex(RuntimeError,'foreign time-span cache'):prepare.prepare()
   self.assertEqual(sorted(p.name for p in cache.iterdir()),['ownership.json']);self.assertEqual(marker.read_bytes(),foreign)
 def test_sdk_archive_and_extracted_source_tamper_reject(self):
  archive=prepare.CACHE/'downloads/rust-src-1.99.1.tar.xz'
  with tempfile.TemporaryDirectory()as directory:
   cache=pathlib.Path(directory);downloads=cache/'downloads';downloads.mkdir();target=downloads/archive.name;target.write_bytes(b'corrupt')
   with mock.patch.object(prepare,'CACHE',cache):
    with self.assertRaisesRegex(RuntimeError,'archive differs'):prepare.verify_string_sdk(False)
   shutil.copy2(archive,target)
   changed=cache/'sdk199/library/alloc/src/string.rs';changed.parent.mkdir(parents=True);changed.write_bytes(b'changed')
   with mock.patch.object(prepare,'CACHE',cache):
    with self.assertRaisesRegex(RuntimeError,'extraction changed'):prepare.verify_string_sdk(False)
   self.assertEqual(changed.read_bytes(),b'changed')
 def test_paired_explicit_compiler_selects_slot_before_generic_validation(self):
  import generate_string
  observed=[]
  def selected():observed.append(os.environ['FERRUGO_RUSTC']);raise RuntimeError('stop before source/native')
  with mock.patch.dict(os.environ,{'FERRUGO_RUSTC':'wrong-generic'}),mock.patch.object(sys,'argv',['generate_string.py','--rustc-199','/explicit/SDK199','--rustc-1101','/explicit/SDK1101']),mock.patch.object(prepare,'prepare',side_effect=selected):
   with self.assertRaisesRegex(RuntimeError,'stop before source'):generate_string.main()
   self.assertEqual(os.environ['FERRUGO_RUSTC'],'wrong-generic')
  self.assertEqual(observed,['/explicit/SDK199'])
 def test_all_original_test_literals_and_expected_values_are_mirrored(self):
  source=(prepare.CACHE/'original'/prepare.SOURCE).read_text()
  go=(prepare.ROOT/'port/cargo/src/util/time_span_test.go').read_text()
  literal=r'("(?:\\.|[^"\\])*")'
  rust=re.findall(r'assert_eq!\(maybe_parse_time_span\('+literal+r'\),\s*(?:d\(([^)]*)\)|(None))\);',source)
  target=re.findall(r'timeSpanAssert\(t,\s*'+literal+r',\s*([0-9_* +]+),\s*(true|false)\)',go)
  self.assertEqual(len(rust),21);self.assertEqual(len(target),21)
  for (text,count,none),(go_text,go_count,present)in zip(rust,target):
   self.assertEqual(json.loads(text),json.loads(go_text));self.assertEqual(none!='None',present=='true')
   if not none:self.assertEqual(eval(count,{'__builtins__':{}},{}),eval(go_count,{'__builtins__':{}},{}))
  self.assertEqual(re.findall(r'#\[test\]\s*fn (\w+)',source),['time_spans','time_span_errors'])
  self.assertEqual(re.findall(r'func (Test\w+)\(',go),['TestTimeSpanUpstreamTimeSpans','TestTimeSpanUpstreamTimeSpanErrors'])

if __name__=='__main__':unittest.main()
