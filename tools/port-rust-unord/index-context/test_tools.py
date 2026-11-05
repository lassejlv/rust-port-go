import gzip,hashlib,importlib.util,json,pathlib,tempfile,unittest
HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('index_context_tools',HERE/'generate.py');tool=importlib.util.module_from_spec(spec);spec.loader.exec_module(tool)
class Integrity(unittest.TestCase):
 def test_rejects_changed_source(self):
  with tempfile.TemporaryDirectory()as d:
   root=pathlib.Path(d);source=root/'compiler/x.rs';source.parent.mkdir();source.write_bytes(b'original')
   provenance=root/'provenance.json';provenance.write_text(json.dumps({'source_files':[{'path':'compiler/x.rs','sha256':tool.sha(b'original')}]}))
   profile={'original_root':d,'workspace':d,'provenance_path':str(provenance),'provenance_sha256':tool.sha(provenance.read_bytes())};tool.verify_sources(profile)
   source.write_bytes(b'changed')
   with self.assertRaisesRegex(ValueError,'Source graph differs'):tool.verify_sources(profile)
 def test_rejects_symlink_source(self):
  with tempfile.TemporaryDirectory()as d:
   root=pathlib.Path(d);source=root/'compiler/x.rs';source.parent.mkdir();original=root/'original';original.write_bytes(b'original');source.symlink_to(original)
   provenance=root/'provenance.json';provenance.write_text(json.dumps({'source_files':[{'path':'compiler/x.rs','sha256':tool.sha(b'original')}]}))
   with self.assertRaisesRegex(ValueError,'Source graph differs'):tool.verify_sources({'original_root':d,'workspace':d,'provenance_path':str(provenance),'provenance_sha256':tool.sha(provenance.read_bytes())})
 def test_keeps_existing_gzip_bytes(self):
  with tempfile.TemporaryDirectory()as d:
   p=pathlib.Path(d)/'fixture.gz';raw=b'reference\n';p.write_bytes(gzip.compress(raw,mtime=12345));self.assertEqual(tool.compressed_bytes(raw,p),p.read_bytes());self.assertEqual(gzip.decompress(tool.compressed_bytes(b'changed',p)),b'changed')
 def test_rejects_missing_abort_coverage(self):
  rows=[{'outcome':'abort'}]*88+[{'outcome':'completed'}]*140;self.assertEqual(tool.verify_coverage(rows),88)
  with self.assertRaisesRegex(ValueError,'Coverage differs'):tool.verify_coverage(rows[:-1])
  with self.assertRaisesRegex(ValueError,'Coverage differs'):tool.verify_coverage([{'outcome':'completed'}]*228)
if __name__=='__main__':unittest.main()
