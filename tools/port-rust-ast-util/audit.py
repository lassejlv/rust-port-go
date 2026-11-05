#!/usr/bin/env python3
"""Read-only pinned source and original assertion preservation audit."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def source(record, revision):
    environment = dict(os.environ, GIT_NO_LAZY_FETCH='1')
    body = subprocess.check_output(['git', '-C', str(ROOT / '.toolchain/upstream-rust'),
                                    'show', revision + ':' + record['path']], env=environment)
    actual = hashlib.sha1(f'blob {len(body)}\0'.encode() + body).hexdigest()
    if actual != record['blob'] or hashlib.sha256(body).hexdigest() != record['sha256'] or len(body) != record['bytes'] or len(body.splitlines()) != record['lines']:
        raise RuntimeError('pinned source identity differs: ' + record['path'])
    return body.decode()


def strings(text):
    return [json.loads(value) for value in re.findall(r'"(?:\\.|[^"\\])*"', text)]


def main():
    manifest = json.loads((HERE / 'source-manifest.json').read_text())
    revision = manifest['revision']
    for record in manifest['source_files']:
        source(record, revision)
        cursor = 1
        for region in record['regions']:
            if region['start'] != cursor or region['end'] < cursor or not region['contract']:
                raise RuntimeError('source regions overlap or omit lines: ' + record['path'])
            cursor = region['end'] + 1
        if cursor != record['lines'] + 1:
            raise RuntimeError('source regions do not cover complete file: ' + record['path'])
        for name in record['targets']:
            if not (ROOT / name).is_file():
                raise RuntimeError('missing complete source target: ' + name)
    for record in manifest['inspected_deferred_files']:
        source(record, revision)
    record = manifest['scoped_dependencies'][0]
    token = source(record, revision)
    block = '\n'.join(token.splitlines()[39:44]) + '\n'
    if hashlib.sha256(block.encode()).hexdigest() != record['block_sha256']:
        raise RuntimeError('scoped CommentKind source block differs')
    record = manifest['original_test']
    original = source(record, revision)
    translated = (ROOT / record['target']).read_text()
    if re.findall(r'#\[test\]\s*fn (\w+)', original) != record['tests']:
        raise RuntimeError('original declaration order changed')
    if re.findall(r'func (Test\w+)\(', translated) != record['go_tests']:
        raise RuntimeError('Go original test declaration order changed')
    if original.count('assert_eq!(') != record['assertions'] or len(re.findall(r'assertBeautified\(t,', translated)) != record['assertions']:
        raise RuntimeError('original assertion sites changed')
    candidates = iter(strings(translated))
    for value in strings(original):
        if not any(candidate == value for candidate in candidates):
            raise RuntimeError('original test literal order/text changed: ' + repr(value))
    print('3 complete utility targets; 1 original child file/5 declarations/10 assertions; scoped CommentKind only; native acceptance pending')


if __name__ == '__main__':
    main()
