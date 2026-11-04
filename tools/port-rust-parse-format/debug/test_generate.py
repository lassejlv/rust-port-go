"""Verify the reference rejects source/archive substitution before compilation."""
import hashlib, importlib.util, io, pathlib, tarfile, tempfile, unittest
spec=importlib.util.spec_from_file_location('parsefmt_debug_generator',pathlib.Path(__file__).with_name('generate.py'))
generator=importlib.util.module_from_spec(spec);spec.loader.exec_module(generator)
class RegistryIntegrity(unittest.TestCase):
 def setUp(self):
  self.temporary=tempfile.TemporaryDirectory();self.addCleanup(self.temporary.cleanup)
  self.home=pathlib.Path(self.temporary.name);self.source=self.home/'registry/src/example-123/example-1.0.0';self.source.mkdir(parents=True)
  self.files={'Cargo.toml':b'[package]\nname="example"\nversion="1.0.0"\n','src/lib.rs':b'pub fn original() {}\n'}
  raw=io.BytesIO()
  with tarfile.open(fileobj=raw,mode='w:gz') as archive:
   for name,data in self.files.items():
    entry=tarfile.TarInfo('example-1.0.0/'+name);entry.size=len(data);archive.addfile(entry,io.BytesIO(data))
    path=self.source/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
  self.archive=self.home/'registry/cache/example-123/example-1.0.0.crate';self.archive.parent.mkdir(parents=True);self.archive.write_bytes(raw.getvalue())
  checksum=hashlib.sha256(raw.getvalue()).hexdigest()
  self.lock=f'[[package]]\nname="example"\nversion="1.0.0"\nsource="registry+example"\nchecksum="{checksum}"\n'.encode()
  self.metadata={'packages':[{'name':'example','version':'1.0.0','source':'registry+example','manifest_path':str(self.source/'Cargo.toml')}]}
 def verify(self):return generator.verify_registry(self.metadata,self.lock,self.home)
 def test_exact_source_and_cargo_metadata(self):
  (self.source/'.cargo-ok').write_text('{}');self.assertEqual(self.verify()[0]['compiled_files'],2)
 def test_modified_normalized_manifest(self):
  (self.source/'Cargo.toml').write_bytes(b'changed');self.assertRaisesRegex(RuntimeError,'differs',self.verify)
 def test_missing_source(self):
  (self.source/'src/lib.rs').unlink();self.assertRaisesRegex(RuntimeError,'missing',self.verify)
 def test_injected_source(self):
  (self.source/'extra.rs').write_bytes(b'injected');self.assertRaisesRegex(RuntimeError,'differs',self.verify)
 def test_symlink_directory(self):
  (self.source/'injected').symlink_to(self.source/'src',target_is_directory=True);self.assertRaisesRegex(RuntimeError,'symlink',self.verify)
 def test_changed_archive(self):
  self.archive.write_bytes(b'changed');self.assertRaisesRegex(RuntimeError,'checksum',self.verify)
if __name__=='__main__':unittest.main()
