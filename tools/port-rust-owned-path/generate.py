#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Run actual SDK1.101 std clients, retaining capacity/ownership/mutation traces.

No Go build is run by this producer. Go replay is a separately scheduled check.
Windows Rust PE executes under authenticated Wine on Linux, not a Windows kernel.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import random
import shutil
import subprocess
import tempfile
import prepare

ROOT=prepare.ROOT
HERE=prepare.HERE

def requests(windows):
    rng=random.Random(0x708c94ef)
    def encoded(s):
        return s.encode('utf-16-le','surrogatepass') if windows else s.encode('utf8','surrogatepass')
    text=['','.','./','..','../','/','//','///','a','a/','a/.','a/..','a//b//','foo.tar.gz','.profile','..x','x.','a/b/../c','é/β','a\x00b']
    if windows:
        text += ['C:','c:','C:/','C:foo','C:foo\\..\\bar','\\foo','\\\\host\\share','\\\\host\\share\\a','\\\\?\\C:\\a/./b','\\\\?\\UNC\\host\\share\\a','\\\\?\\volume\\a','\\\\.\\COM1','\ud800','\udc00','a\udbff','\udfffz','\ud800\udc00']
    paths=[encoded(s) for s in text]
    if windows:
        paths += [n.to_bytes(2,'little') for n in [0x7f,0x80,0x800,0xd800,0xdbff,0xdc00,0xdfff,0xe000,0xffff]]
        paths += [b''.join(rng.randrange(65536).to_bytes(2,'little') for _ in range(rng.randrange(25))) for _ in range(100)]
    else:
        paths += [bytes([n]) for n in range(256)]
        paths += [b'a\xff\xc2\xa9\xf0\x9f', b'\xe0\x80\x80', b'\xed\xa0\x80', b'\xf0\x90\x80', b'abc\xe2\x82']
        paths += [bytes(rng.randrange(256) for _ in range(rng.randrange(25))) for _ in range(100)]
    additions=[encoded(s) for s in ['','x','a','longer_file_name','β','.','..','/abs','a/b','/','x.y','\\root','D:foo','\\\\?\\C:\\x','\udc00' if windows else '界']]
    exts=[encoded(s) for s in ['','rs','x.y','.','/','a/b','\\','é','a'*33]]
    scripts=[]
    for i,path in enumerate(paths):
        for factory,capacity in [('from',0),('cap',[0,1,3,7,8,9,15,16,31,64][i%10])]+([('wide',0)] if windows else []):
            ops=['clone','move','round','intostring','cowowned','clonefrom,0','clonefrom,64','lossy','lossymut,ceb2','cowstr,37','cowclonefrom,64','cowtrans','cowstrtrans','stable']
            for _ in range(4):
                add=rng.choice(additions).hex();ext=rng.choice(exts).hex()
                ops += ['push,'+add,'ext,'+ext,'add,'+rng.choice(exts).hex(),'pop','name,'+rng.choice(additions).hex(),'ospush,'+rng.choice(additions).hex(),'join,'+rng.choice(additions).hex(),'clone','lossy']
            ops += ['reserve,37','exact,85','shrink,3','fit','cowborrow','upper','lower','clear','push,'+encoded('tail').hex(),'exact,18446744073709551615','reserve,18446744073709551615','ext,'+encoded('/bad').hex(),'add,'+encoded('/bad').hex(),'pop','cowowned']
            scripts.append({'factory':factory,'capacity':capacity,'initial':path.hex(),'ops':ops})
    # A String input has UTF-8 bytes on both platforms, and must MOVE its actual
    # spare allocation through the OS/path conversion, not reconstruct text.
    for capacity in [0,1,2,3,7,8,16,63,128]:
        for text in ['','a','abc','β','a/long/path']:
            scripts.append({'factory':'string','capacity':capacity,'initial':text.encode().hex(),'ops':['move','round','intostring','clone','cowowned','cowborrow','lossy','cowstr,20']})
    # Ensure UTF-16 append joins newly paired boundary surrogates. The path
    # version inserts a separator; the OS append must form a supplementary char.
    if windows:
        for lead in [0xd800,0xdbff]:
            for trail in [0xdc00,0xdfff]:
                scripts.append({'factory':'wide','capacity':0,'initial':lead.to_bytes(2,'little').hex(),'ops':['ospush,'+trail.to_bytes(2,'little').hex(),'intostring','lossy','clone','round']})
    for ident,row in enumerate(scripts):
        row['id']=ident
        yield row

