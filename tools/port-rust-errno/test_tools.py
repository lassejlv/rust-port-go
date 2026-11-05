# SPDX-License-Identifier: MIT OR Apache-2.0
"""Small deterministic/selection guards; the actual proof remains native."""
import importlib.util
import os
from pathlib import Path
import subprocess
import unittest

HERE=Path(__file__).absolute().parent
ROOT=HERE.parents[1]
spec=importlib.util.spec_from_file_location('ferrugo_errno_generator',HERE/'generate.py')
generator=importlib.util.module_from_spec(spec);spec.loader.exec_module(generator)

class ToolContract(unittest.TestCase):
 def test_requests_are_bounded_deterministic_and_cstr_valid(self):
  data=generator.requests()
  self.assertEqual(data,generator.requests())
  rows=data.splitlines();self.assertEqual(len(rows),84123)
  for i,row in enumerate(rows):
   ident,kind,value,fail=row.decode().split('\t');self.assertEqual(int(ident),i)
   if kind=='bytes':self.assertNotIn(0,bytes.fromhex(value));self.assertLessEqual(len(value),254)
   else:self.assertGreaterEqual(int(value),-(1<<31));self.assertLess(int(value),1<<31)
   self.assertLessEqual(int(fail),13)
 def test_empty_explicit_go_is_rejected(self):
  result=subprocess.run(['python3',str(HERE/'generate.py'),'--go',''],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=20)
  self.assertNotEqual(result.returncode,0);self.assertIn(b'explicitly empty',result.stderr)
 def test_empty_explicit_rustc_is_rejected(self):
  result=subprocess.run(['python3',str(HERE/'generate.py'),'--rustc',''],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=20)
  self.assertNotEqual(result.returncode,0);self.assertIn(b'explicitly empty',result.stderr)

if __name__=='__main__':unittest.main()
