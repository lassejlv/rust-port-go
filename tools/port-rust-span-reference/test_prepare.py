#!/usr/bin/env python3
"""Fast source/tool integrity tests; full native builds remain explicit."""
import copy,hashlib,importlib.util,json,os,pathlib,subprocess,sys,tempfile,unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
spec=importlib.util.spec_from_file_location('whole_span_prepare',HERE/'prepare.py');p=importlib.util.module_from_spec(spec);spec.loader.exec_module(p)

class PreparationIntegrity(unittest.TestCase):
 def test_executable_keeps_multicall_symlink_name(self):
  with tempfile.TemporaryDirectory()as temporary:
   root=pathlib.Path(temporary);dispatcher=root/'rustup';dispatcher.write_text('#!/bin/sh\nexit 0\n');dispatcher.chmod(0o755);(root/'rustc').symlink_to(dispatcher)
   with patch.dict(os.environ,{'PATH':str(root),'FERRUGO_RUSTC':'rustc'}):self.assertEqual(p.executable(None,'FERRUGO_RUSTC',pathlib.Path('unused')),str(root/'rustc'))
   self.assertEqual(p.executable(str(root/'rustc'),'FERRUGO_RUSTC',pathlib.Path('unused')),str(root/'rustc'))
 def test_empty_explicit_override_fails(self):
  with self.assertRaisesRegex(RuntimeError,'explicitly empty'):p.executable('','FERRUGO_RUSTC',pathlib.Path('unused'))
  with patch.dict(os.environ,{'FERRUGO_RUSTC':''}):
   with self.assertRaisesRegex(RuntimeError,'explicitly empty'):p.executable(None,'FERRUGO_RUSTC',pathlib.Path('unused'))
 def test_source_bytes_are_original_and_edits_rejected(self):
  with tempfile.TemporaryDirectory()as temporary:
   root=pathlib.Path(temporary);source=root/'source';source.mkdir();output=root/'output';output.mkdir()
   def git(*args):return subprocess.check_output(['git','-C',str(source),*args],stderr=subprocess.PIPE)
   git('init','-q','--initial-branch=main');git('config','user.name','Source fixture');git('config','user.email','source@example.invalid')
   relative='compiler/fixture/src/lib.rs';body=b'pub fn original()->u32{7}\n';file=source/relative;file.parent.mkdir(parents=True);file.write_bytes(body);git('add','.');git('commit','-qm','pinned fixture')
   revision=git('rev-parse','HEAD').decode().strip();blob=git('rev-parse','HEAD:'+relative).decode().strip()
   graph={'revision':revision,'files':[{'path':relative,'blob':blob,'bytes':len(body),'sha256':hashlib.sha256(body).hexdigest()}]}
   found=p.materialize(graph,source,output,os.environ.copy(),False);self.assertEqual(found[relative],body)
   copied=output/'original'/relative;self.assertEqual(copied.read_bytes(),body);copied.write_text('fabricated')
   with self.assertRaisesRegex(RuntimeError,'cache bytes changed'):p.materialize(graph,source,output,os.environ.copy(),False)
   copied.write_bytes(body);bad=copy.deepcopy(graph);bad['files'][0]['sha256']='0'*64
   with self.assertRaisesRegex(RuntimeError,'source bytes differ'):p.materialize(bad,source,output,os.environ.copy(),False)
   bad=copy.deepcopy(graph);bad['files'][0]['blob']='0'*40
   with self.assertRaisesRegex(RuntimeError,'missing'):p.materialize(bad,source,output,os.environ.copy(),False)
 def test_selected_registry_lock_matches_original(self):
  graph=json.loads((HERE/'sources.json').read_text());original=next(f for f in graph['files']if f['path']=='Cargo.lock')
  cache=p.ROOT/'.toolchain/upstream-rust'
  if not cache.is_dir():self.skipTest('optional pinned source cache unavailable')
  raw=subprocess.check_output(['git','-C',str(cache),'cat-file','blob',original['blob']]);selected=p.lock_packages((HERE/'Cargo.lock').read_bytes());known=p.lock_packages(raw)
  self.assertEqual(len(selected),108);self.assertTrue(all(known.get(key)==value for key,value in selected.items()))

if __name__=='__main__':unittest.main()
