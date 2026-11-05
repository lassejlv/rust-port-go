"""Pinned whole-crate reference selection for the lib.rs development clients.

No existing profile or artifact is trusted merely because it exists. Every
load reuses the complete reference preparer's source/archive verification,
build, all 43 upstream tests and external linkage smoke. Explicit tool and
source selections retain that preparer's fail-closed/PATH behavior.
"""
import hashlib
import gzip
import io
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def load_profile(path):
    path = Path(path)
    if not path.is_absolute():
        path = ROOT / path
    if path.is_symlink():
        raise RuntimeError('invalid original reference profile file')
    try:
        size = path.stat().st_size
    except FileNotFoundError:
        # Only these exact default-cache names are cold flag selectors. The
        # complete verifier below must still prepare/verify the real source.
        canonical = ROOT / '.toolchain/oracles/rustc-span-reference'
        choices = {canonical / f'build-profile-d{int(d)}o{int(o)}.json': (d, o)
                   for d in (False, True) for o in (False, True)}
        if path not in choices:
            raise
        debug, overflow = choices[path]
    else:
        if size > 1024 * 1024:
            raise RuntimeError('invalid original reference profile file')
        profile = json.loads(path.read_text())
        if profile.get('source_revision') != PIN or profile.get('unicode') != [18, 0, 0] or profile.get('cfg_bootstrap') is not False or profile.get('cfg_release') != '1.101.0':
            raise RuntimeError('reference source/runtime selection differs')
        debug, overflow = profile.get('debug_assertions'), profile.get('overflow_checks')
        if type(debug) is not bool or type(overflow) is not bool:
            raise RuntimeError('invalid original build flags')
    spec = importlib.util.spec_from_file_location('ferrugo_lib_verified_reference', ROOT / 'tools/port-rust-fatal-error/reference.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    selected, environment, _ = module.load(debug, overflow)
    if selected['source_revision'] != PIN or selected['debug_assertions'] != debug or selected['overflow_checks'] != overflow:
        raise RuntimeError('verified reference differs from requested profile')
    # The supplied profile chooses the build flags. Its artifact paths and
    # compiler command are never executed: preparation returns fresh verified
    # linkage, preventing arbitrary edits to cached JSON from changing source.
    return selected, environment


def tool_inputs():
    return {name: sha((ROOT / name).read_bytes()) for name in [
        'tools/port-rust-span-lib/reference.py',
        'tools/port-rust-fatal-error/reference.py',
        'tools/port-rust-symbol/native_profile.py',
    ]}


def compressed_fixture(raw, destination, limit):
    """Preserve a valid old gzip only after bounded full CRC/raw equality."""
    destination = Path(destination)
    if len(raw) > limit:
        raise RuntimeError('native fixture exceeds bound')
    if destination.is_symlink():
        raise RuntimeError('refusing a symlink fixture')
    if destination.exists() and destination.stat().st_size <= limit:
        old = destination.read_bytes()
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(old)) as stream:
                previous = stream.read(limit + 1)
            if previous == raw:
                return old
        except (OSError, EOFError):
            pass
    return gzip.compress(raw, mtime=0)
