import importlib.util,os,pathlib,subprocess,sys,tempfile,unittest
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
SPEC=importlib.util.spec_from_file_location('tested_table_support',HERE/'generate.py');SUPPORT=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(SUPPORT)
class ToolIntegrity(unittest.TestCase):
 def test_explicit_empty_compiler_rejected(self):
  with self.assertRaisesRegex(SystemExit,'empty'):SUPPORT.compiler('',pathlib.Path('/unused'))
 def test_explicit_missing_compiler_rejected(self):
  with self.assertRaisesRegex(SystemExit,'does not resolve'):SUPPORT.compiler('/no/such/ferrugo-rustc',pathlib.Path('/unused'))
 def test_bare_and_relative_compiler_paths(self):
  with tempfile.TemporaryDirectory(prefix='ferrugo-path-proof-')as temporary:
   path=pathlib.Path(temporary)/'chosen-rustc';path.write_text('#!/bin/sh\nexit 0\n');path.chmod(0o755)
   old=os.environ.get('PATH','')
   try:
    os.environ['PATH']=temporary+os.pathsep+old
    self.assertEqual(SUPPORT.compiler('chosen-rustc',pathlib.Path('/unused')),str(path))
    self.assertEqual(SUPPORT.compiler(str(path),pathlib.Path('/unused')),str(path))
   finally:os.environ['PATH']=old
 def test_changed_archive_rejected_before_compilation(self):
  with tempfile.TemporaryDirectory(prefix='ferrugo-tampered-crate-')as temporary:
   home=pathlib.Path(temporary);archive=home/'registry/cache/test/rustc-hash-2.1.1.crate';archive.parent.mkdir(parents=True);archive.write_bytes(b'changed original archive')
   env=dict(os.environ,CARGO_HOME=str(home),FERRUGO_RUSTC='/must/not/be/executed')
   result=subprocess.run([sys.executable,str(HERE/'generate.py'),'--check','--skip-go'],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
   self.assertNotEqual(result.returncode,0);self.assertIn(b'archive checksum differs',result.stderr)
 def test_archive_symlink_rejected(self):
  with tempfile.TemporaryDirectory(prefix='ferrugo-linked-crate-')as temporary:
   home=pathlib.Path(temporary);target=home/'data';target.write_bytes(b'fake');archive=home/'registry/cache/test/rustc-hash-2.1.1.crate';archive.parent.mkdir(parents=True);archive.symlink_to(target)
   result=subprocess.run([sys.executable,str(HERE/'generate.py'),'--check','--skip-go'],cwd=ROOT,env=dict(os.environ,CARGO_HOME=str(home)),stdout=subprocess.PIPE,stderr=subprocess.PIPE)
   self.assertNotEqual(result.returncode,0);self.assertIn(b'Expected one real pinned',result.stderr)
if __name__=='__main__':unittest.main()
