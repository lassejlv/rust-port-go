#!/usr/bin/env python3
"""Compare the exact pinned fingerprint files, callbacks, formatting and codecs."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import random
import shutil
import subprocess
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
FORMATS = [
    ("{}",{}),("{:20}",{"Width":20}),("{:>20}",{"Width":20,"Alignment":2}),
    ("{:*^20}",{"Width":20,"Alignment":3,"Fill":42}),("{:+#020.2}",{"Plus":True,"Alternate":True,"ZeroPad":True,"Width":20}),
    ("{:.0}",{}),("{:?}",{}),("{:#?}",{"Alternate":True}),
    ("{:>24?}",{"Width":24,"Alignment":2}),("{:<24?}",{"Width":24,"Alignment":1}),
    ("{:^24?}",{"Width":24,"Alignment":3}),("{:+?}",{"Plus":True}),
    ("{:024?}",{"Width":24,"ZeroPad":True}),("{:+024?}",{"Width":24,"Plus":True,"ZeroPad":True}),
    ("{:x?}",{"DebugRadix":1}),("{:#x?}",{"DebugRadix":1,"Alternate":True}),
    ("{:X?}",{"DebugRadix":2}),("{:#X?}",{"DebugRadix":2,"Alternate":True}),
    ("{:*^24x?}",{"Width":24,"Alignment":3,"Fill":42,"DebugRadix":1}),
    ("{:.3?}",{}),("{:+#024x?}",{"Plus":True,"Alternate":True,"Width":24,"ZeroPad":True,"DebugRadix":1}),
    ("{:🦀^24?}",{"Width":24,"Alignment":3,"Fill":129408}),
    ("{:\0^24?}",{"Width":24,"Alignment":3,"Fill":0,"FillSet":True}),
    ("{:-#24?}",{"Width":24,"Alternate":True}),
    ("{:<024?}",{"Width":24,"Alignment":1,"ZeroPad":True}),
]

def sha(data): return hashlib.sha256(data).hexdigest()
def executable(value,root):
    if "/" in value:
        path=Path(value);result=str(path if path.is_absolute() else root/path)
    else:result=shutil.which(value)
    if not result or not os.access(result,os.X_OK): raise SystemExit("unavailable executable: "+value)
    return result
def run(command,**kwargs):
    result=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120,**kwargs)
    if result.returncode:raise SystemExit(f"command failed: {command}\n{result.stderr.decode(errors='replace')}")
    return result.stdout
def main():
    parser=argparse.ArgumentParser();parser.add_argument('--root',type=Path,default=ROOT);parser.add_argument('--offline',action='store_true');parser.add_argument('--check',action='store_true');parser.add_argument('--fetch',action='store_true');args=parser.parse_args();root=args.root.resolve()
    env=os.environ.copy();rustc=executable(env.get('FERRUGO_RUSTC','.toolchain/rust/bin/rustc'),root);cargo=executable(env.get('FERRUGO_CARGO','.toolchain/rust/bin/cargo'),root);go=executable(env.get('FERRUGO_GO','.toolchain/go/bin/go'),root)
    env.update(FERRUGO_RUSTC=rustc,FERRUGO_CARGO=cargo,FERRUGO_GO=go,RUSTC_BOOTSTRAP='1',GIT_NO_LAZY_FETCH='1');env.setdefault('GOCACHE','/tmp/ferrugo-go-cache')
    cache=root/'.toolchain/oracles/fingerprint-reference'
    wrapper=(HERE/'oracle.rs').read_text();arms='\n'.join(f'{i}=>fmt::write(sink,format_args!({json.dumps(pattern,ensure_ascii=False).replace(chr(92)+"u0000",chr(92)+"0")},value)),' for i,(pattern,_) in enumerate(FORMATS));wrapper=wrapper.replace('// FORMAT_ARMS',arms)
    for debug in [False,True]:
        run([sys.executable,str(HERE/'reference-build/prepare.py'),'--root',str(root),*(['--offline'] if args.offline else []),*(['--fetch'] if args.fetch else []),*([] if debug else ['--release-assertions'])],env=env)
        profile=json.loads((cache/'build-profile.json').read_text());provenance=json.loads((cache/'source-provenance.json').read_text())
        (cache/'oracle.rs').write_text(wrapper)
        run([rustc,'--edition=2024','-C','opt-level=1','-C','debug-assertions='+str(debug).lower(),str(cache/'oracle.rs'),*profile['link_arguments'],'-o',str(cache/('oracle-debug' if debug else 'oracle-release'))],env=env)
    edges=[0,1,2,9,15,16,255,256,(1<<31)-1,1<<31,(1<<32)-1,1<<32,(1<<63)-1,1<<63,(1<<64)-2,(1<<64)-1]
    requests=[];rng=random.Random(0xF17E2026)
    for a in edges:
        for b in edges:requests.append(f'V\t{a}\t{b}\t{b}\t{a}')
    for _ in range(8192):requests.append('V\t'+'\t'.join(str(rng.getrandbits(64)) for _ in range(4)))
    for a,b in [(0,0),(1,2),(15,16),(255,256),(1<<63,(1<<64)-1),((1<<64)-1,0),(0,(1<<64)-1),((1<<64)-1,(1<<64)-1)]:
        for pattern in range(len(FORMATS)):
            for packed in ['false','true']:requests.append(f'F\t{a}\t{b}\t{pattern}\t{packed}')
    for read in [0,1,2]:
        for drop in [0,1,2,3]:
            if drop and (read!=2 or drop==3):continue
            requests.append(f'O\t{read}\t{drop}')
            requests.append(f'I\t{read}\t{drop}\ttrue')
    requests.append('I\t2\t0\tfalse')
    requests.append('Q\t2\t0')
    for length in [0,1,7,8,15,16,17,31,32,33]:
        data=bytes(range(length)).hex()
        for position in sorted(set([0,min(1,length),max(0,length-16),length,length+1])):
            for footer in ['false','true']:
                for packed in ['false','true']:requests.append(f'D\t{data}\t{position}\t{footer}\t-\t{packed}')
    for length in [0,1,15,16,17,32]:
        for packed in ['false','true']:requests.append(f'D\t000102030405060708090a0b0c0d0e0f\t0\ttrue\t{bytes(range(length)).hex()}\t{packed}')
    for values in ['0','1','0,1','1,0','1,2','18446744073709551615,5']:
        requests.append('U\t'+values)
    for a in [0,1,(1<<64)-1]:
        for b in [0,1,(1<<64)-1]:requests.append(f'W\t{a}\t{b}')
    requests.append('B')
    proof=[]
    for _ in range(96):proof.append(f'H\t{rng.getrandbits(64)}\t{rng.getrandbits(64)}\t{rng.randbytes(rng.randrange(40)).hex()}')
    witness=run([go,'run','./tools/port-rust-fingerprint/witness'],cwd=root,env=env,input=('\n'.join(proof)+'\n').encode())
    requests.extend(witness.decode().splitlines())
    request=('\n'.join(requests)+'\n').encode();parts=[]
    for debug in [False,True]:
        binary=cache/('oracle-debug' if debug else 'oracle-release')
        raw=run([str(binary)],input=request,env=env)
        parts.append(('P\t'+str(debug).lower()+'\n').encode()+raw)
    raw=b''.join(parts)
    aborts=[]
    for read,drop in [(0,1),(0,2),(0,3),(1,1),(1,2),(1,3),(2,3)]:
        abort_env=env.copy();abort_env['FERRUGO_FINGERPRINT_ABORT']=f'{read},{drop}'
        result=subprocess.run([str(cache/'oracle-debug')],env=abort_env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
        if result.returncode==0:raise SystemExit('original double panic returned normally')
        aborts.append({'read':read,'drop':drop,'trace':result.stdout.decode().splitlines(),'abort':True})
    for catch_inner in [False,True]:
        for read in [0,1,2]:
            for drop in [0,1,2,3]:
                if catch_inner:
                    if not(drop and (read!=2 or drop==3)):continue
                elif read==2 and drop==0:continue
                abort_env=env.copy();abort_env['FERRUGO_FINGERPRINT_ABORT']=f'{read},{drop},{str(catch_inner).lower()}'
                result=subprocess.run([str(cache/'oracle-debug')],env=abort_env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
                if result.returncode==0:raise SystemExit('original inherited destructor panic returned normally')
                aborts.append({'read':read,'drop':drop,'inherited':True,'catch_inner':catch_inner,'trace':result.stdout.decode().splitlines(),'abort':True})
    for read in [0,1,2]:
        for drop in [0,1,2,3]:
            if read==2 and drop==0:continue
            abort_env=env.copy();abort_env['FERRUGO_FINGERPRINT_ABORT']=f'{read},{drop},processing'
            result=subprocess.run([str(cache/'oracle-debug')],env=abort_env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
            if result.returncode==0:raise SystemExit('original panic-hook nested panic returned normally')
            aborts.append({'read':read,'drop':drop,'processing':True,'trace':result.stdout.decode().splitlines(),'abort':True})
    fixture=root/'port/rust/compiler/rustc_data_structures/src/testdata/fingerprint_oracle.tsv.gz';manifest=fixture.parent.parent/'fingerprint_oracle_manifest.json'
    compressed=None
    if fixture.exists():
        current=fixture.read_bytes()
        try:
            if gzip.decompress(current)==raw:compressed=current
        except (OSError,EOFError):pass
    if args.check and compressed is None:raise SystemExit('fingerprint oracle differs, missing or corrupt')
    if compressed is None:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
    helper_files=[HERE/'oracle.rs',HERE/'generate.py',HERE/'witness/main.go',*sorted((HERE/'reference-build').glob('*'))]
    metadata={'rust_revision':profile['rust_revision'],'compiler':profile['compiler'],'pinned_source_version':profile['pinned_source_version'],'source_files':provenance['sources'],'stable_hash_fragments':provenance['stable_hash_fragments'],'registry_dependencies':profile['registry_dependencies'],'development_only':True,'rustc_bootstrap':'1','rustflags':profile['rustflags'],'scope':'Complete fingerprint.rs and its original test; exact scoped Unhasher block only in Go; real codecs/hash algorithms for reference only; source version1.101 built by development compiler1.99 without body changes; no whole-compiler ABI claim.','source_test_passed':'combine_commutative_is_order_independent','formats':[{'Pattern':p,'Options':o,'Debug':p.endswith('?}')} for p,o in FORMATS],'requests':len(requests),'records':raw.count(b'\n'),'request_sha256':sha(request),'raw_sha256':sha(raw),'fixture_sha256':sha(compressed),'hash_proof':'96 stateful hash requests replay actual Go Hash/StableHash typed callbacks into real Rust DefaultHasher/StableSipHasher and compare unchanged source operations.','owned_aborts':aborts,'helpers':[{'path':str(p.relative_to(root)),'sha256':sha(p.read_bytes())} for p in helper_files if p.is_file()],'helper_dependencies':profile['helper_dependencies']}
    go_root=root/'port/rust/compiler/rustc_data_structures/src'
    metadata['source_test_go_target']='port/rust/compiler/rustc_data_structures/src/fingerprint/tests_test.go'
    metadata['go_owned_component_sha256']=sha((go_root/'fingerprint.go').read_bytes())
    metadata['go_tests']=[{'path':str((go_root/name).relative_to(root)),'sha256':sha((go_root/name).read_bytes())} for name in ['fingerprint/tests_test.go','fingerprint_test.go','fingerprint_fallback_test.go']]
    metadata['raw_go_fallback']={'isolated_cases':8,'source_timing':'Translated Rust panic sites must use Context.Panic.','boundary':'An escaping raw processing-mode Go panic aborts before component argument cleanup; normal-path Close has the same boundary guard.','host_timing':'Raw callback-local Go defers remain observable and cannot be undone; callback-local recovered panics do not reach the guard.'}
    encoded=(json.dumps(metadata,indent=2,ensure_ascii=False)+'\n').encode()
    if args.check:
        if manifest.read_bytes()!=encoded:raise SystemExit('fingerprint manifest differs')
    else:
        fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
    print(f"fingerprint: {len(requests)} requests, {raw.count(bytes([10]))} records; original source test, stateful Go operation replay, and {len(aborts)} owned/inherited abort cases passed")

if __name__=='__main__':main()
