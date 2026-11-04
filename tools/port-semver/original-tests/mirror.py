#!/usr/bin/env python3
"""Mirror the pinned semver version/requirement test statements without case edits."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
CHECKSUM="8a7852d02fc848982e0c167ef163aaff9cd91dc640ba85e263cb1ce46fae51cd"

def source_files(root):
    cargo_home=Path(os.environ.get("CARGO_HOME",root/".toolchain/port-cargo-home")).resolve()
    archives=list((cargo_home/"registry/cache").glob("*/semver-1.0.28.crate"))
    if len(archives)!=1 or hashlib.sha256(archives[0].read_bytes()).hexdigest()!=CHECKSUM:
        raise SystemExit("missing or mismatched official semver1.0.28 archive")
    with tarfile.open(archives[0],"r:gz") as archive:
        return {member.name.removeprefix("semver-1.0.28/"):archive.extractfile(member).read() for member in archive if member.isfile()}

def decode_string(text,start,quote):
    i=start+1;value=""
    while i<len(text):
        c=text[i];i+=1
        if c==quote:return value,i
        if c!="\\":value+=c;continue
        c=text[i];i+=1
        if c in {'n','r','t','0','\\','"',"'"}:value+={'n':'\n','r':'\r','t':'\t','0':'\0','\\':'\\','"':'"',"'":"'"}[c]
        elif c=='x':value+=chr(int(text[i:i+2],16));i+=2
        elif c=='u':
            end=text.index('}',i);value+=chr(int(text[i+1:end].replace('_',''),16));i=end+1
        else:raise ValueError(f"unexpected source escape {c}")
    raise ValueError("unterminated literal")

def mask(text):
    output=[];literals=[];i=0
    while i<len(text):
        if text.startswith('//',i):
            end=text.find('\n',i);i=len(text) if end<0 else end;continue
        if text.startswith('/*',i):i=text.index('*/',i+2)+2;continue
        if text[i] in ['"',"'"]:
            quote=text[i];value,end=decode_string(text,i,quote);name=f"__LIT_{len(literals)}__";literals.append((quote,value));output.append(name);i=end;continue
        output.append(text[i]);i+=1
    return ''.join(output),literals

def matched(text,start,opening='(',closing=')'):
    depth=0
    for end in range(start,len(text)):
        if text[end]==opening:depth+=1
        elif text[end]==closing:
            depth-=1
            if depth==0:return end
    raise ValueError("unbalanced source")

def split(text):
    parts=[];start=0;depth=0
    for i,c in enumerate(text):
        if c in '([{':depth+=1
        if c in ')]}':depth-=1
        if c==',' and depth==0:parts.append(text[start:i].strip());start=i+1
    if text[start:].strip():parts.append(text[start:].strip())
    return parts

def expression(text):
    text=text.strip()
    if text.startswith('(') and matched(text,0)==len(text)-1:return '('+expression(text[1:-1])+')'
    if text.startswith('!') and not text.startswith('!='):return '!'+expression(text[1:])
    depth=0;i=0
    while i<len(text):
        c=text[i]
        if c in '([{':depth+=1
        if c in ')]}':depth-=1
        if depth==0:
            op=next((op for op in ['==','!=','<=','>=','<','>'] if text.startswith(op,i)),None)
            if op:
                left,right=expression(text[:i]),expression(text[i+len(op):])
                if op=='==':return f"rust_eq({left}, {right})"
                if op=='!=':return f"!rust_eq({left}, {right})"
                return f"rust_cmp({left}, {right}) {op} 0"
        i+=1
    if text.startswith('&'):text=text[1:].lstrip()
    if text.startswith('*'):text=text[1:].lstrip()
    return text

def macros(text):
    pattern=re.compile(r'\b(assert_eq|assert_ne|assert|format)!\s*\(')
    while (match:=pattern.search(text)) is not None:
        opening=match.end()-1;end=matched(text,opening);args=split(text[opening+1:end]);name=match[1]
        args=[macros(arg) for arg in args]
        if name=='format':replacement='rust_format('+', '.join(args)+')'
        elif name=='assert':replacement='assert_true(t, '+', '.join([expression(args[0]),*args[1:]])+')'
        else:replacement=name+'(t, '+', '.join(expression(arg) for arg in args)+')'
        text=text[:match.start()]+replacement+text[end+1:]
    return text

def arrays(text):
    while (start:=text.find('&['))>=0:
        end=matched(text,start+1,'[',']');text=text[:start]+'[]string{'+text[start+2:end]+'}'+text[end+1:]
    match=re.search(r'\blet vs\s*=\s*\[',text)
    if match:
        start=match.end()-1;end=matched(text,start,'[',']');text=text[:start]+'[]string{'+text[start+1:end]+'}'+text[end+1:]
    return text

def leaf(text):
    text=text.replace('&rust_format(','rust_format(')
    for a,b in [('Version::new','semver.NewVersion'),('VersionReq::default()','starReq()'),('VersionReq::STAR','starReq()'),('Prerelease::EMPTY','semver.EmptyPrerelease()'),('BuildMetadata::EMPTY','semver.EmptyBuildMetadata()')]:text=text.replace(a,b)
    text=re.sub(r'\bVersion\s*\{','semver.Version{',text)
    for field in ['major','minor','patch','pre','build']:text=re.sub(r'\b'+field+r'\s*:',field.capitalize()+':',text)
    for function in ['version','version_err','req','req_err','comparator','comparator_err','prerelease','prerelease_err','build_metadata','assert_to_string','assert_match_all','assert_match_none']:
        text=re.sub(r'\b'+function+r'\s*\(',function+'(t, ',text)
    text=text.replace('vs.len()','len(vs)')
    return text

def body(text):
    # The source helper's exact Rust body is retained in the compiler fixture.
    nested=re.search(r'fn calculate_hash\b',text)
    if nested:
        start=text.index('{',nested.start());end=matched(text,start,'{','}');text=text[:nested.start()]+text[end+1:]
    text=macros(text)
    text=arrays(text)
    text=re.sub(r'for (\w+) in ',r'for _, \1 := range ',text)
    text=text.replace('while i < vs.len()','for i < len(vs)')
    seen=set()
    def declaration(match):
        name=match[1];operator='=' if name in seen else ':=';seen.add(name);return name+' '+operator
    text=re.sub(r'\blet(?:\s+ref)?(?:\s+mut)?\s+(\w+)\s*=',declaration,text)
    text=leaf(text)
    return text

def mirror(source,prefix):
    text,literals=mask(source)
    output=['// Code generated from unchanged semver1.0.28 tests by tools/port-semver/original-tests/mirror.py.','package semver_test','import("testing";semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src")']
    functions=[]
    for match in re.finditer(r'#\[test\]\s*(?:pub )?fn (\w+)\(\)\s*\{',text):
        start=match.end()-1;end=matched(text,start,'{','}');functions.append(match[1]);output.append(f'func Test{prefix}_{match[1]}(t *testing.T){{'+body(text[start+1:end])+'}')
    result='\n\n'.join(output)+'\n'
    for i,(quote,value) in enumerate(literals):
        literal=json.dumps(value,ensure_ascii=False) if quote=='"' else "'"+value+"'"
        result=result.replace(f'__LIT_{i}__',literal)
    if prefix=='VersionReq':result=result.replace(';semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"','')
    return result,functions

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--root',type=Path,default=ROOT);parser.add_argument('--check',action='store_true');args=parser.parse_args();root=args.root.resolve();files=source_files(root)
    for path,prefix in [('tests/test_version.rs','Version'),('tests/test_version_req.rs','VersionReq')]:
        result,functions=mirror(files[path].decode(),prefix)
        gofmt=root/'.toolchain/go/bin/gofmt';executable=str(gofmt) if gofmt.exists() else 'gofmt'
        formatted=subprocess.run([executable],input=result.encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        if formatted.returncode:raise SystemExit(formatted.stderr.decode()+result)
        target=root/'port/dependencies/semver/v1_0_28'/path.replace('.rs','_test.go')
        target.parent.mkdir(parents=True,exist_ok=True)
        if args.check:
            if target.read_bytes()!=formatted.stdout:raise SystemExit(f'mirror differs: {target}')
        else:target.write_bytes(formatted.stdout)
        print(path,len(functions),'ordered test functions')
    fixture=HERE/'fixtures'
    for path in ['tests/test_autotrait.rs','tests/node/mod.rs','tests/test_version_req.rs']:
        target=fixture/path;target.parent.mkdir(parents=True,exist_ok=True)
        if args.check:
            if target.read_bytes()!=files[path]:raise SystemExit(f'Rust fixture differs: {target}')
        else:target.write_bytes(files[path])

if __name__=='__main__':main()
