"""Source/reference integrity regressions; never replaces native expectations."""
import hashlib, io, json, pathlib, signal, subprocess, tarfile, tempfile, unittest
import prepare,generate
class SourceSelection(unittest.TestCase):
 def test_root_selection_is_scoped_to_identity_impl(self):
  source=b'impl ExpnId { pub const fn root()->ExpnId { ExpnId{} } }\nimpl SyntaxContext {pub const fn root()->SyntaxContext { SyntaxContext::from_u32(0) }}'
  owner=prepare.rust_block(source,b'impl SyntaxContext {')
  start,end,selected=prepare.rust_block(source,b'pub const fn root()',owner[0])
  self.assertGreater(start,owner[0]);self.assertLess(end,owner[1]);self.assertEqual(selected,b'pub const fn root()->SyntaxContext { SyntaxContext::from_u32(0) }')
 def test_static_declaration_keeps_closure_and_complete_expression(self):
  source=b'pub static SPAN_TRACK: AtomicRef<fn(LocalDefId)> = AtomicRef::new(&((|_| {}) as fn(_)));\nfn next(){}'
  _,_,selected=prepare.declaration(source,b'pub static SPAN_TRACK:')
  self.assertEqual(selected,source.split(b'\n')[0])
 def test_braces_in_diagnostics_and_nested_comments(self):
  source=b'impl X {fn f(){ /* outer { /* inner } */ } */ panic!("}} escaped \\\"{"); }} trailing'
  _,_,selected=prepare.rust_block(source,b'impl X {')
  self.assertEqual(selected,source.removesuffix(b' trailing'))
 def test_dependency_lock_rejects_changed_pinned_version(self):
  original=b'[[package]]\nname="x"\nversion="1"\nsource="registry+x"\nchecksum="a"\n'
  prepare.verify_lock(original,original)
  self.assertRaisesRegex(RuntimeError,'differs',prepare.verify_lock,original.replace(b'version="1"',b'version="2"'),original)
 def test_unexpected_local_dependency_cannot_replace_used_algorithm(self):
  root=pathlib.Path('/tmp/reference')
  metadata={'packages':[{'name':'indexmap','source':None,'manifest_path':str(root/'fake-indexmap/Cargo.toml')}]}
  self.assertRaisesRegex(RuntimeError,'unexpected local',prepare.verify_packages,metadata,root)
class RegistryIntegrity(unittest.TestCase):
 def setUp(self):
  self.temporary=tempfile.TemporaryDirectory();self.addCleanup(self.temporary.cleanup)
  self.home=pathlib.Path(self.temporary.name);self.source=self.home/'registry/src/example-123/example-1.0.0';self.source.mkdir(parents=True)
  self.files={'Cargo.toml':b'[package]\nname="example"\nversion="1.0.0"\n','src/lib.rs':b'pub fn original() {}\n'}
  raw=io.BytesIO()
  with tarfile.open(fileobj=raw,mode='w:gz')as archive:
   for name,data in self.files.items():
    entry=tarfile.TarInfo('example-1.0.0/'+name);entry.size=len(data);archive.addfile(entry,io.BytesIO(data));path=self.source/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
  self.archive=self.home/'registry/cache/example-123/example-1.0.0.crate';self.archive.parent.mkdir(parents=True);self.archive.write_bytes(raw.getvalue())
  checksum=hashlib.sha256(raw.getvalue()).hexdigest();self.lock=f'[[package]]\nname="example"\nversion="1.0.0"\nsource="registry+example"\nchecksum="{checksum}"\n'.encode()
  self.metadata={'packages':[{'name':'example','version':'1.0.0','source':'registry+example','manifest_path':str(self.source/'Cargo.toml')}]}
 def verify(self):return prepare.verify_registry(self.metadata,self.lock,self.home)
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
class AbortEvidence(unittest.TestCase):
 def test_retains_only_observed_state_before_actual_abort(self):
  stderr=b'PROBE_START\t{"op":0,"interner":[],"slots":[]}\nPROBE_STEP\t{"result":"zero"}\nPROBE_START\t{"op":1,"interner":[[1,2,null]],"slots":["zero"]}\nPROBE_EVENT\t"map_call:0:0:false"\nthread caused non-unwinding panic. aborting.\n'
  response=generate.fatal_result(subprocess.CompletedProcess([], -signal.SIGABRT,b'',stderr))
  self.assertEqual(response,{'steps':[{'result':'zero'}],'abort':{'op':1,'class':'abort','events':['map_call:0:0:false'],'last_observed':{'interner':[[1,2,None]],'slots':['zero']}}})
 def test_rejects_unexpected_process_termination(self):
  result=subprocess.CompletedProcess([],1,b'',b'PROBE_START\t{"op":0,"interner":[],"slots":[]}\n')
  self.assertRaisesRegex(RuntimeError,'unexpected',generate.fatal_result,result)
class StagedReference(unittest.TestCase):
 def test_symlink_source_directory_cannot_escape_input_freeze(self):
  with tempfile.TemporaryDirectory()as temporary:
   root=pathlib.Path(temporary);(root/'source').mkdir();(root/'source/lib.rs').write_text('original')
   (root/'aliased_source').symlink_to(root/'source',target_is_directory=True)
   self.assertRaisesRegex(RuntimeError,'symlink',generate.reference_inputs,root)
if __name__=='__main__':unittest.main()
