"""Local-only provenance tests; no upstream checkout or network is required."""

import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("port_inventory", Path(__file__).with_name("inventory.py"))
INVENTORY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(INVENTORY)


class InventoryTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="ferrugo-inventory-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "docs").mkdir()
        self.fixture = b"fn main() {}\n"
        self.pins = {}
        self.make_cache("rust", {
            "compiler/rustc_lexer/src/lib.rs": b"pub fn lexer() {}\n",
            "compiler/rustc_lexer/src/tests.rs": b"#[test] fn case() {}\n",
            "tests/ui/input.rs": self.fixture,
            "Cargo.toml": b"[workspace]\n",
            "README.md": b"reference\n",
        })
        self.make_cache("cargo", {"src/lib.rs": b"pub fn cargo() {}\n", "Cargo.toml": b"[package]\n"})
        self.write_pins()

    def git(self, cache, *args):
        result = subprocess.run(["git", "-C", str(cache), *args], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            self.fail(result.stderr.decode())
        return result.stdout.decode().strip()

    def make_cache(self, name, files):
        cache = self.root / ".toolchain" / ("upstream-" + name)
        cache.mkdir(parents=True)
        self.git(cache, "init", "-b", "main")
        self.git(cache, "remote", "add", "origin", "https://example.invalid/" + name + ".git")
        for source, data in files.items():
            path = cache / source
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        self.git(cache, "add", ".")
        self.git(cache, "-c", "user.name=Inventory test", "-c", "user.email=test@example.invalid", "commit", "-m", "Pinned fixture")
        self.pins[name] = {"repository": "https://example.invalid/" + name, "revision": self.git(cache, "rev-parse", "HEAD")}
        return cache

    def write_pins(self):
        (self.root / INVENTORY.PIN_FILE).write_bytes(INVENTORY.json_bytes(self.pins))

    def row(self, source, name="rust"):
        return next(row for row in INVENTORY.read_rows(self.root / INVENTORY.inventory_path(name, "sources")) if row["source_path"] == source)

    def create(self, relative, data=b"evidence\n"):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        return path

    def mark_library(self, status="ported", validation=None):
        self.create("port/rust/compiler/rustc_lexer/src/lib.go", b"package lexer\n")
        self.create("docs/port.md")
        if validation:
            self.create(validation)
        return INVENTORY.set_status(self.root, "rust", "compiler/rustc_lexer/src/lib.rs", status,
                                    ["docs/port.md"], [validation] if validation else [], None)

    def test_complete_tree_mapping_distinguishes_fixtures_and_harnesses(self):
        manifest = INVENTORY.generate(self.root)
        self.assertEqual(manifest["totals"]["sources"], 4)
        self.assertEqual(manifest["totals"]["support"], 3)
        self.assertEqual(self.row("compiler/rustc_lexer/src/lib.rs")["target_path"], "port/rust/compiler/rustc_lexer/src/lib.go")
        harness = self.row("compiler/rustc_lexer/src/tests.rs")
        self.assertEqual(harness["role"], "test_harness")
        self.assertTrue(harness["target_path"].endswith("tests_test.go"))
        fixture = self.row("tests/ui/input.rs")
        self.assertEqual(fixture["role"], "rust_fixture")
        self.assertEqual(fixture["target_path"], "port/rust/tests/ui/input.rs")
        self.assertEqual(INVENTORY.verify(self.root), manifest)

    def test_generation_is_deterministic_and_preserves_progress(self):
        INVENTORY.generate(self.root)
        before = {path: path.read_bytes() for path in (self.root / INVENTORY.DATA).glob("*.jsonl")}
        before[self.root / INVENTORY.STATUS_FILE] = (self.root / INVENTORY.STATUS_FILE).read_bytes()
        INVENTORY.generate(self.root)
        self.assertEqual(before, {path: path.read_bytes() for path in before})
        self.mark_library()
        INVENTORY.generate(self.root)
        row = self.row("compiler/rustc_lexer/src/lib.rs")
        self.assertEqual(row["status"], "ported")
        self.assertEqual(row["port_evidence"], ["docs/port.md"])
        INVENTORY.verify(self.root)

    def test_completion_needs_destination_and_evidence(self):
        INVENTORY.generate(self.root)
        with self.assertRaisesRegex(INVENTORY.InventoryError, "missing or empty port destination"):
            INVENTORY.set_status(self.root, "rust", "compiler/rustc_lexer/src/lib.rs", "ported", [], [], None)
        self.mark_library()
        with self.assertRaisesRegex(INVENTORY.InventoryError, "validation_evidence"):
            INVENTORY.set_status(self.root, "rust", "compiler/rustc_lexer/src/lib.rs", "validated", [], [], None)
        self.mark_library("validated", "docs/validation.json")
        self.assertTrue(self.row("compiler/rustc_lexer/src/lib.rs")["validated"])
        (self.root / "port/rust/compiler/rustc_lexer/src/lib.go").unlink()
        with self.assertRaisesRegex(INVENTORY.InventoryError, "missing or empty port destination"):
            INVENTORY.verify(self.root)

    def test_missing_evidence_is_detected(self):
        INVENTORY.generate(self.root)
        self.mark_library()
        (self.root / "docs/port.md").unlink()
        with self.assertRaisesRegex(INVENTORY.InventoryError, "missing port_evidence"):
            INVENTORY.verify(self.root)

    def test_upstream_changes_do_not_silently_keep_ported_status(self):
        INVENTORY.generate(self.root)
        self.mark_library()
        cache = self.root / ".toolchain/upstream-rust"
        (cache / "compiler/rustc_lexer/src/lib.rs").write_text("pub fn changed() {}\n")
        self.git(cache, "add", ".")
        self.git(cache, "-c", "user.name=Inventory test", "-c", "user.email=test@example.invalid", "commit", "-m", "Change pinned source")
        self.pins["rust"]["revision"] = self.git(cache, "rev-parse", "HEAD")
        self.write_pins()
        with self.assertRaisesRegex(INVENTORY.InventoryError, "ported source changed"):
            INVENTORY.generate(self.root)
        INVENTORY.generate(self.root, reset_changed=True)
        row = self.row("compiler/rustc_lexer/src/lib.rs")
        self.assertEqual(row["status"], "unported")
        self.assertFalse(row["ported"])
        self.assertIn("Previous blob", row["notes"])
        INVENTORY.verify(self.root)

    def test_mapping_and_hash_tampering_are_detected(self):
        INVENTORY.generate(self.root)
        path = self.root / INVENTORY.inventory_path("rust", "sources")
        rows = INVENTORY.read_rows(path)
        rows[0]["target_path"] = "port/rust/wrong.go"
        path.write_bytes(INVENTORY.rows_bytes(rows))
        with self.assertRaisesRegex(INVENTORY.InventoryError, "target_path"):
            INVENTORY.verify(self.root)

    def test_retained_fixture_needs_original_bytes_and_go_runner(self):
        INVENTORY.generate(self.root)
        target = "port/rust/tests/ui/input.rs"
        self.create(target, b"changed Rust subject\n")
        self.create("docs/fixture-port.md")
        with self.assertRaisesRegex(INVENTORY.InventoryError, "differs from pinned source"):
            INVENTORY.set_status(self.root, "rust", "tests/ui/input.rs", "ported", ["docs/fixture-port.md"], [], None)
        self.create(target, self.fixture)
        with self.assertRaisesRegex(INVENTORY.InventoryError, "Go runner"):
            INVENTORY.set_status(self.root, "rust", "tests/ui/input.rs", "ported", ["docs/fixture-port.md"], [], None)
        runner = "port/rust/tests/runner_test.go"
        self.create(runner, b"package tests\n")
        INVENTORY.set_status(self.root, "rust", "tests/ui/input.rs", "ported", ["docs/fixture-port.md", runner], [], None)
        INVENTORY.verify(self.root)

    def test_target_collisions_reject_before_writing_inventory(self):
        cache = self.root / ".toolchain/upstream-rust"
        (cache / "compiler/rustc_lexer/src/lib.go").write_text("package existing\n")
        self.git(cache, "add", ".")
        self.git(cache, "-c", "user.name=Inventory test", "-c", "user.email=test@example.invalid", "commit", "-m", "Conflicting target")
        self.pins["rust"]["revision"] = self.git(cache, "rev-parse", "HEAD")
        self.write_pins()
        with self.assertRaisesRegex(INVENTORY.InventoryError, "target mapping collision"):
            INVENTORY.generate(self.root)
        self.assertFalse((self.root / INVENTORY.STATUS_FILE).exists())

    def test_gitlinks_cannot_be_claimed_complete_without_expansion(self):
        row = {"source_path": "vendor/library", "target_path": "port/rust/vendor/library", "role": "gitlink",
               "status": "ported", "ported": True, "validated": False, "port_evidence": [], "validation_evidence": []}
        with self.assertRaisesRegex(INVENTORY.InventoryError, "unexpanded"):
            INVENTORY.validate_progress(self.root, row)


if __name__ == "__main__":
    unittest.main()
