#!/usr/bin/env python3
"""Whole-source arbitrary FnOnce Self consumption for two public Span APIs."""
import argparse
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import resource
import signal
import subprocess
import tempfile

import span_wrapper_evidence

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
PIN = '2d20b42b883dcfc014f3c2c56081eeb6091c2179'
spec = importlib.util.spec_from_file_location('consuming_complete_reference', ROOT / 'tools/port-rust-fatal-error/reference.py')
reference = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reference)
ROUTES = ['external_open', 'external_frozen', 'external_bad_state', 'map_inline',
          'map_parent', 'map_pair', 'map_interned', 'map_absent']
ACTIONS = ['normal', 'take_drop', 'forget', 'forget_panic', 'move', 'move_panic',
           'panic', 'drop', 'field', 'drop_field']
MODES = ['normal', 'currentB', 'nil', 'inherited_caught', 'inherited_escape', 'processing']
LIMIT = 32 << 20


def sha(data):
    return hashlib.sha256(data).hexdigest()


def no_core():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def artifact_paths(output_dir=None):
    if output_dir is None:
        directory = HERE
    else:
        if not output_dir:
            raise RuntimeError('consuming producer output directory explicitly empty')
        directory = Path(output_dir).absolute()
    if directory.is_symlink() or directory.exists() and not directory.is_dir():
        raise RuntimeError('invalid consuming producer output directory')
    return directory / 'consuming-callbacks-manifest.json', directory / 'consuming-callbacks.jsonl.gz', directory / 'consuming-callbacks.jsonl'


def retain_gzip(raw, fixture, historical):
    if len(raw) > LIMIT:
        raise RuntimeError('consuming raw fixture exceeds bound')
    if fixture.is_symlink():
        raise RuntimeError('refusing consuming fixture symlink')
    if fixture.exists():
        old = fixture.read_bytes()
        if gzip.decompress(old) == raw:
            return old
        if historical:
            raise RuntimeError('historical consuming producer differs; use --output-dir')
    return gzip.compress(raw, mtime=0)


