#!/usr/bin/env python3
"""Index the unchanged source and its translated implementations for review.

This is a source inventory, not differential evidence or a completion decision.
The native generators independently verify the original complete source graph.
"""
import argparse
import hashlib
import json
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = pathlib.Path(__file__).resolve().parent
SPAN = ROOT / 'port/rust/compiler/rustc_span/src'
REFERENCE = ROOT / 'tools/port-rust-span-reference'

# Disjoint regions include every source item, including private methods,
# generated newtypes, derives, negative impls and default trait bodies.
REGIONS = [
    (1, 51, 'module documentation and imports', [], []),
    (52, 111, 'SyntaxContext, SyntaxContextKey, SyntaxContextData and ExpnIndex',
     ['hygiene.go', 'hygiene_index.go', 'hygiene_data.go', 'hygiene_tuple.go', 'hygiene_debug.go', 'hygiene_hash_ref.go', 'hygiene_order.go'], ['core', 'format', 'hash', 'order', 'owned']),
    (112, 177, 'ExpnId, ExpnHash and consumed ContextOrd context',
     ['hygiene_types.go', 'hygiene_context_ord.go', 'hygiene_codec.go', 'hygiene_debug.go', 'hygiene_hash_ref.go'], ['core', 'format', 'codec', 'hash', 'owned']),
    (178, 201, 'Transparency variants, fallback and derives',
     ['hygiene_types.go', 'hygiene_drop.go', 'hygiene_codec.go', 'hygiene_debug.go', 'hygiene_hash_ref.go', 'hygiene_order.go'], ['format', 'codec', 'invalid_codec', 'hash', 'order']),
    (202, 276, 'LocalExpnId constructors, accessors, allocation and mutation',
     ['hygiene_index.go', 'hygiene_expn.go'], ['core', 'graph', 'owned']),
    (277, 355, 'ExpnId lookup, ancestry and expansion cause',
     ['hygiene_types.go', 'hygiene_expn.go'], ['core', 'graph', 'owned']),
    (356, 500, 'Remapping results, generic remapper trait, defaults and concrete remappers',
     ['hygiene_remap.go', 'hygiene_remap_ref.go', 'hygiene_context_ord.go'], ['remap', 'owned', 'commit']),
    (501, 784, 'HygieneData storage, construction, closure lock, private lookup and chain algorithms',
     ['hygiene_data.go', 'hygiene_context.go', 'hygiene_inspect.go', 'hygiene_drop.go', 'hygiene_tracing.go'], ['core', 'graph', 'tracing', 'owned']),
    (785, 860, 'walk helpers, dollar-crate resolver and compact/verbose registry Debug',
     ['hygiene_context.go', 'hygiene_expn.go', 'hygiene_inspect.go'], ['core', 'graph', 'format', 'tracing', 'owned']),
    (861, 1096, 'public SyntaxContext resolution, mutation, root fast paths and external-macro checks',
     ['hygiene.go', 'hygiene_context.go', 'hygiene_external.go'], ['core', 'graph', 'owned']),
    (1097, 1117, 'Span::mark_with_reason and consumed StableHashCtxt',
     ['hygiene_span.go'], ['core', 'owned']),
    (1118, 1265, 'ExpnData constructors, private disambiguator, owned Arc and derives',
     ['hygiene_types.go', 'hygiene_drop.go', 'hygiene_expn.go', 'hygiene_codec.go', 'hygiene_debug.go'], ['core', 'format', 'codec', 'owned']),
    (1266, 1424, 'ExpnKind, MacroKind, AstPass and DesugaringKind variants, descriptions and derives',
     ['hygiene_types.go', 'hygiene_drop.go', 'hygiene_codec.go', 'hygiene_debug.go', 'hygiene_hash_ref.go', 'hygiene_order.go'], ['format', 'codec', 'invalid_codec', 'hash', 'order']),
    (1425, 1511, 'HygieneEncodeContext default, deduplicated queues and generic encode callbacks',
     ['hygiene_encode.go', 'hygiene_cell.go', 'hygiene_tracing.go'], ['queue', 'tracing', 'owned']),
    (1512, 1653, 'decode cache, registration, consumed decoder closures and local-ID codec',
     ['hygiene_decode.go', 'hygiene_expn.go', 'hygiene_index.go', 'hygiene_codec.go', 'hygiene_codec_context.go', 'hygiene_tracing.go'], ['decode', 'codec', 'tracing', 'owned']),
    (1654, 1766, 'disambiguator collision handling and custom stable hashing',
     ['hygiene_expn.go', 'hygiene_span.go', 'hygiene_remap.go', 'hygiene_panic.go'], ['core', 'remap', 'tracing', 'owned']),
]

