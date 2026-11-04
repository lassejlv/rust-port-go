import importlib.util
import io
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest

sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("edition_reference_prepare", Path(__file__).with_name("prepare.py"))
PREPARE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PREPARE)


class RegistrySourceTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.home = Path(self.temporary.name)
        self.archive = self.home / "registry/cache/example/test-crate-1.0.0.crate"
        self.source = self.home / "registry/src/example/test-crate-1.0.0"
        self.archive.parent.mkdir(parents=True)
        self.source.mkdir(parents=True)
        self.files = {"Cargo.toml": b'[package]\nname="test-crate"\n', "src/lib.rs": b"pub fn original() {}\n"}
        with tarfile.open(self.archive, "w:gz") as package:
            for path, data in self.files.items():
                member = tarfile.TarInfo("test-crate-1.0.0/" + path)
                member.size = len(data)
                package.addfile(member, io.BytesIO(data))
                destination = self.source / path
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes(data)
        (self.source / ".cargo-ok").write_bytes(b'{"v":1}')

    def verify(self):
        return PREPARE.verify_registry_source(self.archive, self.home, "test-crate", "1.0.0")

    def test_exact_extraction(self):
        result = self.verify()
        self.assertEqual(result["compiled_source_files"], 2)
        self.assertEqual(len(result["compiled_source_tree_sha256"]), 64)

    def test_changed_normalized_manifest(self):
        (self.source / "Cargo.toml").write_bytes(b"changed manifest")
        with self.assertRaisesRegex(SystemExit, "differs from verified archive"):
            self.verify()

    def test_unexpected_rust_source(self):
        (self.source / "src/injected.rs").write_bytes(b"pub fn injected() {}")
        with self.assertRaisesRegex(SystemExit, "unexpected registry source file"):
            self.verify()

    def test_symlink_source_even_when_bytes_match(self):
        external = self.home / "external.rs"
        external.write_bytes(self.files["src/lib.rs"])
        (self.source / "src/lib.rs").unlink()
        (self.source / "src/lib.rs").symlink_to(external)
        with self.assertRaisesRegex(SystemExit, "non-regular registry source file"):
            self.verify()

    def test_symlink_source_directory(self):
        external = self.home / "external-src"
        (self.source / "src").rename(external)
        (self.source / "src").symlink_to(external, target_is_directory=True)
        with self.assertRaisesRegex(SystemExit, "symlink registry source directory"):
            self.verify()


if __name__ == "__main__":
    unittest.main()
