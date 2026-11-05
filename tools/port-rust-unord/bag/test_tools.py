import importlib.util,pathlib,unittest
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('bag_tools',HERE/'generate.py');tool=importlib.util.module_from_spec(spec);spec.loader.exec_module(tool)
class Coverage(unittest.TestCase):
 def test_empty_and_missing_kind_rejected(self):
  with self.assertRaisesRegex(ValueError,'coverage differs'):tool.validate_counts(b'',{'C':1,'D':1})
  with self.assertRaisesRegex(ValueError,'coverage differs'):tool.validate_counts(b'C\t0\n',{'C':1,'D':1})
 def test_duplicate_kind_cannot_replace_missing_kind(self):
  with self.assertRaisesRegex(ValueError,'coverage differs'):tool.validate_counts(b'C\t0\nC\t1\n',{'C':1,'D':1})
 def test_mixed_categories_accepted(self):self.assertEqual(tool.validate_counts(b'C\t0\nD\t0\n',{'C':1,'D':1}),{'C':1,'D':1})
if __name__=='__main__':unittest.main()
