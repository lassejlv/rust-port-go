#!/usr/bin/env python3
"""Read-only integrity and concrete-specialization regression evidence."""
import gzip,hashlib,importlib.util,json,pathlib,tempfile,unittest
from unittest import mock
HERE=pathlib.Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(data):return hashlib.sha256(data).hexdigest()
class WrapperProof(unittest.TestCase):
 def test_retained_native_coverage_and_bindings(self):
  metadata=json.loads((HERE/'wrapper_manifest.json').read_text());compressed=(ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_vector_wrapper.jsonl.gz').read_bytes();raw=gzip.decompress(compressed);rows=[json.loads(line)for line in raw.splitlines()]
  self.assertEqual(sha(compressed),metadata['fixture_sha256']);self.assertEqual(sha(raw),metadata['raw_sha256']);self.assertEqual(len(rows),528);self.assertEqual(sum(row['outcome']=='abort'for row in rows),32)
  for path,want in metadata['files'].items():self.assertEqual(sha((ROOT/path).read_bytes()),want,path)
  for path,want in metadata['reference_inputs'].items():self.assertEqual(sha((ROOT/path).read_bytes()),want,path)
 def test_direct_and_wrapper_have_distinct_native_calls_and_capacity(self):
  rows=[json.loads(line)for line in gzip.decompress((ROOT/'port/rust/compiler/rustc_data_structures/src/testdata/unord_vector_wrapper.jsonl.gz').read_bytes()).splitlines()]
  for bag in [False,True]:
   relevant=[row for row in rows if row['mode']=='normal'and row['length']==31 and row['fail']==-1 and row['bag']==bag and not row['resume']]
   self.assertEqual(len(relevant),4)
   for row in relevant:
    lines=row['stdout'].splitlines();emits=[line for line in lines if line.startswith('emit:')];self.assertEqual(len(emits),31 if row['kind']%2 else 0)
    if row['kind']<2:self.assertIn('state:mem:'+('31:32:40:'if row['kind']else'0:32:32:'),next(line for line in lines if line.startswith('state:mem:')))
 def test_profile_linkage_is_selected_only_by_complete_verifier(self):
  spec=importlib.util.spec_from_file_location('wrapper_generator',HERE/'generate_wrapper.py');generator=importlib.util.module_from_spec(spec);spec.loader.exec_module(generator)
  with tempfile.TemporaryDirectory()as directory:
   profile=pathlib.Path(directory)/'untrusted.json';profile.write_text(json.dumps({'rustc':'/untrusted/compiler','link_arguments':['--extern','bad=/untrusted/library']}))
   verified=({'rustc':'verified','link_arguments':['verified linkage']},{'verified':'environment'})
   with mock.patch.dict(generator.os.environ,{'FERRUGO_SPAN_REFERENCE_PROFILE':str(profile)}),mock.patch.object(generator.REFERENCE,'load_profile',return_value=verified)as loader:
    self.assertEqual(generator.load_reference(),verified);loader.assert_called_once_with(str(profile))
if __name__=='__main__':unittest.main()