TRAITS = {
    'SyntaxContext': ['Clone', 'Copy', 'Eq', 'PartialEq', 'Hash', 'Debug', 'Default', 'Idx', 'Add<usize>', 'AddAssign<usize>', 'From<u32/usize>', 'Into<u32/usize>', 'StableHash', '!Ord', '!PartialOrd'],
    'ExpnIndex': ['Clone', 'Copy', 'Eq', 'PartialEq', 'Hash', 'Debug', 'Idx', 'Add<usize>', 'AddAssign<usize>', 'From<u32/usize>', 'Into<u32/usize>', 'Ord', 'PartialOrd', 'Step'],
    'LocalExpnId': ['Clone', 'Copy', 'Eq', 'PartialEq', 'Hash', 'Debug', 'Default', 'Idx', 'Add<usize>', 'AddAssign<usize>', 'From<u32/usize>', 'Into<u32/usize>', 'StableHash', 'Encodable', 'Decodable', '!Ord', '!PartialOrd'],
    'DecodedSyntaxContext': ['Clone', 'Copy', 'Eq', 'PartialEq', 'Hash', 'Debug', 'Idx', 'Add<usize>', 'AddAssign<usize>', 'From<u32/usize>', 'Into<u32/usize>', 'Encodable', 'Decodable'],
    'SyntaxContextData': ['Clone', 'Copy', 'Debug'],
    'SyntaxContextKey / (ExpnId, Transparency)': ['Clone', 'Copy', 'PartialEq', 'Eq', 'Hash', 'Debug', 'Encodable', 'Decodable', 'StableHash'],
    'ExpnId': ['Clone', 'Copy', 'PartialEq', 'Eq', 'Hash', 'Debug', 'StableHash'],
    'ExpnHash': ['Clone', 'Copy', 'PartialEq', 'Eq', 'Hash', 'Debug', 'Encodable', 'Decodable', 'StableHash', 'ContextOrd<E>'],
    'Transparency': ['Clone', 'Copy', 'PartialEq', 'Eq', 'PartialOrd', 'Hash', 'Debug', 'Encodable', 'Decodable', 'StableHash'],
    'HygieneRemappingResults<K,O>': ['Default'],
    'HygieneData': ['Debug'],
    'ExpnData': ['Clone', 'Debug', 'Encodable', 'Decodable', 'StableHash', '!PartialEq', '!Hash'],
    'ExpnKind': ['Clone', 'Debug', 'PartialEq', 'Encodable', 'Decodable', 'StableHash'],
    'MacroKind': ['Clone', 'Copy', 'PartialEq', 'Eq', 'PartialOrd', 'Ord', 'Encodable', 'Decodable', 'Hash', 'Debug', 'StableHash'],
    'AstPass': ['Clone', 'Copy', 'Debug', 'PartialEq', 'Encodable', 'Decodable', 'StableHash'],
    'DesugaringKind': ['Clone', 'Copy', 'PartialEq', 'Debug', 'Encodable', 'Decodable', 'StableHash'],
    'HygieneEncodeContext': ['Default'],
    'HygieneDecodeContext': ['Default'],
}

def sha(data):
    return hashlib.sha256(data).hexdigest()

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    graph = json.loads((REFERENCE / 'sources.json').read_text())
    profile = json.loads((ROOT / '.toolchain/oracles/rustc-span-reference/build-profile.json').read_text())
    path = 'compiler/rustc_span/src/hygiene.rs'
    pin = next(f for f in graph['files'] if f['path'] == path)
    data = (pathlib.Path(profile['workspace']) / 'original' / path).read_bytes()
    if sha(data) != pin['sha256'] or len(data) != pin['bytes']:
        raise RuntimeError('original hygiene source bytes differ')
    lines = data.decode().splitlines()
    if len(lines) != 1766 or REGIONS[0][0] != 1 or REGIONS[-1][1] != len(lines):
        raise RuntimeError('source inventory region extent differs')
    for previous, following in zip(REGIONS, REGIONS[1:]):
        if previous[1] + 1 != following[0]:
            raise RuntimeError('source inventory regions overlap or have a gap')
    regions = []
    functions = 0
    for first, last, purpose, files, evidence in REGIONS:
        declarations = []
        for line, text in enumerate(lines[first-1:last], first):
            stripped = text.strip()
            if re.match(r'(?:pub(?:\([^)]*\))?\s+)?(?:const\s+)?fn\s+', stripped):
                declarations.append({'line': line, 'declaration': stripped})
                functions += 1
        paths = [SPAN / name for name in files]
        regions.append({'source_lines': [first, last], 'purpose': purpose, 'source_functions': declarations,
                        'go_files': {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in paths}, 'native_suites': evidence})
    source_macro = 'compiler/rustc_index_macros/src/newtype.rs'
    macro_data = (pathlib.Path(profile['workspace']) / 'original' / source_macro).read_bytes()
    macro_pin = next(f for f in graph['files'] if f['path'] == source_macro)
    if sha(macro_data) != macro_pin['sha256']:
        raise RuntimeError('source newtype macro bytes differ')
    output = {'schema': 1, 'complete': False,
              'purpose': 'Review inventory of every original source region and explicit function plus generated/derived trait surface. Presence and bindings are not a behavioral equivalence proof or acceptance decision.',
              'source_revision': graph['revision'], 'source': {'path': path, 'sha256': sha(data), 'lines': len(lines), 'explicit_functions': functions},
              'newtype_macro': {'path': source_macro, 'sha256': sha(macro_data)},
              'traits': TRAITS, 'regions': regions,
              'inherited_operations': ['Clone::clone_from', 'PartialEq::ne', 'PartialOrd predicates', 'Ord predicates/min/max/clamp where implemented', 'Hash::hash_slice', 'Idx::plus/increment_by', 'Step::forward/backward and unsafe defaults'],
              'representation_contract': 'Source-valid closed enum variants and active fields; source ownership is expressed by moved Owned/Once dictionaries, explicit Close, Arc handles, borrowed guards, and current Context. Invalid unchecked newtype construction is outside the original Rust contract.',
              'surrounding_traits': 'SpanEncoder/SpanDecoder delegate SyntaxContext/ExpnId codecs through the existing lib.go boundary. The surrounding source files and translated collection/sort/format/hash/runtime dependencies have independent ownership and receive no completion credit from this inventory.'}
    encoded = (json.dumps(output, indent=2) + '\n').encode()
    target = HERE / 'source-inventory.json'
    if args.check:
        if target.read_bytes() != encoded:
            raise RuntimeError('hygiene source inventory differs')
    else:
        target.write_bytes(encoded)
    print(f'{len(lines)} unchanged source lines; {functions} explicit source functions; {len(regions)} disjoint review regions')

if __name__ == '__main__':
    main()
