#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Actual Cargo SDK1.99 io::Error Display vs the selected Unix errno adapter."""
import argparse, hashlib, importlib.util, json, os, platform, shutil, subprocess, tempfile
from pathlib import Path

ROOT = Path(__file__).absolute().parents[3]
HERE = Path(__file__).absolute().parent
COMMIT = '58cf8f9c35b355199727627d07ae34a4e2846d67'

def sha(data): return hashlib.sha256(data).hexdigest()

def selected(name, key, default):
    value = name if name is not None else os.environ.get(key, str(ROOT / default))
    if not value: raise RuntimeError('explicitly empty ' + key)
    executable = shutil.which(value)
    if executable is None: raise RuntimeError('selected executable missing: ' + value)
    return executable

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--rustc'); parser.add_argument('--go')
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    rustc = selected(args.rustc, 'FERRUGO_RUSTC', '.toolchain/rust/bin/rustc')
    go = selected(args.go, 'FERRUGO_GO', '.toolchain/go123-mod-cache/golang.org/toolchain@v0.0.1-go1.23.12.linux-amd64/bin/go')
    compiler = subprocess.check_output([rustc, '-vV'], timeout=20).decode()
    if 'commit-hash: ' + COMMIT not in compiler or 'release: 1.99.1' not in compiler:
        raise RuntimeError('requires the actual selected Cargo SDK1.99.1')
    source = ROOT / 'tools/port-rust-errno'
    spec = importlib.util.spec_from_file_location('graph_errno_requests', source / 'generate.py')
    helper = importlib.util.module_from_spec(spec); spec.loader.exec_module(helper)
    requests = b''.join(row + b'\n' for row in helper.requests().splitlines() if row.split(b'\t')[1] == b'errno')
    env = dict(os.environ, CGO_ENABLED='1')
    with tempfile.TemporaryDirectory(prefix='ferrugo-graph-errno-') as directory:
        native, ported = [Path(directory) / name for name in ['native', 'ported']]
        subprocess.run([rustc, str(HERE / 'native_errno.rs'), '-o', str(native)], cwd=ROOT, env=env, check=True, timeout=60)
        subprocess.run([go, 'build', '-o', str(ported), './tools/port-rust-errno/replay'], cwd=ROOT, env=env, check=True, timeout=120)
        outputs = [subprocess.run([str(binary)], cwd=ROOT, env=env, input=requests, stdout=subprocess.PIPE, check=True, timeout=60).stdout for binary in [native, ported]]
    if outputs[0] != outputs[1]:
        for request, native, ported in zip(requests.splitlines(), outputs[0].splitlines(), outputs[1].splitlines()):
            if native != ported: raise RuntimeError(f'original SDK errno mismatch {request!r}: {native!r} != {ported!r}')
        raise RuntimeError('original SDK errno result count differs')
    paths = [HERE / 'generate_errno.py', HERE / 'native_errno.rs', source / 'generate.py', source / 'replay/main.go',
             ROOT / 'port/cargo/src/util/graph_owned_traversal.go', ROOT / 'port/cargo/src/util/graph_owned.go',
             *sorted((ROOT / 'port/support/rusterrno').glob('*.go'))]
    proof = {'scope': 'Actual SDK1.99.1 io::Error Display used by Cargo stderr; host libc, exact writer calls/failures, signed i32 raw OS errors. Complements genuine whole-graph EPIPE/EBADF proofs; no std-file credit.',
             'compiler': compiler.strip(), 'host': platform.platform(), 'libc': platform.libc_ver(),
             'requests': len(requests.splitlines()), 'request_sha256': sha(requests), 'output_sha256': sha(outputs[0]),
             'files': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in paths}}
    if args.output: args.output.write_text(json.dumps(proof, indent=2) + '\n')
    elif not args.check: (HERE / 'errno-validation.json').write_text(json.dumps(proof, indent=2) + '\n')
    print(proof['requests'], 'actual SDK1.99 native errno/writer records agree on', platform.system())

if __name__ == '__main__': main()
