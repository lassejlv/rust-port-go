"""Checkout relocation checks for the native backtrace client's remap rule."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('backtrace_relocation_generator', HERE / 'generate_macro_backtrace_context.py')
generator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generator)


class RelocationTests(unittest.TestCase):
    def test_relocated_checkout_has_identical_profile_and_actual_root_remap(self):
        with tempfile.TemporaryDirectory() as directory:
            roots = [Path(directory) / 'first_checkout', Path(directory) / 'relocated checkout']
            serialized, commands = [], []
            profile = {'rustc': 'verified-rustc', 'link_arguments': ['--edition=2024', '-L', 'verified-library']}
            for root in roots:
                source = root / 'tools/port-rust-span-lib/native_macro_backtrace_context.rs'
                binary = root / 'native-output'
                with mock.patch.object(generator, 'ROOT', root):
                    serialized.append(json.dumps(generator.client_location_profile(), sort_keys=True))
                    command = generator.native_compile_command(profile, source, binary, generator.ROOT)
                commands.append(command)
                flag = command.index('--remap-path-prefix')
                self.assertEqual(command[flag + 1], str(root) + '/=')
                self.assertEqual(command[:2], ['verified-rustc', str(source)])
                self.assertEqual(command[2:5], profile['link_arguments'])
                self.assertEqual(command[-2:], ['-o', str(binary)])
                # This is the actual filename rustc remaps in the source file;
                # the observer never performs this projection after execution.
                prefix, destination = command[flag + 1].rsplit('=', 1)
                self.assertTrue(str(source).startswith(prefix))
                self.assertEqual(destination + str(source)[len(prefix):],
                                 'tools/port-rust-span-lib/native_macro_backtrace_context.rs')
                self.assertNotIn(str(root), serialized[-1])
            self.assertEqual(serialized[0], serialized[1])
            self.assertNotEqual(commands[0], commands[1])
            rule = json.loads(serialized[0])['client_root_remap_rule']
            self.assertEqual(rule, {'from': 'generator ROOT + "/"', 'to': ''})

    def test_current_manifest_rule_matches_generator_and_bound_generator_sha(self):
        import hashlib
        manifest = json.loads((HERE / 'macro-backtrace-context-manifest.json').read_text())
        self.assertEqual(manifest['location_profile'], generator.client_location_profile())
        key = 'tools/port-rust-span-lib/generate_macro_backtrace_context.py'
        self.assertEqual(manifest['files'][key], hashlib.sha256((HERE / 'generate_macro_backtrace_context.py').read_bytes()).hexdigest())


if __name__ == '__main__':
    unittest.main()
