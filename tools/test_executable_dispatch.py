"""Reference tools must preserve argv[0] for rustup-style symlink dispatch."""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


def load(relative):
    spec = importlib.util.spec_from_file_location('dispatch_subject', ROOT / relative)
    module = importlib.util.module_from_spec(spec)
    original_path = sys.path[:]
    try:
        sys.path.insert(0, str((ROOT / relative).parent))
        spec.loader.exec_module(module)
    finally:
        sys.path[:] = original_path
    return module


class ExecutableDispatch(unittest.TestCase):
    def test_path_and_explicit_symlinks_keep_command_name(self):
        subjects = [
            ('tools/port-lexer-debug/generate.py', 'executable'),
            ('tools/port-rust-parse-format/debug/generate.py', 'executable'),
            ('tools/port-rust-span-encoding/prepare.py', 'selected'),
            ('tools/port-rust-binary-search/owned/generate.py', 'owned'),
            ('tools/port-rust-stable-hash/original-tests/run.py', 'plain'),
            ('tools/port-rust-span-reference/prepare.py', 'span-reference'),
            ('tools/port-rust-fingerprint/hasher-contract/generate.py', 'hash-contract'),
            ('tools/port-rust-def-id/generate.py', 'root-executable'),
            ('tools/port-rust-def-id/reference-build/prepare.py', 'root-selected'),
        ]
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            mux = folder / 'multiplexer'
            mux.write_text('#!/bin/sh\nprintf "%s" "${0##*/}"\n')
            mux.chmod(0o755)
            for command in ['rustc', 'rustdoc', 'cargo', 'go']:
                (folder / command).symlink_to(mux.name)
            for relative, selector in subjects:
                subject = load(relative)
                for command in ['rustc', 'rustdoc', 'cargo', 'go']:
                    for explicit in [False, True]:
                        name = str(folder / command) if explicit else command
                        with self.subTest(tool=relative, command=command, explicit=explicit):
                            with patch.dict(os.environ, {'PATH': str(folder), 'FERRUGO_DISPATCH_TEST': name}):
                                if selector == 'hash-contract':
                                    selected = subject.executable(name if explicit else None, 'FERRUGO_DISPATCH_TEST', Path('absent'), ROOT)
                                elif selector.startswith('root-'):
                                    selected = getattr(subject, selector.removeprefix('root-'))(ROOT, 'FERRUGO_DISPATCH_TEST', Path('absent'), command)
                                elif selector == 'span-reference':
                                    selected = subject.executable(name, 'FERRUGO_DISPATCH_TEST', Path('absent'))
                                elif selector == 'plain':
                                    selected = subject.executable(name)
                                elif selector == 'owned':
                                    selected = subject.executable(name, ROOT)
                                else:
                                    selected = getattr(subject, selector)('FERRUGO_DISPATCH_TEST', Path('absent'), command)
                                self.assertEqual(Path(selected).name, command)
                                self.assertEqual(subprocess.check_output([selected]), command.encode())

    def test_hash_and_def_id_relative_symlinks_keep_command_name(self):
        subjects = [
            ('tools/port-rust-fingerprint/hasher-contract/generate.py', 'hash-contract'),
            ('tools/port-rust-def-id/generate.py', 'executable'),
            ('tools/port-rust-def-id/reference-build/prepare.py', 'selected'),
        ]
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            mux = folder / 'multiplexer'
            mux.write_text('#!/bin/sh\nprintf "%s" "${0##*/}"\n')
            mux.chmod(0o755)
            for command in ['rustc', 'rustdoc', 'cargo', 'go']:
                (folder / command).symlink_to(mux.name)
                name = os.path.relpath(folder / command, ROOT)
                for relative, selector in subjects:
                    subject = load(relative)
                    with self.subTest(tool=relative, command=command):
                        with patch.dict(os.environ, {'FERRUGO_DISPATCH_TEST': name}):
                            if selector == 'hash-contract':
                                selected = subject.executable(name, 'FERRUGO_DISPATCH_TEST', Path('absent'), ROOT)
                            else:
                                selected = getattr(subject, selector)(ROOT, 'FERRUGO_DISPATCH_TEST', Path('absent'), command)
                            self.assertEqual(Path(selected).name, command)
                            self.assertEqual(subprocess.check_output([selected]), command.encode())


if __name__ == '__main__':
    unittest.main()
