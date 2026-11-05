#!/usr/bin/env python3
"""Actual complete-crate public context and owned FnOnce entry points."""
import argparse
import json
from pathlib import Path
import resource
import signal
import subprocess
import tempfile

import reference
import span_wrapper_evidence

ROOT = reference.ROOT
HERE = Path(__file__).resolve().parent
LIMIT = 16 * 1024 * 1024
SURFACE = [name + '_' + state for name in ['def','call','mixed','ed2015','ed2018','ed2021','ed2024','subst'] for state in ['present','absent']]
SURFACE += ['subst_dummy','lines_ok','lines_bad0','lines_bad3']
OWNED = ['default','default_present','meta_present','meta_absent','set','set_present','with_present','with_absent','create','create_present','create_ctorfail','if_present','if_absent']
MODES = ['normal','currentB','nil','unwinding','processing']


def no_core():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def expected_abort(op, mode, call, drop, strategy):
    if op in OWNED:
        missing = op in ['meta_absent','with_absent','default_present','set_present','create_present','create_ctorfail']
        if mode == 'processing':
            return missing or call or (drop and strategy == 'normal')
        return drop and (missing or call and strategy != 'early')
    return mode == 'processing' and (op.endswith('absent') or op in ['lines_bad0','lines_bad3'])


def evidence_paths(output_dir=None):
    if output_dir is None:
        return (HERE/'remaining-context-manifest.json',
                ROOT/'port/rust/compiler/rustc_span/src/testdata/lib-remaining-context.jsonl.gz',None)
    if output_dir == '':
        raise RuntimeError('remaining-context --output-dir explicitly empty')
    directory=Path(output_dir).absolute()
    if directory.is_symlink() or directory.exists() and not directory.is_dir():
        raise RuntimeError('invalid remaining-context output directory')
    paths=(directory/'remaining-context-manifest.json',directory/'lib-remaining-context.jsonl.gz',directory/'lib-remaining-context.jsonl')
    historical=evidence_paths()[:2]
    if any(path.resolve() == old.resolve() for path in paths for old in historical):
        raise RuntimeError('fresh --output-dir overlaps historical evidence')
    for path in paths:
        if path.is_symlink() or path.exists() and not path.is_file():
            raise RuntimeError('remaining-context evidence is not a regular owned file: '+str(path))
    return paths


