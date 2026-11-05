#!/usr/bin/env python3
"""Generate observations from original full rustc_ast, never Go predictions."""
import argparse
import gzip
import hashlib
import io
import itertools
import json
import os
from pathlib import Path
import resource
import shutil
import subprocess
import sys

import reference

ROOT, HERE = reference.ROOT, reference.HERE
DEFAULT = ROOT / 'port/rust/compiler/rustc_ast/src/util/testdata'
MAX_RAW = 32 << 20


def sha(body):
    return hashlib.sha256(body).hexdigest()


def requests():
    atoms = ['', '*', '**', '***', ' ', '\t', ' *', '\t*', '* ', '**x', '*x',
             'text', ' * x', '  * y', '\t* z', '\r', '\r*', '\u0085', '\u00a0*',
             '\u2000', '\u202f*', '\0[*]']
    texts = ['\n'.join(parts) for parts in itertools.product(atoms, repeat=3)]
    texts += ['', '\n', '\n\n', '*', '*\n*', 'text\r', 'text\r\n', '***\r\n***\r\n',
              '\n * Test \n **  Test\n *   Test\n', '\n * Test\n *  Test\n', '\n let a: *i32;\n *a = 5;\n',
              ' # Returns\n     *\n     ', '\n     * # Returns\n     *\n     ', '\n *     a\n ']
    # All Rust whitespace boundary candidates, including Unicode18's exact
    # White_Space set and adjacent nonmembers; no host Unicode classifier.
    for n in [*range(0, 34), 0x84, 0x85, 0x86, 0x9f, 0xa0, 0xa1, 0x167f, 0x1680,
              0x1681, *range(0x1fff, 0x200c), 0x2027, 0x2028, 0x2029, 0x202a,
              0x202e, 0x202f, 0x2030, 0x205e, 0x205f, 0x2060, 0x2fff, 0x3000,
              0x3001, 0xfeff, 0x11f00, 0x1e6c0]:
        c = chr(n)
        texts += [f'{c}\n * x\n{c}', f'{c}*\n{c}* y\n{c}*', f'{c}\r\n\t*\r\n{c}\r\n']
    seed = 0x8A27B9C4D15E603F
    pieces = atoms + ['[', ']', '\u202a', '\u2069', '\u2603', 'é', '界', '\r\n', '\n', '\n\n']
    for _ in range(2048):
        value = ''
        for _ in range(1 + (seed & 15)):
            seed ^= (seed << 13) & ((1 << 64)-1)
            seed ^= seed >> 7
            seed ^= (seed << 17) & ((1 << 64)-1)
            value += pieces[seed % len(pieces)]
        texts.append(value)
    result = [f'B\t{k}\t{text.encode().hex()}' for text in texts for k in range(2)]
    for style in range(4):
        for position in [0, 1, 0x7fffffff, 0xffffffff]:
            for lines in [[], [''], ['abc'], ['', 'é', '界\0x', '\n*'], ['a'*257, 'z']]:
                result.append('\t'.join(['C', str(style), str(position), *[s.encode().hex() for s in lines]]))
    return ('\n'.join(result) + '\n').encode()


def run(arguments, body=None, environment=None):
    return subprocess.run(list(map(str, arguments)), input=body, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, env=environment, cwd=ROOT, timeout=240,
                          preexec_fn=lambda: resource.setrlimit(resource.RLIMIT_CORE, (0, 0)))


def compressed(raw, target, explicit):
    if len(raw) > MAX_RAW:
        raise ValueError('AST fixture exceeds raw bound')
    if target.is_symlink():
        raise ValueError('AST fixture is a symlink')
    if target.exists():
        if not target.is_file() or target.stat().st_size > MAX_RAW:
            raise ValueError('unsafe existing AST fixture')
        old = target.read_bytes()
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(old)) as stream:
                if stream.read(MAX_RAW + 1) == raw:
                    return old
        except (EOFError, OSError):
            pass
        if not explicit:
            raise ValueError('historical AST observations differ; select a fresh --output-dir producer profile')
    return gzip.compress(raw, mtime=0)


def bindings():
    files = [ROOT / 'port/rust/compiler/rustc_ast/src' / name for name in
             ['token_comment_kind.go', 'util/case.go', 'util/case_types/case.go',
              'util/comments.go', 'util/unicode.go', 'util/comments/tests_test.go',
              'util/native_oracle_test.go']]
    files += sorted(p for p in HERE.rglob('*') if p.is_file() and p.suffix in ['.rs', '.py'] and '__pycache__' not in p.parts)
    files += [HERE / 'source-manifest.json', HERE / 'reference-build/Cargo.lock', HERE / 'reference-build/sources.json']
    files += [ROOT / 'tools/port-rust-span-reference' / name for name in
              ['prepare.py', 'verify.py', 'toolchain.json']]
    files += [ROOT / name for name in ['port/support/ruststr/owned_string.go',
                                      'tools/port-cargo-time-span/string-manifest.json',
                                      'tools/port-cargo-time-span/sdk1101.json']]
    return {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in files}


