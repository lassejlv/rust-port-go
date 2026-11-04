"""Preparation restores copied inputs; subsequent verification rejects tampering."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("def_id_prepare", Path(__file__).with_name("prepare.py"))
SUBJECT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SUBJECT)


class LocalSourceIntegrity(unittest.TestCase):
    def test_verification_after_preparation_rejects_tampering(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            expected = {}
            for folder in ["original", "crates", "wrapper"]:
                path = root / folder / "lib.rs"
                path.parent.mkdir()
                path.write_bytes(b"exact source\n")
                expected[path] = path.read_bytes()
            SUBJECT.verify_local(root, expected)
            path = root / "crates/lib.rs"
            path.write_bytes(b"altered source\n")
            with self.assertRaisesRegex(RuntimeError, "source differs"):
                SUBJECT.verify_local(root, expected)
            path.write_bytes(expected[path])
            injected = root / "wrapper/injected.rs"
            injected.write_bytes(b"extra module\n")
            with self.assertRaisesRegex(RuntimeError, "inventory"):
                SUBJECT.verify_local(root, expected)
            injected.unlink()
            path.unlink()
            path.symlink_to(root / "original/lib.rs")
            with self.assertRaisesRegex(RuntimeError, "symlink"):
                SUBJECT.verify_local(root, expected)
            path.unlink()
            path.write_bytes(expected[path])
            build = root / "build.rs"
            build.write_bytes(b"fn main(){}\n")
            with self.assertRaisesRegex(RuntimeError, "auto-discovery"):
                SUBJECT.verify_local(root, expected)


if __name__ == "__main__":
    unittest.main()
