import importlib.util,pathlib,unittest
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('codec_tools',HERE/'generate.py');tool=importlib.util.module_from_spec(spec);spec.loader.exec_module(tool)
class Coverage(unittest.TestCase):
 def records(self):
  return b''.join((str(kind)+'\t'+'\t'.join(['0']*11)+'\n').encode()for kind in range(6)for _ in range(322 if kind%2==0 else 236))
 def test_complete_type_matrix(self):self.assertEqual(tool.validate_rows(self.records())['4'],322)
 def test_empty_or_truncated_rejected(self):
  for value in [b'',self.records().split(b'\n',1)[1]]:
   with self.assertRaises(ValueError):tool.validate_rows(value)
 def test_replaced_type_and_bad_protocol_rejected(self):
  for value in [self.records().replace(b'5\t',b'4\t'),self.records().replace(b'0\t0\t',b'0\t',1)]:
   with self.assertRaises(ValueError):tool.validate_rows(value)
if __name__=='__main__':unittest.main()
