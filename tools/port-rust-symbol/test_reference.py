"""Verifier selection regression tests; no native artifact is assumed present."""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import symbol_reference


class ReferenceSelection(unittest.TestCase):
    def test_every_request_verifies_even_without_cached_profiles(self):
        verified = ({'rustc': 'verified compiler'}, {'verified': 'environment'}, {})
        with mock.patch.object(symbol_reference.VERIFIED, 'load', return_value=verified) as load:
            for debug in (False, True):
                for overflow in (False, True):
                    self.assertIs(symbol_reference.load(debug, overflow), verified)
            self.assertEqual(load.call_args_list, [mock.call(False, False), mock.call(False, True), mock.call(True, False), mock.call(True, True)])

    def test_requested_json_never_supplies_commands_or_artifacts(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'profile.json'
            path.write_text(json.dumps({'debug_assertions': False, 'overflow_checks': True,
                                        'rustc': '/untrusted/compiler',
                                        'link_arguments': ['--extern', 'rustc_span=/untrusted/source.rlib']}))
            selected = ({'rustc': 'verified', 'link_arguments': ['verified linkage']}, {}, {})
            with mock.patch.object(symbol_reference.VERIFIED, 'load', return_value=selected) as load:
                self.assertIs(symbol_reference.load_requested(path), selected)
                load.assert_called_once_with(False, True)

    def test_invalid_selection_does_not_invoke_verifier_or_fall_back(self):
        with mock.patch.object(symbol_reference.VERIFIED, 'load') as load:
            for debug, overflow in [(1, True), (False, None), ('false', True)]:
                with self.assertRaises(ValueError):
                    symbol_reference.load(debug, overflow)
            with self.assertRaises((ValueError, FileNotFoundError)):
                symbol_reference.load_requested(HERE / 'absent-profile.json')
            load.assert_not_called()


if __name__ == '__main__':
    unittest.main()
