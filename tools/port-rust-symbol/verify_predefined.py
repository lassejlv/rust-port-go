#!/usr/bin/env python3
"""Audit Go's complete generated symbol data against the unchanged Rust macro.

The name-only Rust declarations permit expansion, not type checking or runtime
execution. This proves generated tokens/indices/prefills, not Symbol's runtime.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

import generate_predefined as generated
import native_profile
import symbol_reference

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
TOKEN=re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"|[A-Za-z_][A-Za-z_0-9]*|[0-9]+[A-Za-z_0-9]*|\S',re.S)
def sha(data):return hashlib.sha256(data).hexdigest()
def tokens(source):return [m.group() for m in TOKEN.finditer(source) if not m.group().startswith(('//','/*'))]
def run(command,**kwargs):
    result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180,**kwargs)
    if result.returncode:raise SystemExit(f'command failed: {command}\n{result.stderr.decode(errors="replace")[:4096]}')
    return result.stdout
def executable(value):
    path=Path(value) if '/' in value else None
    result=str(path if path.is_absolute() else ROOT/path) if path is not None else shutil.which(value)
    if not result or not os.access(result,os.X_OK):raise SystemExit('unavailable executable: '+value)
    return result
def macro_input(source):
    # Source braces inside strings/comments are single tokens. The original
    # invocation bytes, including whitespace/comments, remain unchanged.
    start=source.index('symbols! {');opening=source.index('{',start);depth=0
    for match in TOKEN.finditer(source,opening):
        value=match.group()
        if value=='{':depth+=1
        elif value=='}':
            depth-=1
            if depth==0:return source[start:match.end()]
    raise ValueError('unterminated macro invocation')
def native_data(expanded):
    stream=tokens(expanded);named=[];group=None;depth=0;group_depth=0
    for i,token in enumerate(stream):
        if token=='mod' and stream[i+1] in ['kw_generated','sym_generated']:
            group='kw' if stream[i+1]=='kw_generated' else 'sym';group_depth=depth+1
        if token=='{':depth+=1
        elif token=='}':
            depth-=1
            if group is not None and depth<group_depth:group=None
        if group and token=='const' and stream[i+2:i+5]==[':','Symbol','=']:
            tail=stream[i+5:i+12]
            if len(tail)<7 or tail[:5]!=['Symbol',':',':','new','('] or tail[6]!=')' or re.fullmatch(r'[0-9]+u32',tail[5]) is None:
                raise ValueError('unexpected generated Symbol constructor')
            named.append((group,stream[i+1],int(tail[5].removesuffix('u32'))))
    position=expanded.index('Interner::prefill(&[')+len('Interner::prefill(&[')
    values=[]
    for match in TOKEN.finditer(expanded,position):
        value=match.group()
        if value==']':break
        if value.startswith('"'):values.append(generated.rust_string(value))
        elif value!=',':raise ValueError('unexpected generated prefill token: '+value)
    bases=[int(re.search(r'const '+name+r': u32 = ([0-9]+)u32;',expanded).group(1)) for name in ['SYMBOL_DIGITS_BASE','SYMBOL_UPPERCASE_LETTERS_BASE','SYMBOL_LOWERCASE_LETTERS_BASE']]
    count=int(re.search(r'const PREDEFINED_SYMBOLS_COUNT: u32 = ([0-9]+)u32;',expanded).group(1))
    if count!=len(values):raise ValueError('native count differs from prefill length')
    return values,named,bases
def main():
    parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--offline',action='store_true');parser.add_argument('--prepared',action='store_true');parser.add_argument('--macro-profile',type=Path,default=None);args=parser.parse_args()
    profile,env,compiler=symbol_reference.load_requested(args.macro_profile)
    rustc=profile['rustc'];macro=profile['artifacts']['rustc_macros']['filenames'][0]
    original=native_profile.source_bytes('compiler/rustc_span/src/symbol.rs')
    if sha(original)!=generated.SOURCE_SHA256:raise SystemExit('pinned symbol checksum mismatch')
    macro_source=native_profile.source_bytes('compiler/rustc_macros/src/symbols.rs')
    if sha(macro_source)!='270cd2d8dfdd91c6de5eaa2ba0e8ceac6af747aef4af2bf0294fd31a95f5a682':raise SystemExit('pinned macro checksum mismatch')
    cache=ROOT/'.toolchain/oracles/symbol-predefined';cache.mkdir(parents=True,exist_ok=True)
    (cache/'macro.rs').write_text('use rustc_macros::symbols;\nstruct Symbol;\nstruct Interner;\n'+macro_input(original.decode())+'\n')
    release_profiles=[]
    for release in ['1.101.0','rustc','0','🦀','', 'release-with\nbytes']:
        env['CFG_RELEASE']=release
        command=[rustc,'--edition=2024','-Zunpretty=expanded','--crate-type=lib','--extern',f'rustc_macros={macro}','-L',f'dependency={Path(macro).parent}',str(cache/'macro.rs')]
        expanded=run(command,env=env)
        actual=native_data(expanded.decode());expected=generated.definitions(original.decode(),release)
        if actual!=expected:raise SystemExit('native/generated predefined data mismatch for '+repr(release))
        values,named,bases=actual
        release_profiles.append({'cfg_release':release,'predefined':len(values),'named_constants':len(named),'bases':bases,'env_index':next(index for group,name,index in named if name=='env_CFG_RELEASE'),'expanded_sha256':sha(expanded)})
    run([sys.executable,str(HERE/'generate_predefined.py'),'--check'],env=env)
    metadata={'rust_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179',**compiler,'symbol_source_blob':'27b585c9e6fbdb566ce77efa9c7a1e06534dd07b','symbol_source_sha256':sha(original),'macro_source_blob':'bba6488e2ea73a98f0b82b43f998d7d163d16e30','macro_source_sha256':sha(macro_source),'scope':'Complete generated named constants, insertion order, prefill bytes, ASCII bases and tracked CFG_RELEASE alias behavior; actual unchanged Rust proc macro expansion, no runtime/type-checking claim for the name-only expansion wrapper.','reference_inputs':symbol_reference.inputs(),'profiles':release_profiles,'generated_go_sha256':sha(generated.TARGET.read_bytes()),'helpers':[{'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())} for path in [HERE/'generate_predefined.py',HERE/'verify_predefined.py',HERE/'native_profile.py']]}
    path=ROOT/'port/rust/compiler/rustc_span/src/symbol_predefined_manifest.json';encoded=(json.dumps(metadata,ensure_ascii=False,indent=2)+'\n').encode()
    if args.check:
        if not path.exists() or path.read_bytes()!=encoded:raise SystemExit('predefined manifest differs')
    else:path.write_bytes(encoded)
    print(json.dumps({'profiles':len(release_profiles),'named_constants_per_profile':release_profiles[0]['named_constants'],'default_predefined':release_profiles[0]['predefined']}))
if __name__=='__main__':main()
