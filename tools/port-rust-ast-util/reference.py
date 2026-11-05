"""Mandatory complete original-crate preparation for every selected profile."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def digest(body):
    return hashlib.sha256(body).hexdigest()


def bounded_json(path, limit=8 << 20):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise ValueError('missing/unsafe AST reference metadata: ' + str(path))
    return json.loads(path.read_bytes())


def load(debug=True, overflow=True):
    if type(debug) is not bool or type(overflow) is not bool:
        raise ValueError('AST profile flags must be booleans')
    # Never accept a cached linkage file as an alternative to this preparation.
    subprocess.run([sys.executable, str(HERE / 'reference-build/prepare.py'),
                    '--debug-assertions', str(debug).lower(),
                    '--overflow-checks', str(overflow).lower()], cwd=ROOT, check=True)
    name = 'd' + str(int(debug)) + 'o' + str(int(overflow))
    cache = ROOT / '.toolchain/oracles/rustc-ast-util-reference'
    profile = bounded_json(cache / ('build-profile-' + name + '.json'))
    selection = bounded_json(ROOT / 'tools/port-rust-span-reference/toolchain.json')
    if (profile['schema'] != 1 or profile['debug_assertions'] is not debug or
            profile['overflow_checks'] is not overflow or profile['original_ast_tests'] != 6 or
            profile['rustc_version'] != selection['rustc'] or profile['target'] != selection['target'] or
            profile['rustc_commit'] != selection['rustc_commit']):
        raise ValueError('wrong AST reference profile')
    provenance = Path(profile['provenance_path'])
    proof = bounded_json(provenance)
    if digest(provenance.read_bytes()) != profile['provenance_sha256']:
        raise ValueError('AST reference provenance checksum differs')
    for name, expected in proof['tool_sha256'].items():
        if digest((ROOT / name).read_bytes()) != expected:
            raise ValueError('AST reference source binding differs: ' + name)
    for entries in profile['artifacts'].values():
        for entry in entries:
            for filename in entry['filenames']:
                file = Path(filename)
                if file.is_symlink() or not file.is_file() or not file.stat().st_size:
                    raise ValueError('missing AST reference artifact: ' + filename)
    return profile


def environment():
    result = dict(os.environ, RUSTC_BOOTSTRAP='1', CFG_RELEASE='1.101.0', GIT_NO_LAZY_FETCH='1')
    for name in ['RUSTFLAGS', 'CARGO_ENCODED_RUSTFLAGS', 'RUSTC_WRAPPER', 'RUSTC_WORKSPACE_WRAPPER']:
        result.pop(name, None)
    return result