def source_site(message, request):
    if message == 'call':
        return 'call_' + ('external' if request['route'].startswith('external') else 'map') + '_' + request['action']
    if message == 'cannot access a scoped thread local variable without calling `set` first':
        return 'rustc_span::scoped_tls::with'
    if message.startswith('unexpected state '):
        return 'rustc_span::lib.rs:2372:21'
    return message


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--output-dir')
    args = parser.parse_args()
    manifest, fixture, raw_path = artifact_paths(args.output_dir)
    rows, locations, profiles = [], {}, []
    for debug in [False, True]:
        for overflow in [False, True]:
            profile, env, _ = reference.load(debug, overflow)
            if profile['source_revision'] != PIN or profile['debug_assertions'] is not debug or profile['overflow_checks'] is not overflow:
                raise RuntimeError('consuming original source/profile differs')
            with tempfile.TemporaryDirectory(prefix='ferrugo-consuming-') as directory:
                binary = Path(directory) / 'native'
                command = [profile['rustc'], str(HERE / 'native_consuming_callbacks.rs'), *profile['link_arguments'],
                           '--remap-path-prefix', str(ROOT) + '/=', '-o', str(binary)]
                built = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, timeout=120)
                if built.returncode:
                    raise RuntimeError('consuming original client compilation failed:\n' + built.stderr.decode(errors='replace')[-5000:])
                for route in ROUTES:
                    for action in ACTIONS:
                        for mode in MODES:
                            result = subprocess.run([str(binary), route, action, mode], cwd=ROOT, env=env,
                                                    capture_output=True, timeout=15, preexec_fn=no_core)
                            if result.returncode not in [0, -signal.SIGABRT]:
                                raise RuntimeError(f'consuming native failure {result.returncode}:{route}/{action}/{mode}\n' + result.stderr.decode(errors='replace')[-5000:])
                            output = result.stdout.decode()
                            abort = result.returncode == -signal.SIGABRT
                            if len(result.stdout) + len(result.stderr) > 65536 or not output.startswith('construct:false:1:1:') or not output.endswith('\n'):
                                raise RuntimeError('consuming native event framing/bound differs')
                            if abort and ('after_target:' in output or 'finish:' in output):
                                raise RuntimeError('consuming abort gained post-termination observations')
                            if not abort and 'finish:false:3:' not in output:
                                raise RuntimeError('consuming native caller did not finish')
                            diagnostic = result.stderr.decode()
                            if abort and not diagnostic.endswith(('thread caused non-unwinding panic. aborting.\n', 'thread panicked while processing panic. aborting.\n')):
                                raise RuntimeError('unexpected native consuming abort diagnostic')
                            if not abort and diagnostic:
                                raise RuntimeError('unexpected native consuming stderr')
                            request = {'route': route, 'action': action, 'mode': mode, 'debug': debug, 'overflow': overflow}
                            for line in output.splitlines():
                                if line.startswith('hook:'):
                                    fields = line.split(':')
                                    message = bytes.fromhex(fields[2]).decode()
                                    site, location = source_site(message, request), ':'.join(fields[5:-4])
                                    if site in locations and locations[site] != location:
                                        raise RuntimeError('consuming verbatim source location differs:' + site)
                                    locations[site] = location
                            rows.append({'request': request, 'want': {'abort': abort, 'status': result.returncode, 'output': output, 'native_stderr': diagnostic}})
            profiles.append({'debug': debug, 'overflow': overflow, 'provenance_sha256': profile['provenance_sha256'], 'rustc_version': profile['rustc_version']})
    raw = b''.join((json.dumps(row, separators=(',', ':')) + '\n').encode() for row in rows)
    compressed = retain_gzip(raw, fixture, args.output_dir is None)
    # Complete production closure, all build-tag variants, plus this standalone
    # replay. Unrelated sibling test edits do not change a production binding.
    files = {path for path in span_wrapper_evidence.inputs() if path.suffix != '.go' or not path.name.endswith('_test.go')}
    files.update(ROOT / name for name in ['port/rust/compiler/rustc_span/src/lib_consuming_callbacks_oracle_test.go',
                                         'tools/port-rust-span-lib/generate_consuming_callbacks.py',
                                         'tools/port-rust-span-lib/native_consuming_callbacks.rs',
                                         'tools/port-rust-span-reference/prepare.py', 'tools/port-rust-span-reference/verify.py',
                                         'tools/port-rust-span-reference/sources.json', 'tools/port-rust-span-reference/toolchain.json'])
    metadata = {'kind': 'span-consuming-fnonce-v1', 'revision': PIN, 'records': len(rows),
                'aborts': sum(row['want']['abort'] for row in rows), 'raw': sha(raw), 'gzip': sha(compressed),
                'profiles': profiles, 'locations': locations,
                'location_profile': {'name': 'verbatim-current-producer', 'environment_specific': True,
                                     'client_remap_rule': {'from': 'generator ROOT + "/"', 'to': ''},
                                     'observer_normalization': 'none; original native scoped-tls registry paths are retained verbatim'},
                'scope': 'Actual full-crate public SourceFile::add_external_src and Span::map_ctxt with manual FnOnce(Self), actual Arc/Vec fields, whole-Self Take/Move/forget and forget-then-panic, call/Drop/field failures, called/frozen/unused and four span representations/unset-session, current A/B/nil and late3, inherited/processing states and native SIGABRT complete stdout prefixes. Native stderr retained as native-only diagnostics. No new source-file or standard-library completion credit.',
                'files': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in sorted(files)}}
    outputs = {manifest: (json.dumps(metadata, indent=2, sort_keys=True) + '\n').encode(), fixture: compressed, raw_path: raw}
    for path, data in outputs.items():
        if path.is_symlink():
            raise RuntimeError('refusing consuming producer output symlink')
        if args.check:
            if path.read_bytes() != data:
                raise RuntimeError('consuming producer evidence differs:' + str(path))
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
    print(f'consuming FnOnce:{len(rows)} original observations; {metadata["aborts"]} native SIGABRT prefixes')


if __name__ == '__main__':
    main()
