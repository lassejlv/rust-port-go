#!/usr/bin/env python3
import gzip,io,json,os
from pathlib import Path
import shutil,subprocess,tempfile,unittest
from unittest.mock import patch

import generate
import prepare

class Tools(unittest.TestCase):
 def test_source_and_license_verification(self):
  proof,_=prepare.verified();self.assertEqual(len(proof['sources']),4)
 def test_archive_source_and_marker_tamper(self):
  _,source=prepare.verified()
  for member in ['channel-rust-nightly.toml','rust-src-nightly.tar.xz','original/library/core/src/str/mod.rs','ownership.json']:
   with self.subTest(member=member),tempfile.TemporaryDirectory()as directory:
    cache=Path(directory)/'cache';shutil.copytree(source,cache);target=cache/member;target.write_bytes(target.read_bytes()+b'\nBAD')
    with self.assertRaises((RuntimeError,json.JSONDecodeError)):prepare.verified(cache)
 def test_unowned_cache_is_preserved(self):
  with tempfile.TemporaryDirectory()as directory:
   cache=Path(directory);sentinel=cache/'unknown';sentinel.write_text('keep')
   with self.assertRaises(RuntimeError):prepare.verified(cache)
   self.assertEqual(sentinel.read_text(),'keep')
 def test_cache_symlink_is_rejected(self):
  with tempfile.TemporaryDirectory()as directory:
   cache=Path(directory)/'link';cache.symlink_to(prepare.ROOT/'.toolchain/ruststr-sdk-1.101',target_is_directory=True)
   with self.assertRaises(RuntimeError):prepare.verified(cache)
 def test_owned_descendant_symlink_rejected(self):
  _,source=prepare.verified()
  with tempfile.TemporaryDirectory()as directory:
   cache=Path(directory)/'cache';shutil.copytree(source,cache);original=cache/'original';shutil.rmtree(original);original.symlink_to(source/'original',target_is_directory=True)
   with self.assertRaisesRegex(RuntimeError,'source parent'):prepare.verified(cache)
 def test_isolated_snapshot_shared_toolchain_live_checks(self):
  actual,_=prepare.selected()
  with tempfile.TemporaryDirectory(prefix='ferrugo-str-snapshot-')as directory:
   root=Path(directory);shutil.copytree(prepare.HERE,root/'tools/port-rust-str',ignore=shutil.ignore_patterns('__pycache__'));shutil.copytree(prepare.ROOT/'port/support/ruststr',root/'port/support/ruststr');target=root/'port/support/rustfmt';target.mkdir(parents=True);shutil.copyfile(prepare.ROOT/'port/support/rustfmt/unicode18.go',target/'unicode18.go');(root/'.toolchain').symlink_to(prepare.ROOT/'.toolchain',target_is_directory=True)
   for name in ['generate.py','generate_ambient.py']:
    result=subprocess.run([shutil.which('python3'),str(root/'tools/port-rust-str'/name),'--check'],cwd=root,env=dict(os.environ,FERRUGO_RUSTC=actual),stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=90)
    self.assertEqual(result.returncode,0,result.stderr.decode())
 def test_explicit_empty_and_missing_compiler(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC':''}):
   with self.assertRaisesRegex(RuntimeError,'explicitly empty'):prepare.selected()
  with self.assertRaisesRegex(RuntimeError,'unavailable'):prepare.selected('/not-a-ferrugo-rustc')
 def test_bare_path_selection_preserves_selected_executable(self):
  actual,_=prepare.selected()
  with tempfile.TemporaryDirectory()as directory:
   entry=Path(directory)/'selected-rustc';entry.symlink_to(actual)
   with patch.dict(os.environ,{'PATH':directory,'FERRUGO_RUSTC':'selected-rustc'}):
    selected,version=prepare.selected();self.assertEqual(selected,str(entry));self.assertIn(prepare.COMMIT,version)
 def test_wrong_compiler_rejected(self):
  with patch('prepare.shutil.which',return_value='/selected'),patch('prepare.subprocess.run',return_value=subprocess.CompletedProcess([],0,b'release: wrong\n',b'')):
   with self.assertRaisesRegex(RuntimeError,'differs'):prepare.selected('rustc')
 def test_gzip_reuse_and_corruption_rebuild(self):
  with tempfile.TemporaryDirectory()as directory:
   path=Path(directory)/'fixture';raw=b'abcd'*100
   # A valid alternative header must survive changes in host Go/Python gzip.
   stream=io.BytesIO()
   with gzip.GzipFile(fileobj=stream,mode='wb',mtime=123,filename='old-profile')as file:file.write(raw)
   original=stream.getvalue();path.write_bytes(original);self.assertEqual(generate.compressed(raw,path),original)
   for invalid in [original[:-1],original[:-8]+b'badcrc!!',original+b'badtrailing',gzip.compress(b'changed')]:
    path.write_bytes(invalid);result=generate.compressed(raw,path);self.assertEqual(gzip.decompress(result),raw);self.assertNotEqual(result,invalid)
 def test_gzip_bounds_and_symlink(self):
  with tempfile.TemporaryDirectory()as directory:
   path=Path(directory)/'fixture';path.write_bytes(gzip.compress(b'x'*128))
   with patch('generate.LIMIT',64):
    result=generate.compressed(b'y',path);self.assertEqual(gzip.decompress(result),b'y')
    with self.assertRaises(RuntimeError):generate.compressed(b'y'*65,path)
   linked=path.with_name('link');linked.symlink_to(path)
   with self.assertRaises(RuntimeError):generate.compressed(b'y',linked)
 def test_rejected_cli_does_not_change_evidence(self):
  paths=[prepare.ROOT/'port/support/ruststr/testdata'/name for name in ['cases.tsv.gz','manifest.json','ambient.tsv','ambient-manifest.json']]
  before={path:path.read_bytes()for path in paths}
  for generator in ['generate.py','generate_ambient.py']:
   result=subprocess.run([shutil.which('python3'),str(prepare.HERE/generator),'--check'],env=dict(os.environ,FERRUGO_RUSTC=''),stdout=subprocess.PIPE,stderr=subprocess.PIPE)
   self.assertNotEqual(result.returncode,0);self.assertIn(b'explicitly empty',result.stderr)
  self.assertEqual(before,{path:path.read_bytes()for path in paths})

if __name__=='__main__':unittest.main()
