"""True cold default selectors always reach complete verification; no builds."""
import json
from pathlib import Path
from types import SimpleNamespace
import tempfile
import unittest
from unittest import mock

import reference


def profile(debug=False, overflow=False):
    return {'source_revision': reference.PIN, 'unicode': [18,0,0],
            'cfg_bootstrap': False, 'cfg_release': '1.101.0',
            'debug_assertions': debug, 'overflow_checks': overflow,
            'rustc': 'untrusted-cached-command', 'link_arguments': ['untrusted']}


def verified_module(load):
    return SimpleNamespace(load=load), SimpleNamespace(loader=SimpleNamespace(exec_module=lambda value: None))


class ColdReferenceTests(unittest.TestCase):
    def test_all_four_true_cold_canonical_selectors_verify_fresh_linkage(self):
        for debug in (False,True):
            for overflow in (False,True):
                for relative in (False,True):
                    with self.subTest(debug=debug, overflow=overflow, relative=relative), tempfile.TemporaryDirectory() as directory:
                        root=Path(directory);name=f'build-profile-d{int(debug)}o{int(overflow)}.json'
                        path=root/'.toolchain/oracles/rustc-span-reference'/name
                        self.assertFalse(path.exists())
                        selected={**profile(debug,overflow),'rustc':'fresh-verified-command','link_arguments':['fresh-verified-linkage']}
                        environment={'VERIFIED':'source+archives+tests+linkage'}
                        load=mock.Mock(return_value=(selected,environment,{}));module,spec=verified_module(load)
                        with mock.patch.object(reference,'ROOT',root), \
                             mock.patch.object(reference.importlib.util,'spec_from_file_location',return_value=spec) as imported, \
                             mock.patch.object(reference.importlib.util,'module_from_spec',return_value=module):
                            got,env=reference.load_profile(path.relative_to(root) if relative else path)
                        load.assert_called_once_with(debug,overflow)
                        imported.assert_called_once_with('ferrugo_lib_verified_reference',root/'tools/port-rust-fatal-error/reference.py')
                        self.assertIs(got,selected);self.assertIs(env,environment)
                        self.assertEqual(got['link_arguments'],['fresh-verified-linkage'])
                        self.assertFalse(path.exists(),'mock verifier must not have fabricated a profile')

    def test_missing_unknown_noncanonical_and_parent_alias_paths_do_not_infer_flags(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);canonical=root/'.toolchain/oracles/rustc-span-reference'
            paths=[root/'explicit/build-profile-d0o0.json',canonical/'build-profile-d2o0.json',
                   canonical/'build-profile-d0o0.other',canonical/'../rustc-span-reference/build-profile-d0o0.json',
                   root/'build-profile-d1o1.json']
            for path in paths:
                with self.subTest(path=path), mock.patch.object(reference,'ROOT',root), \
                     mock.patch.object(reference.importlib.util,'spec_from_file_location',side_effect=AssertionError('must reject before verifier')):
                    with self.assertRaises(FileNotFoundError):reference.load_profile(path)

    def test_existing_tampered_canonical_profile_does_not_take_cold_shortcut(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);path=root/'.toolchain/oracles/rustc-span-reference/build-profile-d0o0.json';path.parent.mkdir(parents=True)
            values=[b'{',json.dumps({**profile(),'source_revision':'0'*40}).encode(),
                    json.dumps({**profile(),'debug_assertions':'false'}).encode(),b'x'*(1024*1024+1)]
            for body in values:
                path.write_bytes(body)
                with self.subTest(body=len(body)), mock.patch.object(reference,'ROOT',root), \
                     mock.patch.object(reference.importlib.util,'spec_from_file_location',side_effect=AssertionError('tamper must reject first')):
                    with self.assertRaises((RuntimeError,json.JSONDecodeError)):reference.load_profile(path)
                self.assertEqual(path.read_bytes(),body)

    def test_canonical_leaf_symlinks_including_broken_links_reject(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);path=root/'.toolchain/oracles/rustc-span-reference/build-profile-d0o0.json';path.parent.mkdir(parents=True)
            target=root/'target.json'
            for exists in (False,True):
                if exists:target.write_text(json.dumps(profile()))
                path.symlink_to(target)
                with self.subTest(exists=exists), mock.patch.object(reference,'ROOT',root), \
                     mock.patch.object(reference.importlib.util,'spec_from_file_location',side_effect=AssertionError('symlink must reject first')):
                    with self.assertRaisesRegex(RuntimeError,'invalid original reference profile'):reference.load_profile(path)
                path.unlink()

    def test_existing_profile_flags_still_verify_instead_of_using_cached_commands(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);path=root/'.toolchain/oracles/rustc-span-reference/build-profile-d0o0.json';path.parent.mkdir(parents=True)
            path.write_text(json.dumps(profile(True,True)))
            selected={**profile(True,True),'rustc':'fresh','link_arguments':['verified']}
            load=mock.Mock(return_value=(selected,{'FRESH':'1'},{}));module,spec=verified_module(load)
            with mock.patch.object(reference,'ROOT',root), \
                 mock.patch.object(reference.importlib.util,'spec_from_file_location',return_value=spec), \
                 mock.patch.object(reference.importlib.util,'module_from_spec',return_value=module):
                got,_=reference.load_profile(path)
            load.assert_called_once_with(True,True);self.assertIs(got,selected)
            self.assertEqual(json.loads(path.read_text())['rustc'],'untrusted-cached-command')

    def test_cold_verification_failure_and_wrong_returned_profile_propagate(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);path=root/'.toolchain/oracles/rustc-span-reference/build-profile-d1o0.json'
            for result in [RuntimeError('changed actual source'),({**profile(True,False),'source_revision':'0'*40},{},{}), (profile(False,False),{}, {})]:
                load=mock.Mock(side_effect=result) if isinstance(result,Exception) else mock.Mock(return_value=result)
                module,spec=verified_module(load)
                with self.subTest(result=type(result).__name__), mock.patch.object(reference,'ROOT',root), \
                     mock.patch.object(reference.importlib.util,'spec_from_file_location',return_value=spec), \
                     mock.patch.object(reference.importlib.util,'module_from_spec',return_value=module):
                    with self.assertRaisesRegex(RuntimeError,'changed actual source|verified reference differs'):reference.load_profile(path)
                load.assert_called_once_with(True,False);self.assertFalse(path.exists())


if __name__=='__main__':unittest.main()
