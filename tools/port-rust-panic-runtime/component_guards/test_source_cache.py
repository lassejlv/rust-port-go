"""Exercise real Git partial-clone hydration without a network dependency."""
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('component_source_cache',HERE/'generate.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class SourceCache(unittest.TestCase):
 def test_explicit_fetch_and_offline_rejection(self):
  with tempfile.TemporaryDirectory(prefix='ferrugo-lexer-source-cache-')as temporary:
   root=Path(temporary);origin=root/'origin';cache=root/'cache'
   env=dict(os.environ,GIT_CONFIG_GLOBAL=os.devnull,GIT_CONFIG_NOSYSTEM='1',GIT_TERMINAL_PROMPT='0',GIT_NO_LAZY_FETCH='1')
   def git(*args):
    p=subprocess.run(['git',*map(str,args)],env=env,capture_output=True,timeout=30)
    if p.returncode:self.fail(p.stderr.decode(errors='replace'))
    return p.stdout.decode().strip()
   git('init','-b','main',origin)
   source=origin/'compiler/rustc_lexer/src/lib.rs';source.parent.mkdir(parents=True);body=b'// Source-cache protocol fixture, never a behavioral reference.\n';source.write_bytes(body)
   git('-C',origin,'add','.');git('-C',origin,'-c','user.name=Source cache test','-c','user.email=cache@example.invalid','commit','-m','Pinned fixture')
   revision=git('-C',origin,'rev-parse','HEAD');blob=git('-C',origin,'rev-parse','HEAD:compiler/rustc_lexer/src/lib.rs')
   git('-C',origin,'config','uploadpack.allowFilter','true');git('-C',origin,'config','uploadpack.allowAnySHA1InWant','true')
   git('clone','--filter=blob:none','--no-checkout',origin.as_uri(),cache)
   with self.assertRaisesRegex(RuntimeError,'--fetch-source'):
    module.read_lexer_source(cache,blob,revision,env,False)
   self.assertEqual(module.read_lexer_source(cache,blob,revision,env,True),body)
   self.assertEqual(module.read_lexer_source(cache,blob,revision,env,False),body)
   with self.assertRaisesRegex(RuntimeError,'revision/path object differs'):
    module.read_lexer_source(cache,revision,revision,env,False)

if __name__=='__main__':unittest.main()
