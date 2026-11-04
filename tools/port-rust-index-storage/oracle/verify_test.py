"""Integrity-guard unit tests; these never generate Rust comparison output."""
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch


class RegistryVerification(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="ferrugo-registry-guard-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.home = self.root / "cargo-home"
        self.archive = self.home / "registry/cache/test/fixture-1.0.0.crate"
        self.archive.parent.mkdir(parents=True)
        self.source = self.home / "registry/src/test/fixture-1.0.0"
        files = {"Cargo.toml": b'[package]\nname="fixture"\nversion="1.0.0"\n', "src/lib.rs": b"pub fn original() {}\n"}
        with tarfile.open(self.archive, "w:gz") as archive:
            for name, contents in files.items():
                member = tarfile.TarInfo("fixture-1.0.0/" + name)
                member.size = len(contents)
                archive.addfile(member, io.BytesIO(contents))
                target = self.source / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(contents)
        checksum = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.lock = f'[[package]]\nname="fixture"\nversion="1.0.0"\nsource="registry+https://github.com/rust-lang/crates.io-index"\nchecksum="{checksum}"\n'.encode()
        self.manifest = self.root / "tools/port-rust-index-storage/oracle/Cargo.toml"
        self.manifest.parent.mkdir(parents=True)
        self.manifest.write_text("[package]\n")
        self.manifest.with_name("Cargo.lock").write_bytes(self.lock)
        self.metadata = json.dumps({"packages": [{"name": "fixture", "version": "1.0.0", "source": "registry+https://github.com/rust-lang/crates.io-index", "manifest_path": str(self.source / "Cargo.toml")}]}).encode()

    def execute(self):
        def original_metadata(command):
            if command[0] == "git":
                self.assertIn("2d20b42b883dcfc014f3c2c56081eeb6091c2179:Cargo.lock", command)
                return self.lock
            self.assertEqual(command[0], "fixture-cargo")
            self.assertIn("--locked", command)
            self.assertIn("--offline", command)
            return self.metadata

        output = io.StringIO()
        with patch.object(sys, "argv", ["verify.py", str(self.root), "fixture-cargo"]), patch.dict(os.environ, {"CARGO_HOME": str(self.home)}), patch("subprocess.check_output", side_effect=original_metadata), contextlib.redirect_stdout(output):
            runpy.run_path(str(Path(__file__).with_name("verify.py")), run_name="__main__")
        return json.loads(output.getvalue())

    def test_selected_expanded_sources_and_archive_are_checked(self):
        rows = self.execute()
        self.assertEqual(rows[0]["verified_files"], 2)

    def test_modified_expanded_source_fails(self):
        (self.source / "src/lib.rs").write_text("pub fn replacement() {}\n")
        with self.assertRaisesRegex(SystemExit, "expanded registry source differs"):
            self.execute()

    def test_modified_archive_fails(self):
        self.archive.write_bytes(self.archive.read_bytes() + b"modified")
        with self.assertRaisesRegex(SystemExit, "archive missing/ambiguous/modified"):
            self.execute()

    def test_missing_archive_fails(self):
        self.archive.unlink()
        with self.assertRaisesRegex(SystemExit, "archive missing/ambiguous/modified"):
            self.execute()

    def test_dependency_pin_mismatch_fails(self):
        self.manifest.with_name("Cargo.lock").write_bytes(self.lock.replace(b"checksum=", b"wrong_checksum="))
        with self.assertRaisesRegex(SystemExit, "dependency differs from pinned root lock"):
            self.execute()


if __name__ == "__main__":
    unittest.main()
