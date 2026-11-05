# SPDX-License-Identifier: MIT OR Apache-2.0
"""Bounded current-producer artifacts for build-specific panic filenames."""
import gzip
import hashlib
import io
from pathlib import Path

ROOT = Path(__file__).absolute().parents[2]
HERE = Path(__file__).absolute().parent
LIMIT = 2 * 1024 * 1024


def evidence_paths(scope, output_dir=None):
    if scope not in ('session', 'assert'):
        raise RuntimeError('unsupported fresh panic producer scope')
    directory = ROOT / 'port/rust/compiler/rustc_span/src/testdata'
    if output_dir is not None:
        if output_dir == '':
            raise RuntimeError('panic --output-dir explicitly empty')
        directory = Path(output_dir).absolute()
        if directory.is_symlink() or directory.exists() and not directory.is_dir():
            raise RuntimeError('invalid panic output directory')
    stem = 'source_map_panic_' + scope
    paths = (directory / (stem + '_manifest.json'), directory / (stem + '.tsv.gz'),
             directory / (stem + '.tsv') if output_dir is not None else None)
    if output_dir is not None:
        for other in ('session', 'assert'):
            if any(path.resolve() == old.resolve() for path in paths for old in evidence_paths(other)[:2]):
                raise RuntimeError('fresh --output-dir overlaps historical evidence')
        for path in paths:
            if path.is_symlink() or path.exists() and not path.is_file():
                raise RuntimeError('panic evidence is not a regular owned file: ' + str(path))
    return paths


def fixture(raw, destination, historical):
    if len(raw) > LIMIT:
        raise RuntimeError('panic fixture exceeds bound')
    old = None
    if destination.is_symlink() or destination.exists() and not destination.is_file():
        raise RuntimeError('panic evidence is not a regular owned file')
    if destination.is_file():
        if destination.stat().st_size > LIMIT:
            raise RuntimeError('compressed panic evidence exceeds bound')
        old = destination.read_bytes()
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(old)) as stream:
                previous = stream.read(LIMIT + 1)
        except (OSError, EOFError):
            if historical:
                raise RuntimeError('historical panic evidence invalid; use --output-dir')
        else:
            if previous == raw:
                return old
    if historical:
        raise RuntimeError('historical producer filenames/observations differ; use --output-dir')
    return gzip.compress(raw, mtime=0)


def input_bindings():
    paths = [HERE / name for name in ('generate_panic_sites.py', 'panic_site_producer.py',
                                     'test_panic_site_producer.py', 'panic_sites.rs')]
    source = ROOT / 'port/rust/compiler/rustc_span/src'
    paths += [source / name for name in ('source_map_panic_sites_test.go', 'source_map_panic_producer_test.go',
                                        'source_map_panic.go', 'source_map_files.go', 'source_map_locations.go')]
    return {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in sorted(paths)}
