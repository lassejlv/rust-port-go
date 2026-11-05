#!/usr/bin/env python3
"""Retain all four arithmetic profiles of the two unchanged edit algorithms."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import random
import shutil
import subprocess
import native_profile as symbol_profile

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
SOURCE_PATH='compiler/rustc_span/src/edit_distance.rs'
SOURCE_SHA='46fd5412ef0bc4d59559bc48a79edd7b11e1045aec881a61ac6386257aa033fe'

def sha(data):return hashlib.sha256(data).hexdigest()
def block(source,marker):
    start=source.index(marker);opening=source.index('{',start);depth=0;i=opening
    while i<len(source):
        if source.startswith('//',i):i=source.index('\n',i)+1;continue
        if source[i]=='{':depth+=1
        elif source[i]=='}':
            depth-=1
            if depth==0:return start,i+1,source[start:i+1]
        i+=1
    raise ValueError('unterminated original function')
def executable(value):
    if '/' in value:
        path=Path(value);path=path if path.is_absolute() else ROOT/path
        result=str(path)
    else:result=shutil.which(value)
    if not result or not os.access(result,os.X_OK):raise SystemExit('unavailable executable: '+value)
    return result
def run(command,**kwargs):
    result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120,**kwargs)
    if result.returncode:raise SystemExit(f'command failed: {command}\n{result.stderr.decode(errors="replace")}')
    return result.stdout
def main():
    parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');parser.add_argument('--offline',action='store_true');args=parser.parse_args()
    rustc,env,compiler=symbol_profile.compiler_profile()
    original=symbol_profile.source_bytes(SOURCE_PATH)
    if sha(original)!=SOURCE_SHA:raise SystemExit('pinned edit_distance source checksum mismatch')
    cache=ROOT/'.toolchain/oracles/symbol-edit-distance';cache.mkdir(parents=True,exist_ok=True)
    fragments=[];bodies=[]
    for marker in ['pub fn edit_distance(', 'pub fn edit_distance_with_substrings(']:
        start,end,body=block(original.decode(),marker);bodies.append(body)
        fragments.append({'name':marker,'first_line':original[:start].count(b'\n')+1,'sha256':sha(body.encode()),'bytes':len(body.encode())})
    (cache/'edit_distance_original.rs').write_text('\n\n'.join(bodies)+'\n')
    (cache/'oracle.rs').write_bytes((HERE/'edit_distance_oracle.rs').read_bytes())
    words=['','a','b','ab','ba','abc','abcd','abcdef','Σ','ΟΣ','σ','ς','𐐀','🦀','\0','\n','e\u0301','ß','ﬃ','İ','ǅ','foo_bar','bar_foo','Märy']
    requests=[]
    for a in words:
        for b in words:
            for limit in [0,1,2,3,4,9,(1<<64)-2,(1<<64)-1]:
                for op in ['D','S']:requests.append(f'{op}\t{a.encode().hex()}\t{b.encode().hex()}\t{limit}')
    rng=random.Random(0x5A4B2026);alphabet='abcde_Σσςéδß🦀\0\n'
    for i in range(2048):
        a=''.join(rng.choice(alphabet) for _ in range(rng.randrange(25)))
        b=''.join(rng.choice(alphabet) for _ in range(rng.randrange(25)))
        limit=(1<<64)-1 if i%17==0 else rng.randrange(17)
        for op in ['D','S']:requests.append(f'{op}\t{a.encode().hex()}\t{b.encode().hex()}\t{limit}')
    request=('\n'.join(requests)+'\n').encode();raw=[];profiles=[]
    for debug in [False,True]:
        for overflow in [False,True]:
            name=f'd{int(debug)}o{int(overflow)}';binary=cache/name
            flags=['--edition=2024','-C','opt-level=1','-C','debug-assertions='+str(debug).lower(),'-C','overflow-checks='+str(overflow).lower()]
            run([rustc,*flags,str(cache/'oracle.rs'),'-o',str(binary)],env=env)
            output=run([str(binary)],input=request,env=env).decode().splitlines()
            if len(output)!=len(requests):raise SystemExit('native record count mismatch')
            raw.extend(f'{name}\t{line}\t{result}\n' for line,result in zip(requests,output))
            profiles.append({'name':name,'debug_assertions':debug,'overflow_checks':overflow,'records':len(output),'panic_records':output.count('panic')})
    raw=''.join(raw).encode();base=ROOT/'port/rust/compiler/rustc_span/src'
    fixture=base/'testdata/symbol_edit_distance.tsv.gz';manifest=base/'symbol_edit_distance_manifest.json'
    compressed=None
    if fixture.exists():
        current=fixture.read_bytes()
        try:
            if gzip.decompress(current)==raw:compressed=current
        except (OSError,EOFError):pass
    if args.check and compressed is None:raise SystemExit('edit-distance fixture differs, missing or corrupt')
    if compressed is None:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
    metadata={'rust_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179',**compiler,'source_path':SOURCE_PATH,'source_blob':'416e9daa8fb090222fa37e6dc6577cf0c27994cf','source_sha256':sha(original),'source_fragments':fragments,'scope':'Two complete unchanged original edit-distance function bodies; Symbol-based matching is validated separately with the whole original span crate.','word_bits':64,'profiles':profiles,'requests':len(requests),'records':raw.count(b'\n'),'request_sha256':sha(request),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'helpers':[{'path':str(p.relative_to(ROOT)),'sha256':sha(p.read_bytes())} for p in [HERE/'generate_edit_distance.py',HERE/'edit_distance_oracle.rs',HERE/'native_profile.py']]}
    encoded=(json.dumps(metadata,indent=2)+'\n').encode()
    if args.check:
        if not manifest.exists() or manifest.read_bytes()!=encoded:raise SystemExit('edit-distance manifest differs')
    else:
        fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
    print(json.dumps({'requests':len(requests),'records':raw.count(b'\n'),'profiles':profiles,'fixture_bytes':len(compressed)}))
if __name__=='__main__':main()
