#!/usr/bin/env python3
"""Source-integrity and explicit tool selection regressions, without Rust."""
import hashlib,importlib.util,io,json,os,pathlib,tarfile,tempfile,unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('lexer_contract_generator',HERE/'generate.py')
g=importlib.util.module_from_spec(spec);spec.loader.exec_module(g)

class SourceGuards(unittest.TestCase):
 def prepare(self,root):
  source=root/'registry/src/example/crate-1.0.0';source.mkdir(parents=True)
  files={'Cargo.toml':b'[package]\nname="crate"\nversion="1.0.0"\n','src/lib.rs':b'pub fn original() {}\n'}
  raw=io.BytesIO()
  with tarfile.open(fileobj=raw,mode='w:gz')as archive:
   for name,data in files.items():
    info=tarfile.TarInfo('crate-1.0.0/'+name);info.size=len(data);archive.addfile(info,io.BytesIO(data))
    path=source/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
  archive=root/'registry/cache/example/crate-1.0.0.crate';archive.parent.mkdir(parents=True);archive.write_bytes(raw.getvalue())
  lock=f'[[package]]\nname="crate"\nversion="1.0.0"\nsource="registry+https://example"\nchecksum="{hashlib.sha256(raw.getvalue()).hexdigest()}"\n'.encode()
  metadata={'packages':[{'name':'crate','version':'1.0.0','source':'registry+https://example','manifest_path':str(source/'Cargo.toml')}]}
  return source,archive,lock,metadata
 def test_expanded_source_matches_archive(self):
  for alteration in ['none','changed','extra','missing','symlink','archive']:
   with self.subTest(alteration=alteration),tempfile.TemporaryDirectory()as temp:
    root=pathlib.Path(temp);source,archive,lock,metadata=self.prepare(root)
    if alteration=='changed':(source/'src/lib.rs').write_text('pub fn fabricated() {}\n')
    if alteration=='extra':(source/'src/injected.rs').write_text('')
    if alteration=='missing':(source/'src/lib.rs').unlink()
    if alteration=='symlink':(source/'src/lib.rs').unlink();(source/'src/lib.rs').symlink_to(source/'Cargo.toml')
    if alteration=='archive':archive.write_bytes(archive.read_bytes()+b'changed')
    if alteration=='none':self.assertEqual(g.verify_registry(metadata,lock,root)[0]['compiled_files'],2)
    else:
     with self.assertRaises(RuntimeError):g.verify_registry(metadata,lock,root)
 def test_explicit_empty_tool_fails(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC':''}):
   with self.assertRaisesRegex(RuntimeError,'explicitly empty'):g.executable('FERRUGO_RUSTC',pathlib.Path('unused'),'rustc')
 def test_bare_tool_uses_path(self):
  with tempfile.TemporaryDirectory()as temp:
   tool=pathlib.Path(temp)/'custom-rust';tool.write_text('#!/bin/sh\nexit 0\n');tool.chmod(0o755)
   with patch.dict(os.environ,{'FERRUGO_RUSTC':'custom-rust','PATH':temp}):self.assertEqual(g.executable('FERRUGO_RUSTC',pathlib.Path('unused'),'rustc'),str(tool))
 def test_relative_tool_uses_repository(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC':'.toolchain/rust/bin/rustc'}):
   expected=g.ROOT/'.toolchain/rust/bin/rustc'
   if not expected.exists():self.skipTest('optional local compiler not present')
   self.assertEqual(g.executable('FERRUGO_RUSTC',pathlib.Path('unused'),'rustc'),str(expected.absolute()))

if __name__=='__main__':unittest.main()
