#!/usr/bin/env python3
import contextlib,gzip,importlib.util,io,json,os,shutil,tempfile,unittest
from pathlib import Path
from unittest import mock
import prepare
import generate

class ToolIntegrity(unittest.TestCase):
 def test_selected_empty(self):
  with self.assertRaisesRegex(RuntimeError,'explicitly empty'):prepare.selected('1.99','')
 def test_selected_missing(self):
  with self.assertRaisesRegex(RuntimeError,'missing'):prepare.selected('1.99','/missing/rustc')
 def test_selected_multiplexer_preserves_rustc_name(self):
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);mux=root/'multiplexer';entry=root/'rustc'
   mux.write_text('#!/bin/sh\ncase "${0##*/}" in rustc) printf "commit-hash: '+prepare.PROFILE['1.99']['commit']+'\\n";; *) exit 42;; esac\n');mux.chmod(0o755);entry.symlink_to(mux.name)
   for value in [str(entry),'rustc']:
    with mock.patch.dict(os.environ,{'PATH':str(root)}):
     selected,version=prepare.selected('1.99',value)
    self.assertEqual(selected,str(entry));self.assertIn(prepare.PROFILE['1.99']['commit'],version)
 def test_selected_wrong_sdk(self):
  path=prepare.ROOT/prepare.PROFILE['1.101']['default']
  if not path.is_file():self.skipTest('specific compiler not installed for negative pin test')
  with self.assertRaisesRegex(RuntimeError,'identity differs'):prepare.selected('1.99',str(path))
 def test_explicit_empty_env(self):
  with mock.patch.dict(os.environ,{'FERRUGO_STDIO_RUSTC_199':''}):
   with self.assertRaisesRegex(RuntimeError,'explicitly empty'):prepare.selected('1.99')
 def test_cache_root_symlink(self):
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);(root/'real').mkdir();(root/'cache').symlink_to(root/'real',target_is_directory=True)
   with self.assertRaisesRegex(RuntimeError,'root is a symlink'):prepare.verified(root/'cache',True)
 def test_cache_descendant_symlink(self):
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);(root/'ownership.json').write_text(json.dumps({'owner':'tools/port-rust-stdio','schema':1}));(root/'fake').symlink_to('/missing')
   with self.assertRaisesRegex(RuntimeError,'descendant is a symlink'):prepare.verified(root,True)
 def test_wrong_marker(self):
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);(root/'ownership.json').write_text('{}')
   with self.assertRaisesRegex(RuntimeError,'ownership differs'):prepare.verified(root,True)
 def test_unowned_nonempty_cache(self):
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);(root/'keep').write_text('foreign')
   with self.assertRaisesRegex(RuntimeError,'unowned'):prepare.verified(root,True)
   self.assertEqual((root/'keep').read_text(),'foreign')
 def test_corrupt_archive(self):
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);(root/'ownership.json').write_text(json.dumps({'owner':'tools/port-rust-stdio','schema':1}));(root/'sdk-1.99.tar.xz').write_bytes(b'corrupt')
   with self.assertRaisesRegex(RuntimeError,'checksum differs'):prepare.verified(root,True)
 def test_missing_offline_archive(self):
  with tempfile.TemporaryDirectory()as tmp:
   with self.assertRaisesRegex(RuntimeError,'missing offline'):prepare.verified(Path(tmp),True)
 def test_trusted_parent_symlink(self):
  real=prepare.ROOT/'.toolchain/stdio-sdk'
  if not real.is_dir():self.skipTest('retained cache not installed')
  with tempfile.TemporaryDirectory()as tmp:
   root=Path(tmp);(root/'.toolchain').symlink_to(real.parent,target_is_directory=True)
   with mock.patch.object(prepare,'ROOT',root):observed=prepare.verified(offline=True)
   self.assertEqual(set(observed),{'1.99','1.101'})
 def test_preserves_alternative_valid_gzip(self):
  with tempfile.TemporaryDirectory()as tmp:
   dest=Path(tmp)/'case.gz';raw=b'original oracle\n'*100;encoded=gzip.compress(raw,compresslevel=1,mtime=123);dest.write_bytes(encoded)
   with mock.patch.object(generate,'DEST',dest):self.assertEqual(generate.compressed(raw),encoded)
 def test_corrupt_gzip_rebuilt(self):
  with tempfile.TemporaryDirectory()as tmp:
   dest=Path(tmp)/'case.gz';raw=b'original oracle\n';encoded=bytearray(gzip.compress(raw,mtime=0));encoded[-8]^=1;dest.write_bytes(encoded)
   with mock.patch.object(generate,'DEST',dest):self.assertEqual(gzip.decompress(generate.compressed(raw)),raw)
 def test_fixture_symlink_rejected(self):
  with tempfile.TemporaryDirectory()as tmp:
   dest=Path(tmp)/'case.gz';dest.symlink_to('/missing')
   with mock.patch.object(generate,'DEST',dest):
    with self.assertRaisesRegex(RuntimeError,'fixture is a symlink'):generate.compressed(b'original')
 def test_raw_bound(self):
  with self.assertRaisesRegex(RuntimeError,'exceeds bound'):generate.compressed(bytes(generate.LIMIT+1))

if __name__=='__main__':unittest.main()