def producer_fixture(raw, destination, historical):
    if len(raw)>LIMIT:
        raise RuntimeError('remaining-context native fixture exceeds bound')
    if historical:
        if destination.is_symlink() or not destination.is_file():
            raise RuntimeError('historical remaining-context evidence unavailable; use --output-dir')
        old=destination.read_bytes()
        try:
            import gzip,io
            with gzip.GzipFile(fileobj=io.BytesIO(old)) as stream:
                previous=stream.read(LIMIT+1)
        except (OSError,EOFError):
            raise RuntimeError('historical remaining-context evidence invalid; use --output-dir')
        if previous!=raw:
            raise RuntimeError('historical producer filenames/observations differ; use --output-dir')
        return old
    return reference.compressed_fixture(raw,destination,LIMIT)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--output-dir',help='fresh verbatim producer manifest, gzip and raw JSONL; historical evidence is preserved')
    parser.add_argument('--reference-root', type=Path, default=ROOT / '.toolchain/oracles/rustc-span-reference')
    args = parser.parse_args()
    path,destination,raw_artifact=evidence_paths(args.output_dir)
    rows, profiles, locations = [], [], {}
    source = HERE / 'native_remaining_context.rs'
    paths = sorted(set(span_wrapper_evidence.inputs() + [source,HERE/'generate_remaining_context.py',HERE/'native_source_helpers.rs',HERE/'test_remaining_context.py']))
    bindings={str(p.relative_to(ROOT)):reference.sha(p.read_bytes())for p in paths}
    selected_tools=reference.tool_inputs()
    for debug in [False, True]:
        for overflow in [False, True]:
            name = f'd{int(debug)}o{int(overflow)}'
            profile, environment = reference.load_profile(args.reference_root / f'build-profile-{name}.json')
            if profile['debug_assertions'] != debug or profile['overflow_checks'] != overflow:
                raise RuntimeError('remaining-context reference profile differs')
            with tempfile.TemporaryDirectory(prefix='ferrugo-lib-remaining-') as directory:
                binary = Path(directory) / 'native'
                command = [profile['rustc'], str(source.relative_to(ROOT)), *profile['link_arguments'], '-o', str(binary)]
                built = subprocess.run(command, cwd=ROOT, env=environment, capture_output=True, timeout=120)
                if built.returncode:
                    raise RuntimeError('remaining-context compile failed:\n' + built.stderr.decode(errors='replace')[-6000:])
                cases = [(op,mode,False,False,'normal')for op in SURFACE for mode in MODES]
                cases += [(op,mode,call,drop,strategy)for op in OWNED for mode in MODES for call in [False,True]for drop in [False,True]for strategy in ['normal','late','early']]
                for op, mode, call, drop, strategy in cases:
                    child = subprocess.run([str(binary), op, mode, str(call).lower(), str(drop).lower(), strategy], cwd=ROOT, env=environment,
                                           capture_output=True, timeout=15, preexec_fn=no_core)
                    abort = expected_abort(op, mode, call, drop, strategy)
                    expected = -signal.SIGABRT if abort else 0
                    if child.returncode != expected:
                        raise RuntimeError(f'unexpected native status {name}/{op}/{mode}/{call}/{drop}: {child.returncode}, expected {expected}\n' + child.stderr.decode(errors='replace')[-4000:])
                    if len(child.stdout) + len(child.stderr) > 64 * 1024:
                        raise RuntimeError('remaining-context output bound exceeded')
                    events = [line.removeprefix('EVENT:')for line in child.stderr.decode().splitlines()if line.startswith('EVENT:')]
                    if not events or events[0] != 'prepared_input:false:1':
                        raise RuntimeError('missing source input/constructor framing')
                    if (b'DONE\n' in child.stdout) == abort or any(line.startswith('after:')for line in events) == abort:
                        raise RuntimeError('native termination gained/lost final observations')
                    if not abort and child.stdout != b'DONE\n':
                        raise RuntimeError('unexpected native completion output')
                    for line in events:
                        if not line.startswith('hook:'):
                            continue
                        fields = line.split(':')
                        text = bytes.fromhex(fields[2]).decode()
                        location = [bytes.fromhex(fields[3]).decode(),int(fields[4]),int(fields[5])]
                        key=op+'|'+text
                        if key in locations and locations[key] != location:
                            raise RuntimeError('actual source location changed for payload:' + text)
                        locations[key] = location
                    rows.append({'request':{'op':op,'mode':mode,'call':call,'drop':drop,'strategy':strategy,'debug':debug,'overflow':overflow},
                                 'want':{'aborted':abort,'events':events}})
            profiles.append({'name':name,'provenance_sha256':profile['provenance_sha256'],'rustc_version':profile['rustc_version']})
    raw = b''.join((json.dumps(row,separators=(',',':'))+'\n').encode()for row in rows)
    compressed = producer_fixture(raw,destination,args.output_dir is None)
    final_paths = sorted(set(span_wrapper_evidence.inputs() + [source,HERE/'generate_remaining_context.py',HERE/'native_source_helpers.rs',HERE/'test_remaining_context.py']))
    if paths!=final_paths or bindings!={str(p.relative_to(ROOT)):reference.sha(p.read_bytes())for p in final_paths} or selected_tools!=reference.tool_inputs():
        raise RuntimeError('remaining-context source/replay/tool inputs changed during native execution')
    manifest = {'schema':1,'artifact_kind':'span-lib-remaining-producer-v1','source_revision':reference.PIN,'status':'actual-public-context-and-owned-FnOnce-closure-no-extra-file-credit',
                'profiles':profiles,'records':len(rows),'actual_aborts':sum(row['want']['aborted']for row in rows),
                'source_locations':locations,'client_source_spelling':'relative ROOT compiler argument; no observer normalization',
                'raw_sha256':reference.sha(raw),'fixture_sha256':reference.sha(compressed),
                'files':bindings,
                'tool_inputs_sha256':selected_tools,
                'scope':'Actual public methods and complete manual non-Copy FnOnce values, boxed-cell identity, original metavars, real Arc return counts/leakage, unused/call/drop failures, current A/B/nil and normal/inherited/processing contexts, independent assertion/overflow profiles. No copied source body, allocator ABI or arbitrary raw-Go-defer equivalence.'}
    encoded = (json.dumps(manifest,indent=2,sort_keys=True)+'\n').encode()
    if args.check:
        if not destination.exists()or destination.read_bytes()!=compressed or not path.exists()or path.read_bytes()!=encoded or raw_artifact is not None and (not raw_artifact.is_file() or raw_artifact.read_bytes()!=raw):
            raise RuntimeError('remaining-context evidence differs')
    else:
        destination.parent.mkdir(parents=True,exist_ok=True)
        destination.write_bytes(compressed)
        path.parent.mkdir(parents=True,exist_ok=True)
        path.write_bytes(encoded)
        if raw_artifact is not None:raw_artifact.write_bytes(raw)
    print(f"Remaining public contexts: {len(rows)} actual-source records / {manifest['actual_aborts']} actual SIGABRT")


if __name__ == '__main__':
    main()
