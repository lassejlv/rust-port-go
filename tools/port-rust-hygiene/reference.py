"""Fresh whole-crate source/compiler verification for every Hygiene profile.

The accepted preparer is unchanged. Cached profile files never select compiler
commands or linkage; the existing complete loader prepares the requested flags
even when that profile has never existed in this checkout.
"""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'
_path = ROOT / 'tools/port-rust-fatal-error/reference.py'
_spec = importlib.util.spec_from_file_location('hygiene_complete_verified_reference', _path)
_verified = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_verified)


def load(debug=True, overflow=True, rustc=None):
    profile, environment, metadata = _verified.load(debug, overflow, rustc)
    if profile['source_revision'] != PIN or profile['debug_assertions'] is not debug or profile['overflow_checks'] is not overflow:
        raise RuntimeError('verified Hygiene source/profile differs')
    return profile, environment, metadata


def select_main_profile(path=None):
    # --profile is a backward-compatible assertion about the requested build,
    # never authority to execute supplied cached commands/artifacts. No default
    # profile file is needed on a cold runner.
    if path is not None:
        path = Path(path)
        if not path.is_absolute():
            path = ROOT / path
        if path.is_symlink() or path.stat().st_size > 1024 * 1024:
            raise RuntimeError('invalid Hygiene profile selection')
        selected = json.loads(path.read_text())
        if selected.get('source_revision') != PIN or selected.get('debug_assertions') is not True or selected.get('overflow_checks') is not True:
            raise RuntimeError('main Hygiene requires checked/default assertion profile')
    return load(True, True)


def all_profiles(rustc=None):
    return [(debug, overflow, *load(debug, overflow, rustc))
            for debug in [False, True] for overflow in [False, True]]


def tool_inputs():
    names = ['tools/port-rust-hygiene/reference.py',
             'tools/port-rust-fatal-error/reference.py',
             'tools/port-rust-symbol/native_profile.py',
             'tools/port-rust-span-reference/prepare.py',
             'tools/port-rust-span-reference/verify.py']
    return {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in names}
