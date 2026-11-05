"""Complete verified whole-crate linkage for Symbol development clients.

No cached command/artifact path is trusted. Every request runs the existing
complete preparer; Cargo can reuse verified build artifacts. --prepared in
clients is only a compatibility spelling and never disables this verification.
"""
import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
_path = ROOT / 'tools/port-rust-fatal-error/reference.py'
_spec = importlib.util.spec_from_file_location('ferrugo_symbol_verified_reference', _path)
VERIFIED = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(VERIFIED)


def load(debug=True, overflow=True):
    if type(debug) is not bool or type(overflow) is not bool:
        raise ValueError('Symbol reference flags must be boolean')
    return VERIFIED.load(debug, overflow)


def load_requested(path=None):
    if path is None:
        return load()
    path = Path(path)
    path = path if path.is_absolute() else ROOT / path
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 1 << 20:
        raise ValueError('invalid requested Symbol build profile')
    requested = json.loads(path.read_text())
    # The supplied file selects flags only. Its executable and linkage are
    # deliberately ignored, even if it names an apparently existing artifact.
    return load(requested['debug_assertions'], requested['overflow_checks'])


def inputs():
    names = ['tools/port-rust-symbol/symbol_reference.py',
             'tools/port-rust-symbol/native_profile.py',
             'tools/port-rust-fatal-error/reference.py',
             'tools/port-rust-span-reference/prepare.py',
             'tools/port-rust-span-reference/verify.py',
             'tools/port-rust-span-reference/sources.json',
             'tools/port-rust-span-reference/toolchain.json',
             'tools/port-rust-span-reference/Cargo.lock']
    return {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in names}


def compressed_fixture(raw, path, limit=16 << 20):
    """Preserve an existing valid representation, including its CRC, boundedly."""
    if len(raw) > limit:
        raise ValueError('Symbol producer fixture exceeds bound')
    path = Path(path)
    if path.is_symlink():
        raise ValueError('Symbol producer fixture is a symlink')
    if path.exists():
        if not path.is_file() or path.stat().st_size > limit:
            raise ValueError('invalid Symbol producer fixture')
        previous = path.read_bytes()
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(previous)) as stream:
                if stream.read(limit + 1) == raw:
                    return previous
        except (OSError, EOFError):
            pass
    return gzip.compress(raw, mtime=0)
