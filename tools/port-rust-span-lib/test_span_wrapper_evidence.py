"""Isolated normal-Span reference/binding checks; no Rust or Go build runs."""
from collections import deque
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import unittest
from unittest import mock

import reference
import span_wrapper_evidence

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


def local_production_packages():
    """Read all platform/build-tag variants without selecting host Go files."""
    module = re.search(r'(?m)^module\s+(\S+)', (ROOT / 'go.mod').read_text()).group(1) + '/'
    pending = deque([ROOT / 'port/rust/compiler/rustc_span/src'])
    packages = set()
    while pending:
        package = pending.popleft()
        if package in packages:
            continue
        if not package.is_dir():
            raise AssertionError('missing imported local Go package: ' + str(package))
        packages.add(package)
        for path in package.glob('*.go'):
            if path.name.endswith('_test.go'):
                continue
            source = path.read_text()
            blocks = re.findall(r'(?ms)^[ \t]*import\s*\((.*?)^[ \t]*\)', source)
            blocks += re.findall(r'(?m)^[ \t]*import\s+([^\n]+)', source)
            for block in blocks:
                block = re.sub(r'/\*.*?\*/|//[^\n]*', '', block, flags=re.S)
                for quoted, raw in re.findall(r'"([^"\n]+)"|`([^`]+)`', block):
                    imported = quoted or raw
                    if imported.startswith(module):
                        pending.append(ROOT / imported[len(module):])
    return packages


def assert_local_production_coverage(test, inputs):
    bound = set(inputs)
    required = {path for package in local_production_packages()
                for path in package.glob('*.go') if not path.name.endswith('_test.go')}
    missing = sorted(str(path.relative_to(ROOT)) for path in required - bound)
    test.assertEqual(missing, [], 'unbound local production dependencies: ' + ', '.join(missing))


def generator(stem):
    spec = importlib.util.spec_from_file_location('isolated_span_' + stem, HERE / ('generate_' + stem + '.py'))
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


class BindingTests(unittest.TestCase):
    def test_actual_complete_local_production_import_closure_is_bound(self):
        assert_local_production_coverage(self, span_wrapper_evidence.inputs())

    def test_removing_real_stable_hash_dependency_is_detected(self):
        removed = ROOT / 'port/dependencies/rustc-stable-hash/v0_1_2/src'
        inputs = span_wrapper_evidence.inputs()
        self.assertTrue(any(path.is_relative_to(removed) for path in inputs))
        with self.assertRaisesRegex(AssertionError, 'rustc-stable-hash'):
            assert_local_production_coverage(self, [path for path in inputs if not path.is_relative_to(removed)])

    def test_exact_recursive_package_and_tool_files_are_cwd_independent(self):
        # The helper deliberately binds complete recursive package roots,
        # including tests and inactive platform variants. Reconstruct that
        # conservative set from actual production imports, not its root list.
        expected = {path for package in local_production_packages() for path in package.rglob('*.go')}
        expected.update(ROOT / name for name in reference.tool_inputs())
        expected.add(Path(span_wrapper_evidence.__file__).resolve())
        baseline = span_wrapper_evidence.inputs()
        self.assertEqual(baseline, sorted(expected))
        self.assertTrue(all(path.is_absolute() and path.is_file() for path in baseline))
        previous = Path.cwd()
        with tempfile.TemporaryDirectory() as directory:
            try:
                os.chdir(directory)
                self.assertEqual(span_wrapper_evidence.inputs(), baseline)
            finally:
                os.chdir(previous)


class VerifiedSelectionTests(unittest.TestCase):
    def test_both_prepared_modes_use_fresh_verified_linkage_for_every_profile(self):
        for stem in ['span_geometry', 'span_hygiene']:
            for prepared in [False, True]:
                with self.subTest(generator=stem, prepared=prepared), tempfile.TemporaryDirectory() as directory:
                    tool = generator(stem)
                    root = Path(directory)
                    here = root / 'tools/port-rust-span-lib'
                    here.mkdir(parents=True)
                    (root / 'port/rust/compiler/rustc_span/src/testdata').mkdir(parents=True)
                    for name in ['generate_' + stem + '.py', 'native_' + stem + '.rs']:
                        (here / name).write_text('isolated proof input\n')
                    go_input = root / 'production.go'
                    go_input.write_text('package production\n')
                    calls, compiles = [], []

                    def select(path):
                        calls.append(Path(path).name)
                        name = Path(path).stem.removeprefix('build-profile-')
                        debug, overflow = name[1] == '1', name[3] == '1'
                        return {'rustc': 'verified-' + name, 'link_arguments': ['verified-linkage-' + name],
                                'source_revision': reference.PIN, 'debug_assertions': debug,
                                'overflow_checks': overflow, 'provenance_sha256': 'verified-' + name}, {'VERIFIED': name}

                    def run(command, env, data=None):
                        if data is None:
                            name = env['VERIFIED']
                            self.assertEqual(command[0], 'verified-' + name)
                            self.assertIn('verified-linkage-' + name, command)
                            compiles.append(name)
                            return b''
                        return b'ok\n' * len(data.splitlines())

                    argv = [str(here / ('generate_' + stem + '.py'))]
                    if prepared:
                        argv.append('--prepared')
                    with mock.patch.object(tool, 'ROOT', root), mock.patch.object(tool, 'HERE', here), \
                         mock.patch.object(tool.native_profile, 'compiler_profile', return_value=('untrusted-initial-command', {}, {})), \
                         mock.patch.object(tool.reference, 'load_profile', side_effect=select), \
                         mock.patch.object(tool.span_wrapper_evidence, 'inputs', return_value=[go_input]), \
                         mock.patch.object(tool, 'run', side_effect=run), mock.patch.object(sys, 'argv', argv), \
                         mock.patch('builtins.print'):
                        if stem == 'span_geometry':
                            with mock.patch.object(tool, 'requests', return_value=['lo\tisolated']):
                                tool.main()
                        else:
                            tool.main()
                    self.assertEqual(calls, ['build-profile-d0o0.json', 'build-profile-d0o1.json',
                                             'build-profile-d1o0.json', 'build-profile-d1o1.json'])
                    self.assertEqual(compiles, ['d0o0', 'd0o1', 'd1o0', 'd1o1'])

    def test_prepared_verification_failure_propagates_before_any_native_command(self):
        class RejectedReference(RuntimeError):
            pass
        for stem in ['span_geometry', 'span_hygiene']:
            with self.subTest(generator=stem), tempfile.TemporaryDirectory() as directory:
                tool = generator(stem)
                with mock.patch.object(tool, 'ROOT', Path(directory)), \
                     mock.patch.object(tool.native_profile, 'compiler_profile', return_value=('initial', {}, {})), \
                     mock.patch.object(tool.reference, 'load_profile', side_effect=RejectedReference('changed actual source')), \
                     mock.patch.object(tool, 'run', side_effect=AssertionError('must verify before compilation')), \
                     mock.patch.object(sys, 'argv', [str(tool.__file__), '--prepared', '--check']):
                    with self.assertRaisesRegex(RejectedReference, 'changed actual source'):
                        tool.main()


if __name__ == '__main__':
    unittest.main()
