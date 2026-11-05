#!/usr/bin/env python3
import importlib.util,os,pathlib,unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('panic_generator',HERE/'generate.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class Tools(unittest.TestCase):
 def test_all_guard_families(self):
  self.assertEqual(len(module.ACTIONS),13)
  for name in ['map_hash','map_entry','map_capacity','vec_next','vec_capacity','vec_iter_drop_double','sort_compare','sort_invalid']:
   self.assertIn(name,module.ACTIONS)
 def test_empty_compiler(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC_NIGHTLY':''}):
   with self.assertRaises(SystemExit):module.SDK.selected('FERRUGO_RUSTC_NIGHTLY','unused')
 def test_missing_compiler(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC_NIGHTLY':'/missing/guard-rustc'}):
   with self.assertRaises(SystemExit):module.SDK.selected('FERRUGO_RUSTC_NIGHTLY','unused')
 def test_empty_go(self):
  with patch.dict(os.environ,{'FERRUGO_GO':''}):
   with self.assertRaises(SystemExit):module.SDK.go_compiler()
if __name__=='__main__':unittest.main()
