#!/usr/bin/env python3
"""Bounded tool integrity checks; these do not produce expected sort results."""
import importlib.util,os,pathlib,tempfile,unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('sort_generator',HERE/'generate.py')
generator=importlib.util.module_from_spec(spec);spec.loader.exec_module(generator)
class Tools(unittest.TestCase):
 def test_explicit_empty_rustc_fails(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC_NIGHTLY':''}):
   with self.assertRaises(SystemExit):generator.selected('FERRUGO_RUSTC_NIGHTLY','unused')
 def test_explicit_missing_rustc_fails(self):
  with patch.dict(os.environ,{'FERRUGO_RUSTC_NIGHTLY':'/missing/ferrugo-sort-rustc'}):
   with self.assertRaises(SystemExit):generator.selected('FERRUGO_RUSTC_NIGHTLY','unused')
 def test_explicit_empty_go_fails(self):
  with patch.dict(os.environ,{'FERRUGO_GO':''}):
   with self.assertRaises(SystemExit):generator.go_compiler()
 def test_explicit_missing_go_fails(self):
  with patch.dict(os.environ,{'FERRUGO_GO':'/missing/ferrugo-sort-go'}):
   with self.assertRaises(SystemExit):generator.go_compiler()
 def test_bare_compiler_preserves_wrapper_identity(self):
  with tempfile.TemporaryDirectory()as directory:
   path=pathlib.Path(directory)/'chosen-rustc';path.write_text('#!/bin/sh\nexit 0\n');path.chmod(0o700)
   with patch.dict(os.environ,{'PATH':directory,'FERRUGO_RUSTC_NIGHTLY':'chosen-rustc'}):
    self.assertEqual(generator.selected('FERRUGO_RUSTC_NIGHTLY','unused'),str(path))
if __name__=='__main__':unittest.main()