def generate(args):
    if args.output_dir is not None and not args.output_dir.strip():
        raise ValueError('explicit AST output directory is empty')
    output = Path(args.output_dir).resolve() if args.output_dir is not None else DEFAULT
    if output.is_symlink() or output == ROOT or HERE == output or HERE in output.parents:
        raise ValueError('unsafe AST producer output directory')
    if not args.check:
        output.mkdir(parents=True, exist_ok=True)
    before = bindings()
    request = requests()
    manifest = {'schema': 1, 'revision': '2d20b42b883dcfc014f3c2c56081eeb6091c2179',
                'unicode_version': [18, 0, 0], 'source_bindings': before,
                'target': 'x86_64-unknown-linux-gnu',
                'requests_sha256': sha(request), 'requests': len(request.splitlines()),
                'scalar_slots': 0x110000, 'valid_scalars': 1112064, 'profiles': {}}
    artifacts = {}
    for debug, overflow in itertools.product([False, True], repeat=2):
        if shutil.disk_usage(ROOT).free < 3 << 30:
            raise ValueError('AST reference paused: less than 3 GiB free before the next profile')
        name = 'd' + str(int(debug)) + 'o' + str(int(overflow))
        profile = reference.load(debug, overflow)
        cache = Path(profile['workspace'])
        executable = cache / ('ast-util-' + name)
        arguments = [profile['rustc'], *profile['link_arguments'], '--crate-name', 'ast_util_oracle',
                     '--remap-path-prefix=' + str(HERE / 'oracle.rs') + '=tools/port-rust-ast-util/oracle.rs',
                     HERE / 'oracle.rs', '-o', executable]
        compiled = run(arguments, environment=reference.environment())
        if compiled.returncode:
            raise ValueError('actual AST client compilation failed:\n' + compiled.stderr.decode(errors='replace'))
        result = run([executable], request)
        if result.returncode:
            raise ValueError('actual AST source client failed:\n' + result.stderr.decode(errors='replace'))
        traits = run([executable, 'traits'])
        flow = run([executable, 'flow'])
        if traits.returncode or flow.returncode or len(flow.stdout) != 0x110000:
            raise ValueError('actual AST trait/scalar client failed')
        # Preserve original requests independently for the concrete Clone input.
        rows = result.stdout.splitlines()
        inputs = request.splitlines()
        if len(rows) != len(inputs):
            raise ValueError('actual AST response count differs')
        for i, row in enumerate(rows):
            if row.startswith(b'C\t'):
                rows[i] += b'\t' + b','.join(inputs[i].split(b'\t')[3:])
        raw = b'\n'.join(rows) + b'\n' + traits.stdout
        stem = 'ast-util-' + name
        for suffix, body in [('.tsv.gz', raw), ('-flow.bin.gz', flow.stdout)]:
            filename = stem + suffix
            packed = compressed(body, output / filename, args.output_dir is not None)
            artifacts[filename] = packed
        tags = []
        for mode in ['normal', 'inherited', 'processing']:
            for tag in range(256) if mode == 'normal' else [0, 1, 2, 255]:
                observed = run([executable, 'tag', tag, mode], environment=dict(os.environ, FERRUGO_AST_EVENTS='1'))
                events = [line.removeprefix('EVENT:') for line in observed.stderr.decode().splitlines() if line.startswith('EVENT:')]
                if observed.returncode not in [0, -6]:
                    raise ValueError(f'AST tag {name}/{mode}/{tag} unexpected exit {observed.returncode}: ' + observed.stderr.decode())
                tags.append({'tag': tag, 'mode': mode, 'abort': observed.returncode == -6, 'events': events})
        tag_raw = b''.join((json.dumps(row, separators=(',', ':')) + '\n').encode() for row in tags)
        tag_file = stem + '-tags.jsonl.gz'
        artifacts[tag_file] = compressed(tag_raw, output / tag_file, args.output_dir is not None)
        manifest['profiles'][name] = {'debug_assertions': debug, 'overflow_checks': overflow,
                                    'rustc': profile['rustc_version'], 'original_ast_tests': 6,
                                    'rows': len(raw.splitlines()), 'tags': len(tags),
                                    'aborts': sum(row['abort'] for row in tags),
                                    'fixtures': {filename: {'sha256': sha(body),
                                                           'raw_sha256': sha(gzip.decompress(body)),
                                                           'bytes': len(body), 'raw_bytes': len(gzip.decompress(body))}
                                                 for filename, body in artifacts.items() if filename.startswith(stem)}}
        free = shutil.disk_usage(ROOT).free
        print(name, manifest['profiles'][name]['rows'], 'rows, all scalars,', len(tags),
              'tag cases; free disk', round(free/(1 << 30), 2), 'GiB', flush=True)
        if free < 3 << 30:
            raise ValueError('AST reference paused: less than 3 GiB free after this profile')
    if before != bindings():
        raise ValueError('AST producer source changed during generation')
    artifacts['ast-util-manifest.json'] = (json.dumps(manifest, indent=2) + '\n').encode()
    for filename, body in artifacts.items():
        path = output / filename
        if args.check:
            if path.is_symlink() or not path.is_file() or path.read_bytes() != body:
                raise ValueError('AST producer drift: ' + filename)
        else:
            path.write_bytes(body)
    print('Original AST source evidence', 'checked' if args.check else 'written', output)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--output-dir')
    parser.add_argument('--prepared', action='store_true', help='compatibility only; complete verification always runs')
    try:
        generate(parser.parse_args())
    except Exception as error:
        print(error, file=sys.stderr)
        sys.exit(1)
