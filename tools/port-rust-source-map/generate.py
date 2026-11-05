#!/usr/bin/env python3
# SPDX-License-Identifier: MIT OR Apache-2.0
"""Build a client of the complete, unchanged compiler graph and record behavior."""
import argparse,gzip,hashlib,json,os,random,subprocess,sys,tempfile
from pathlib import Path
ROOT=Path(__file__).absolute().parents[2]
HERE=Path(__file__).absolute().parent
sys.path.insert(0,str(ROOT/'tools/port-rust-fatal-error'))
import reference
DEST=ROOT/'port/rust/compiler/rustc_span/src/testdata/source_map_algorithms.tsv.gz'
def requests():
    ops=['snippet','prev','next','accessible','multiline','imported','diag','short','indent','margin','line_empty','lines','start','end','point','follow','extend_prev','extend_prev_nl','extend_before','extend_before_nl','extend_next','extend_next_nl','extend_line','until','through','head','while','prev_while','take','until_white','until_nonwhite','wrapped','prev_str','prev_str_ws','prev_str_nl','prev_str_nl_ws','metadata']
    texts=['','a','a\n','a…b','first line.\nsecond line','\ufeffa\r\nb\r\n','  let mut x = (a);\n\tb\n','α\u2003β\u0085γ\u3000','<test>','(test)','abc   ','abc fn  xyz','\x00\t\n\r\u2028🦀\n']
    rng=random.Random(0x2d20)
    atoms=['a','β','…','🦀','\n','\r',' ','\t','\u00a0','{','}','(',')','<','>',';','f','n','\x00']
    texts += [''.join(rng.choice(atoms) for _ in range(rng.randrange(0,48))) for _ in range(100)]
    id=0
    for text in texts:
        n=len(text.encode())
        points=sorted(set([0,1,2,max(0,n-1),n,n+1,n+3]))
        pairs=[(a,b) for a in points for b in points if a<=b]
        pairs+= [(rng.randrange(n+4),rng.randrange(n+4)) for _ in range(4)]
        for a,b in pairs:
            for op in ops:
                arg='fn' if op.startswith('prev_str') else ('yz' if op=='follow' else ('\n' if id%3==0 else ';' if id%3==1 else 'β'))
                yield f'{id}\t{text.encode().hex()}\t{a}\t{b}\t{op}\t{arg.encode().hex()}\n'
                id+=1

def main():
    p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--all-profiles',action='store_true');p.add_argument('--rustc');args=p.parse_args()
    # The retained fixture always covers all independent source build flags.
    # Keep --all-profiles as a compatible explicit spelling of this default.
    profiles=[(d,o) for d in (False,True) for o in (False,True)]
    data=''.join(requests()).encode(); output=bytearray()
    with tempfile.TemporaryDirectory(prefix='ferrugo-source-map-') as tmp:
        for debug,overflow in profiles:
            key=f'd{int(debug)}o{int(overflow)}';graph,env,_=reference.load(debug,overflow,args.rustc);binary=Path(tmp)/key
            subprocess.run([graph['rustc'],*graph['link_arguments'],str(HERE/'oracle.rs'),'-o',str(binary)],cwd=ROOT,env=env,check=True,timeout=60)
            run=subprocess.run([str(binary)],input=data,stdout=subprocess.PIPE,cwd=ROOT,env=env,check=True,timeout=60)
            expected=run.stdout.splitlines(); actual=data.splitlines()
            if len(expected)!=len(actual):raise RuntimeError('record count differs')
            for request,result in zip(actual,expected):
                ri,status,value=result.split(b'\t'); qi,*fields=request.split(b'\t')
                if ri!=qi:raise RuntimeError('record order differs')
                output.extend(key.encode()+b'\t'+request+b'\t'+status+b'\t'+value+b'\n')
            print(key,len(actual),'native records',flush=True)
    raw=bytes(output)
    if args.check:
        if gzip.decompress(DEST.read_bytes())!=raw:raise SystemExit('native source-map fixture drift')
    else:
        DEST.parent.mkdir(parents=True,exist_ok=True)
        if not DEST.exists() or gzip.decompress(DEST.read_bytes())!=raw:DEST.write_bytes(gzip.compress(raw,mtime=0))
    manifest={'schema':1,'source_revision':'2d20b42b883dcfc014f3c2c56081eeb6091c2179','profiles':[f'd{int(d)}o{int(o)}' for d,o in profiles],'records':len(raw.splitlines()),'fixture_raw_sha256':hashlib.sha256(raw).hexdigest(),'fixture_gzip_sha256':hashlib.sha256(DEST.read_bytes()).hexdigest(),'source':{name:hashlib.sha256(reference.source_bytes('compiler/rustc_span/src/'+name)).hexdigest() for name in ['source_map.rs','source_map/tests.rs','analyze_source_file.rs','analyze_source_file/tests.rs','fatal_error.rs']},'wrapper_sha256':hashlib.sha256((HERE/'oracle.rs').read_bytes()).hexdigest(),'complete':False,'coverage':'Initial public algorithms only; full ownership/tracing/path/formatter/native branch proof remains WIP.'}
    mp=DEST.with_name('source_map_algorithms_manifest.json')
    rendered=json.dumps(manifest,indent=2)+'\n'
    if args.check:
        if mp.read_text()!=rendered:raise SystemExit('source-map manifest drift')
    else:mp.write_text(rendered)
if __name__=='__main__':main()
