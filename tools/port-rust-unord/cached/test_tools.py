#!/usr/bin/env python3
import importlib.util,os,pathlib,unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('cached_generator',HERE/'generate.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class Tools(unittest.TestCase):
 def test_request_coverage(self):
  requests=module.child_specs();self.assertEqual(len(requests),33)
  self.assertTrue(any(r['key_fail']==0 and r['fn_fail']for r in requests))
  self.assertTrue(any(r['cmp_fail']==0 and r['drop_fail']==0 for r in requests))
  self.assertTrue(any(r['moved']and r['key_fail']==0 for r in requests))
 def test_empty_rustc_override(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC_NIGHTLY':''}):
   with self.assertRaises(SystemExit):module.SORT.selected('FERRUGO_RUSTC_NIGHTLY','unused')
 def test_missing_rustc_override(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC_NIGHTLY':'/missing/ferrugo-cached-rustc'}):
   with self.assertRaises(SystemExit):module.SORT.selected('FERRUGO_RUSTC_NIGHTLY','unused')
 def test_empty_go_override(self):
  with patch.dict(os.environ,{'FERRUGO_GO':''}):
   with self.assertRaises(SystemExit):module.SORT.go_compiler()
if __name__=='__main__':unittest.main()
