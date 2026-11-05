#!/usr/bin/env python3
"""Exact whole rustc_ast reference; --source-only never invokes Rust or Cargo."""
import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent
SPAN_TOOL = ROOT / 'tools/port-rust-span-reference'
spec = importlib.util.spec_from_file_location('ast_exact_span_source_support', SPAN_TOOL / 'prepare.py')
SUPPORT = importlib.util.module_from_spec(spec)
spec.loader.exec_module(SUPPORT)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def run(arguments, environment, work):
    result = subprocess.run(list(map(str, arguments)), cwd=work, env=environment,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=240)
    if result.returncode:
        raise RuntimeError(str(arguments) + '\n' + result.stderr.decode(errors='replace')[-12000:])
    return result.stdout, result.stderr


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--source-only', action='store_true')
    parser.add_argument('--fetch-source', action='store_true')
    parser.add_argument('--fetch-deps', action='store_true')
    parser.add_argument('--source')
    parser.add_argument('--output')
    parser.add_argument('--debug-assertions', choices=['true', 'false'], default='true')
    parser.add_argument('--overflow-checks', choices=['true', 'false'], default='true')
    args = parser.parse_args()
    graph = json.loads((HERE / 'sources.json').read_text())
    selection = json.loads((SPAN_TOOL / 'toolchain.json').read_text())
    environment = dict(os.environ, GIT_NO_LAZY_FETCH='1')
    source = SUPPORT.path_option(args.source if args.source is not None else environment.get('FERRUGO_RUST_SOURCE', str(ROOT / '.toolchain/upstream-rust')), 'FERRUGO_RUST_SOURCE')
    output = SUPPORT.path_option(args.output if args.output is not None else str(ROOT / '.toolchain/oracles/rustc-ast-util-reference'), 'output')
    if output == source or output.is_relative_to(source) or source.is_relative_to(output):
        raise RuntimeError('AST reference output overlaps original Git cache')
    identity = {'schema': 1, 'tool': 'port-rust-ast-util/reference-build', 'revision': graph['revision']}
    marker = output / 'reference-cache.json'
    if output.exists() and any(output.iterdir()) and not marker.is_file():
        raise RuntimeError('unowned AST reference output directory')
    output.mkdir(parents=True, exist_ok=True)
    if marker.exists() and json.loads(marker.read_text()) != identity:
        raise RuntimeError('AST reference output identity differs')
    marker.write_text(json.dumps(identity, indent=2) + '\n')
    with (output / '.prepare.lock').open('a+b') as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        original = SUPPORT.materialize(graph, source, output, environment, args.fetch_source)
        selected_lock = (HERE / 'Cargo.lock').read_bytes()
        selected = SUPPORT.lock_packages(selected_lock)
        upstream = SUPPORT.lock_packages(original['Cargo.lock'])
        if len(selected) != 109 or any(upstream.get(key) != value for key, value in selected.items()):
            raise RuntimeError('AST registry selection differs from pinned original Cargo.lock')
        (output / 'Cargo.toml').write_text('[workspace]\nresolver="2"\nmembers=' + json.dumps(['original/' + c for c in graph['crates']]) + '\n')
        (output / 'Cargo.lock').write_bytes(selected_lock)
        if args.source_only:
            print('Source-only: 12 unchanged compiler crates/221 files and 109 locked registry selections verified; no Rust/Cargo invoked')
            return
        for name in ['RUSTFLAGS', 'CARGO_ENCODED_RUSTFLAGS', 'RUSTC_WRAPPER', 'RUSTC_WORKSPACE_WRAPPER', 'CARGO_BUILD_TARGET', 'UPDATE_EXPECT']:
            environment.pop(name, None)
        rustc = SUPPORT.executable(None, 'FERRUGO_RUSTC', Path('.toolchain/rust-nightly-2026-11-04/bin/rustc'))
        cargo = SUPPORT.executable(None, 'FERRUGO_CARGO', Path('.toolchain/rust-nightly-2026-11-04/bin/cargo'))
        rustdoc = SUPPORT.executable(None, 'FERRUGO_RUSTDOC', Path(rustc).parent / 'rustdoc')
        for executable, expected in [(rustc, selection['rustc']), (cargo, selection['cargo']), (rustdoc, selection['rustc'].replace('rustc ', 'rustdoc ', 1))]:
            actual, _ = run([executable, '--version'], environment, output)
            if actual.decode().strip() != expected:
                raise RuntimeError('AST reference requires exact matching dated nightly: ' + executable)
        cfg, _ = run([rustc, '--print', 'cfg'], environment, output)
        required_cfg = [b'target_pointer_width="64"', b'target_arch="x86_64"', b'target_os="linux"', b'target_endian="little"']
        if any(value not in cfg.splitlines() for value in required_cfg):
            raise RuntimeError('AST native reference profile requires x86_64-unknown-linux-gnu')
        verbose, _ = run([rustc, '-vV'], environment, output)
        if ('commit-hash: ' + selection['rustc_commit']).encode() not in verbose.splitlines() or ('host: ' + selection['target']).encode() not in verbose.splitlines():
            raise RuntimeError('AST reference compiler commit/host differs from selected exact nightly')
        cargo_home = SUPPORT.path_option(environment.get('CARGO_HOME', str(ROOT / '.toolchain/port-cargo-home')), 'CARGO_HOME')
        name = 'd' + str(int(args.debug_assertions == 'true')) + 'o' + str(int(args.overflow_checks == 'true'))
        environment.update(RUSTC=rustc, RUSTDOC=rustdoc, RUSTC_BOOTSTRAP='1', CFG_RELEASE='1.101.0',
                           CARGO_HOME=str(cargo_home), CARGO_TARGET_DIR=str(output / 'target' / name))
        for profile in ['DEV', 'TEST']:
            for key, value in [('DEBUG_ASSERTIONS', args.debug_assertions), ('OVERFLOW_CHECKS', args.overflow_checks), ('OPT_LEVEL', '0'), ('PANIC', 'unwind')]:
                environment['CARGO_PROFILE_' + profile + '_' + key] = value
        if args.fetch_deps:
            run([cargo, 'fetch', '--locked'], environment, output)
        metadata, _ = run([cargo, 'metadata', '--locked', '--offline', '--format-version=1'], environment, output)
        metadata = json.loads(metadata)
        dependencies = SUPPORT.verify_registry(metadata, selected_lock, cargo_home)
        built, errors = run([cargo, 'build', '--locked', '--offline', '-p', 'rustc_ast', '--message-format=json'], environment, output)
        logs = output / 'logs'; logs.mkdir(exist_ok=True)
        (logs / (name + '-build.jsonl')).write_bytes(built)
        (logs / (name + '-build.stderr')).write_bytes(errors)
        artifacts, directories = {}, set()
        for line in built.splitlines():
            row = json.loads(line)
            if row.get('reason') != 'compiler-artifact':
                continue
            files = [p for p in row.get('filenames', []) if p.endswith(('.rlib', '.rmeta', '.so', '.dylib'))]
            if files:
                directories.update(str(Path(p).parent) for p in files)
                artifacts.setdefault(row['target']['name'], []).append({'filenames': files, 'package_id': row['package_id'], 'kinds': row['target']['kind']})
        arguments = ['--edition=2024', '-C', 'opt-level=0', '-C', 'debug-assertions=' + args.debug_assertions, '-C', 'overflow-checks=' + args.overflow_checks]
        for directory in sorted(directories): arguments += ['-L', 'dependency=' + directory]
        for crate in graph['crates']:
            crate = crate.rsplit('/', 1)[1]
            found = artifacts.get(crate, [])
            if len(found) != 1: raise RuntimeError('missing or ambiguous original AST graph artifact: ' + crate)
            for file in found[0]['filenames']: arguments += ['--extern', crate + '=' + file]
        tests, errors = run([cargo, 'test', '--locked', '--offline', '-p', 'rustc_ast', '--lib'], environment, output)
        (logs / (name + '-tests.stdout')).write_bytes(tests); (logs / (name + '-tests.stderr')).write_bytes(errors)
        if b'6 passed; 0 failed;' not in tests: raise RuntimeError('expected all six original rustc_ast unit tests')
        if dependencies != SUPPORT.verify_registry(metadata, selected_lock, cargo_home): raise RuntimeError('AST expanded registry source changed during compilation')
        for path, body in original.items():
            target = output / ('original' if path.startswith('compiler/') else 'upstream-root') / path
            if target.read_bytes() != body: raise RuntimeError('AST original source bytes changed: ' + path)
        smoke = output / ('smoke-' + name + '.rs')
        smoke.write_text('fn main(){rustc_span::create_default_session_globals_then(||{let value=rustc_ast::util::comments::beautify_doc_string(rustc_span::Symbol::intern("\\n * x\\n"),rustc_ast::token::CommentKind::Block);println!("{:?}:{:?}:{}",rustc_ast::util::case::Case::Sensitive,rustc_ast::token::CommentKind::Line,value.as_str());});}\n')
        run([rustc, *arguments, smoke, '-o', output / ('smoke-' + name)], environment, output)
        observed, _ = run([output / ('smoke-' + name)], environment, output)
        if observed != b'Sensitive:Line: x\n': raise RuntimeError('AST actual external linkage smoke differs')
        proof = {'schema': 1, 'source_revision': graph['revision'], 'source_graph_sha256': sha((HERE / 'sources.json').read_bytes()),
                 'source_files': graph['files'], 'source_crates': graph['crates'], 'registry_dependencies': dependencies,
                 'selected_lock_sha256': sha(selected_lock), 'rustc_version': selection['rustc'], 'cargo_version': selection['cargo'],
                 'target': selection['target'], 'rustc_commit': selection['rustc_commit'],
                 'debug_assertions': args.debug_assertions == 'true', 'overflow_checks': args.overflow_checks == 'true',
                 'cfg_release': '1.101.0', 'cfg_bootstrap': False, 'injected_features': [],
                 'tool_sha256': {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in [Path(__file__), HERE / 'sources.json', HERE / 'Cargo.lock', SPAN_TOOL / 'prepare.py', SPAN_TOOL / 'verify.py', SPAN_TOOL / 'toolchain.json']}}
        provenance = output / ('source-provenance-' + name + '.json'); provenance.write_text(json.dumps(proof, indent=2) + '\n')
        profile = {'schema': 1, 'rustc': rustc, 'cargo': cargo, 'rustc_version': selection['rustc'], 'source_revision': graph['revision'],
                   'target': selection['target'], 'rustc_commit': selection['rustc_commit'],
                   'debug_assertions': proof['debug_assertions'], 'overflow_checks': proof['overflow_checks'],
                   'link_arguments': arguments, 'artifacts': artifacts, 'provenance_path': str(provenance), 'provenance_sha256': sha(provenance.read_bytes()),
                   'original_ast_tests': 6, 'external_smoke': observed.decode().strip(), 'workspace': str(output)}
        (output / ('build-profile-' + name + '.json')).write_text(json.dumps(profile, indent=2) + '\n')
        print('Full original rustc_ast: 12 compiler crates/109 exact registry dependencies/6 original tests and external linkage passed; ' + name)


if __name__ == '__main__':
    try: main()
    except Exception as error: print(error, file=sys.stderr); sys.exit(1)
