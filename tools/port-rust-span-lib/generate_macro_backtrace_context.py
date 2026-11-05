#!/usr/bin/env python3
"""Retain actual full-crate backtrace iterator ownership/runtime observations."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import resource
import signal
import subprocess
import tempfile

import reference

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
LIMIT = 4 * 1024 * 1024
CASES = ['ok', 'first1', 'second1', 'second2', 'recursive1', 'recursive2',
         'recursive3', 'resume_first1', 'resume_second2', 'resume_recursive3']
MODES = ['normal', 'currentB', 'nil', 'inherited_caught', 'inherited_escape', 'processing']


def sha(data):
    return hashlib.sha256(data).hexdigest()


def no_core():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def native_compile_command(profile, source, binary, execution_root):
    # rustc receives the actual checkout prefix as one argument, including
    # spaces. The retained profile describes the rule rather than that host
    # path; native panic filenames are still observed without rewriting.
    return [profile['rustc'], str(source), *profile['link_arguments'],
            '--remap-path-prefix', str(execution_root) + '/=', '-o', str(binary)]


def client_location_profile():
    return {'name': 'backtrace-native-client-remapped-root',
            'client_root_remap_rule': {'from': 'generator ROOT + "/"', 'to': ''},
            'observer_normalization': 'none; native hook filenames are retained verbatim'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--reference-root', type=Path, default=ROOT / '.toolchain/oracles/rustc-span-reference')
    args = parser.parse_args()
    rows, profiles, locations = [], [], {}
    source = HERE / 'native_macro_backtrace_context.rs'
    for debug in [False, True]:
        for overflow in [False, True]:
            name = f'd{int(debug)}o{int(overflow)}'
            profile, env = reference.load_profile(args.reference_root / f'build-profile-{name}.json')
            if profile['debug_assertions'] != debug or profile['overflow_checks'] != overflow:
                raise RuntimeError('backtrace reference profile differs')
            with tempfile.TemporaryDirectory(prefix='ferrugo-backtrace-') as directory:
                binary = Path(directory) / 'native'
                command = native_compile_command(profile, source, binary, ROOT)
                built = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, timeout=120)
                if built.returncode:
                    raise RuntimeError('backtrace client failed to compile:\n' + built.stderr.decode(errors='replace')[-5000:])
                for selected in CASES:
                    for mode in MODES:
                        result = subprocess.run([str(binary), selected, mode], cwd=ROOT, env=env,
                                                capture_output=True, timeout=15, preexec_fn=no_core)
                        abort = selected != 'ok' and mode in ['inherited_escape', 'processing']
                        expected = -signal.SIGABRT if abort else 0
                        if result.returncode != expected:
                            raise RuntimeError(f'backtrace native status {selected}/{mode}: {result.returncode}, expected {expected}\n' + result.stderr.decode(errors='replace')[-3000:])
                        if len(result.stdout) + len(result.stderr) > 64 * 1024:
                            raise RuntimeError('backtrace native output exceeds bound')
                        output = result.stdout.decode()
                        if not output.endswith('\n') or not output.startswith('construct:false:1:2,2\n'):
                            raise RuntimeError('backtrace native event framing differs')
                        if abort and ('after_target:' in output or 'session_dropped:' in output):
                            raise RuntimeError('backtrace abort gained post-termination observations')
                        if not abort and not output.endswith('session_dropped:false:' + ('1' if mode == 'normal' else '0' if mode == 'nil' else '2') + ':1,1\n'):
                            raise RuntimeError('backtrace owners did not return to actual external Arc counts')
                        for line in output.splitlines():
                            if not line.startswith('hook:'):
                                continue
                            fields = line.split(':')
                            text = bytes.fromhex(fields[2]).decode()
                            location = ':'.join(fields[5:-3])
                            if text in locations and locations[text] != location:
                                raise RuntimeError('backtrace actual source location changed:' + text)
                            locations[text] = location
                        diagnostic = result.stderr.decode()
                        expected_diagnostic = ''
                        if mode == 'inherited_escape' and abort:
                            expected_diagnostic = 'thread caused non-unwinding panic. aborting.\n'
                        elif mode == 'processing' and abort:
                            if not selected.startswith('resume_'):
                                expected_diagnostic = f'panicked at {locations["track"]}:\ntrack\n'
                            expected_diagnostic += 'thread panicked while processing panic. aborting.\n'
                        if diagnostic != expected_diagnostic:
                            raise RuntimeError('backtrace native abort diagnostic/prefix differs:' + selected + '/' + mode)
                        rows.append({'request': {'case': selected, 'mode': mode, 'debug': debug, 'overflow': overflow},
                                     'want': {'abort': abort, 'native_status': result.returncode,
                                              'output': output, 'native_stderr': diagnostic}})
            profiles.append({'name': name, 'reference_provenance_sha256': profile['provenance_sha256'],
                             'rustc_version': profile['rustc_version']})
    raw = b''.join((json.dumps(row, separators=(',', ':')) + '\n').encode() for row in rows)
    fixture = HERE / 'macro-backtrace-context.jsonl.gz'
    compressed = reference.compressed_fixture(raw, fixture, LIMIT)
    base = 'port/rust/compiler/rustc_span/src/'
    # Explicit source inputs. Independent frozen dependency handoffs bind the
    # genuine hygiene/container/hash implementation reached by these wrappers.
    names = ['lib.go', 'span_encoding.go', 'lib_span_context.go', 'lib_span_ops.go',
             'lib_span_combine.go', 'lib_span_track.go', 'lib_session.go',
             'lib_session_scope.go', 'lib_sync.go', 'lib_shared.go', 'lib_pos.go',
             'lib_pos_context.go', 'lib_panic.go', 'source_map_owned.go',
             'hygiene.go', 'hygiene_context.go', 'hygiene_expn.go', 'hygiene_data.go',
             'hygiene_drop.go', 'hygiene_cell.go', 'hygiene_types.go',
             'lib_macro_backtrace_context_oracle_test.go']
    paths = [base + name for name in names] + [
        'port/support/rustfmt/formatter.go', 'port/support/rustfmt/panic.go',
        'tools/port-rust-span-lib/native_macro_backtrace_context.rs',
        'tools/port-rust-span-lib/generate_macro_backtrace_context.py',
        'tools/port-rust-span-reference/sources.json', 'tools/port-rust-span-reference/toolchain.json']
    manifest = {'schema': 1, 'status': 'partial-lib-validation', 'source_revision': reference.PIN,
                'scope': 'Actual unchanged full-crate Span::macro_backtrace Iterator::next: owned ExpnData Arc fields before both tracking reads; failure and recursive-skip state updates; owned return Drop; catch/resume; current operation A/B/nil, inherited caught/escape, processing panic and actual native SIGABRT. Native stderr is retained as native-only diagnostic evidence; Go compares exact complete stdout event prefixes and termination class, never post-abort state. No whole standard-library or compiler completion credit.',
                'location_profile': client_location_profile(),
                'profiles': profiles, 'source_locations': locations,
                'tool_inputs_sha256': reference.tool_inputs(),
                'files': {name: sha((ROOT / name).read_bytes()) for name in paths},
                'records': len(rows), 'aborts': sum(row['want']['abort'] for row in rows),
                'raw_sha256': sha(raw), 'fixture_sha256': sha(compressed)}
    encoded = (json.dumps(manifest, indent=2, sort_keys=True) + '\n').encode()
    outputs = {fixture: compressed, HERE / 'macro-backtrace-context-manifest.json': encoded}
    for path, data in outputs.items():
        if args.check:
            if path.is_symlink() or path.read_bytes() != data:
                raise RuntimeError('backtrace retained evidence differs:' + str(path))
        else:
            if path.is_symlink():
                raise RuntimeError('refusing a backtrace evidence symlink')
            path.write_bytes(data)
    print(f'backtrace iterator:{len(rows)} original observations; {manifest["aborts"]} native SIGABRT prefixes')


if __name__ == '__main__':
    main()