def digest(data):return hashlib.sha256(data).hexdigest()
def run_profile(compiler,windows,cache):
    rows=list(requests(windows)); env=os.environ.copy()
    if windows:
        module=prepare.load('owned_path_windows_prepare',ROOT/'tools/port-rust-source-map/windows-reference/prepare.py')
        command,env,profile=module.prepare(False,compiler)
        binary=cache/'native.exe'
    else:
        command=[compiler];binary=cache/'native';profile={'host':'x86_64-unknown-linux-gnu','scope':'actual std on Linux'}
    build=subprocess.run([*command,str(HERE/'native.rs'),'-C','opt-level=0','-C','debuginfo=0','-o',str(binary)],cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True,timeout=60)
    data=''.join(f"{r['id']}\t{r['factory']}\t{r['capacity']}\t{r['initial']}\t{';'.join(r['ops'])}\n" for r in rows).encode()
    argv=[env['WINELOADER'],str(binary)] if windows else [str(binary)]
    result=subprocess.run(argv,input=data,cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=False,timeout=60)
    if result.returncode:raise RuntimeError(f'native client failed {result.returncode}: {result.stderr.decode(errors="replace")} stdout tail={result.stdout[-1000:]!r}')
    observations=result.stdout.decode().splitlines()
    if len(observations)!=len(rows):raise RuntimeError('native observations missing')
    for row,observed in zip(rows,observations):
        ident,want=observed.split('\t',1)
        if int(ident)!=row['id']:raise RuntimeError('native observation ID differs')
        row['windows']=windows;row['want']=want
    return rows,profile,digest(build.stderr),digest(result.stderr)

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
    compiler,version,source=prepare.verified()
    if shutil.disk_usage(ROOT).free<3<<30:raise RuntimeError('native witness requires at least 3GiB free')
    allrows=[];profiles=[]
    with tempfile.TemporaryDirectory(prefix='ferrugo-owned-path-') as directory:
        for windows in [False,True]:
            rows,profile,build_stderr,run_stderr=run_profile(compiler,windows,Path(directory))
            allrows.extend(rows);profiles.append({'windows':windows,'profile':profile,'build_stderr_sha256':build_stderr,'run_stderr_sha256':run_stderr})
    data=''.join(json.dumps(row,sort_keys=True,separators=(',',':'))+'\n' for row in allrows).encode()
    compressed=gzip.compress(data,mtime=0)
    manifest={'schema':1,'scope':source['scope'],'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','compiler':version,'sources_sha256':digest((HERE/'SOURCES.json').read_bytes()),'native_wrapper_sha256':digest((HERE/'native.rs').read_bytes()),'records':len(allrows),'operations':sum(len(r['ops']) for r in allrows),'raw_sha256':digest(data),'gzip_sha256':digest(compressed),'profiles':profiles}
    outputs={HERE/'native.jsonl.gz':compressed,HERE/'native-manifest.json':(json.dumps(manifest,indent=2,sort_keys=True)+'\n').encode()}
    for path,content in outputs.items():
        if args.check:
            if path.read_bytes()!=content:raise RuntimeError('native proof differs: '+str(path))
        else:path.write_bytes(content)
    print(manifest['records'],'actual std owning path scripts;',manifest['operations'],'operations; Windows uses Wine on Linux')
if __name__=='__main__':main()
