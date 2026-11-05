"""Isolated consuming-callback producer selection tests, without a compiler."""
import gzip
import importlib.util
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('isolated_consuming_generator', HERE / 'generate_consuming_callbacks.py')
tool = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tool)


class ProducerTests(unittest.TestCase):
    def test_historical_raw_drift_fails_without_changing_old_gzip(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.gz'
            old = gzip.compress(b'verbatim old producer\n', mtime=123)
            path.write_bytes(old)
            self.assertEqual(tool.retain_gzip(b'verbatim old producer\n', path, True), old)
            with self.assertRaisesRegex(RuntimeError, 'use --output-dir'):
                tool.retain_gzip(b'new producer filename\n', path, True)
            self.assertEqual(path.read_bytes(), old)
            self.assertEqual(gzip.decompress(tool.retain_gzip(b'new producer filename\n', path, False)), b'new producer filename\n')

    def test_explicit_output_isolated_and_empty_choice_rejected(self):
        original = tool.artifact_paths()
        with tempfile.TemporaryDirectory() as directory:
            chosen = tool.artifact_paths(directory)
            self.assertEqual([p.parent for p in chosen], [Path(directory)] * 3)
            self.assertTrue(set(chosen).isdisjoint(original))
        with self.assertRaisesRegex(RuntimeError, 'explicitly empty'):
            tool.artifact_paths('')

    def test_every_cold_profile_uses_verified_returned_compiler_and_linkage(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            here = root / 'tools/port-rust-span-lib'
            names = ['tools/port-rust-span-lib/native_consuming_callbacks.rs',
                     'tools/port-rust-span-lib/generate_consuming_callbacks.py',
                     'tools/port-rust-span-reference/prepare.py', 'tools/port-rust-span-reference/verify.py',
                     'tools/port-rust-span-reference/sources.json', 'tools/port-rust-span-reference/toolchain.json',
                     'port/rust/compiler/rustc_span/src/lib_consuming_callbacks_oracle_test.go']
            for name in names:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('isolated selected input\n')
            loaded, compiled = [], []

            def select(debug, overflow):
                loaded.append((debug, overflow))
                tag = f'd{int(debug)}o{int(overflow)}'
                return {'source_revision': tool.PIN, 'debug_assertions': debug, 'overflow_checks': overflow,
                        'rustc': 'verified-' + tag, 'link_arguments': ['verified-link-' + tag],
                        'provenance_sha256': tag, 'rustc_version': 'verified official'}, {'PROFILE': tag}, {}

            def execute(command, **kwargs):
                if command[0].startswith('verified-'):
                    tag = kwargs['env']['PROFILE']
                    self.assertEqual(command[0], 'verified-' + tag)
                    self.assertIn('verified-link-' + tag, command)
                    self.assertIn(str(root) + '/=', command)
                    compiled.append(tag)
                    return SimpleNamespace(returncode=0, stderr=b'')
                return SimpleNamespace(returncode=0, stdout=b'construct:false:1:1:false\nfinish:false:3:1:_\n', stderr=b'')

            import sys
            with mock.patch.object(tool, 'ROOT', root), mock.patch.object(tool, 'HERE', here), \
                 mock.patch.object(tool.reference, 'load', side_effect=select), \
                 mock.patch.object(tool.span_wrapper_evidence, 'inputs', return_value=[]), \
                 mock.patch.object(tool.subprocess, 'run', side_effect=execute), \
                 mock.patch.object(tool, 'ROUTES', ['external_open']), mock.patch.object(tool, 'ACTIONS', ['normal']), \
                 mock.patch.object(tool, 'MODES', ['normal']), mock.patch.object(sys, 'argv', ['generate_consuming_callbacks.py']), \
                 mock.patch('builtins.print'):
                tool.main()
            self.assertEqual(loaded, [(False, False), (False, True), (True, False), (True, True)])
            self.assertEqual(compiled, ['d0o0', 'd0o1', 'd1o0', 'd1o1'])
            self.assertFalse((root / '.toolchain').exists())


if __name__ == '__main__':
    unittest.main()
