#!/usr/bin/env python3
"""Generate ownership expectations exclusively from the unchanged Rust source."""
import argparse
from collections import Counter
import gzip
import hashlib
import itertools
import json
import os
from pathlib import Path
import random
import resource
import shutil
import signal
import subprocess

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
PIN="2d20b42b883dcfc014f3c2c56081eeb6091c2179"
SOURCE="compiler/rustc_data_structures/src/binary_search_util/mod.rs"
BLOB="1c6e227cec40870c0288f60270f609aeb4ebdc8e"
SHA="6ed8ff0a143e0e4badf23ccd92e483fb14597584c0665cee582882597f086fc9"
FIELDS=("extract","lt","ne","le","keyDrop","fnDrop")
MAX_CASES=4000

def sha(data):return hashlib.sha256(data).hexdigest()
def executable(name,root):
    if "/" in name:
        path=Path(name);value=path if path.is_absolute() else root/path
    else:value=shutil.which(name)
    if not value or not os.access(value,os.X_OK):raise SystemExit("unavailable executable: "+name)
    return os.path.abspath(value)
def command(args,**kwargs):
    result=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120,**kwargs)
    if result.returncode:raise SystemExit(f"command failed: {args}\n{result.stderr.decode(errors='replace')}")
    return result.stdout
def line(request):
    fail=request["fail"]
    values=",".join(map(str,request["values"]))
    return "\t".join([request["id"],str(2 if request["embedded"] else int(request["zst"])),request["length"],str(request["query"]),str(request["zstKey"]),str(request["context"]),*[str(fail.get(field,0)) for field in FIELDS],values])
def new_case(name,values,query,zst=False,length=0):
    return {"id":name,"zst":zst,"embedded":False,"length":str(length),"zstKey":0,"values":values,"query":query,"context":0,"fail":{}}
def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument("--root",type=Path,default=ROOT);parser.add_argument("--check",action="store_true");args=parser.parse_args();root=args.root.resolve()
    environment=os.environ.copy();environment["GIT_NO_LAZY_FETCH"]="1"
    compiler=executable(environment.get("FERRUGO_RUSTC",".toolchain/rust/bin/rustc"),root)
    checkout=Path(environment.get("FERRUGO_RUST_SOURCE",".toolchain/upstream-rust"))
    if not checkout.is_absolute():checkout=root/checkout
    def git(*args):return command(["git","-C",str(checkout),*args],env=environment)
    if git("rev-parse","HEAD").decode().strip()!=PIN:raise SystemExit("Rust source revision differs")
    source=git("show",BLOB)
    if hashlib.sha1(f"blob {len(source)}\0".encode()+source).hexdigest()!=BLOB or sha(source)!=SHA:raise SystemExit("original source identity differs")
    cache=root/".toolchain/oracles/binary-search-owned";cache.mkdir(parents=True,exist_ok=True);(cache/"mod.rs").write_bytes(source)
    environment["BINARY_SEARCH_SOURCE"]=str(cache/"mod.rs")
    binary=cache/"oracle";wrapper=HERE/"oracle.rs"
    command([compiler,"--edition=2024","-C","opt-level=1",str(wrapper),"-o",str(binary)],env=environment)
    version=command([compiler,"-vV"],env=environment).decode().strip()
    cfg=command([compiler,"--print","cfg"],env=environment).decode().splitlines()
    bits=int(next(item.split('"')[1] for item in cfg if item.startswith("target_pointer_width=")))
    maximum=(1<<bits)-1
    baseline=[]
    for size in range(6):
        for values in itertools.combinations_with_replacement([-1,0,1],size):
            for query in [-2,-1,0,1,2]:baseline.append(new_case(f"sorted-{len(baseline)}",list(values),query))
    rng=random.Random(0xB1A0_0ED)
    for index in range(160):
        values=sorted(rng.randrange(-16,17) for _ in range(rng.randrange(65)))
        baseline.append(new_case(f"seeded-{index}",values,rng.randrange(-20,21)))
    for query in [-(1<<63),-1,0,1,(1<<63)-1]:
        baseline.append(new_case(f"extreme-{len(baseline)}",[-(1<<63),-1,0,0,(1<<63)-1],query))
    for length in [0,1,2,63,64,65,1<<(bits-1),maximum]:
        for query in [-1,0,1]:baseline.append(new_case(f"zst-{length}-{query}",[],query,True,length))
    for length in [0,1,4]:
        for query in [-1,0,1]:
            request=new_case(f"embedded-zst-{length}-{query}",[],query,True,length);request["embedded"]=True;baseline.append(request)
    selected=[]
    for values,query in [([],0),([0],0),([0],1),([-2,0,0,0,2],0),([-2,0,0,0,2],-3),([-2,0,0,0,2],3),([0]*9,0),([1,2,3,4,5,6],4)]:
        request=new_case(f"selected-{len(selected)}",values,query);selected.append(request);baseline.append(request)
    for query in [-1,0,1]:
        request=new_case(f"selected-zst-{query}",[],query,True,maximum);selected.append(request);baseline.append(request)
    request=new_case("selected-embedded-zst",[],0,True,4);request["embedded"]=True;selected.append(request);baseline.append(request)
    native=command([str(binary)],input=("\n".join(map(line,baseline))+"\n").encode(),env=environment)
    expected=[json.loads(item) for item in native.splitlines()]
    if len(expected)!=len(baseline):raise SystemExit("native baseline record count differs")
    records=[{"request":request,"expected":result} for request,result in zip(baseline,expected)]
    by_id={item["id"]:item for item in expected}
    pending=[]
    for request in selected:
        state=by_id[request["id"]]["snapshot"]
        limits=[state["extracts"],*state["operators"],state["keyDrops"],1]
        for field,limit in zip(FIELDS,limits):
            indices=list(range(1,limit+1)) if limit<=12 else sorted(set([1,2,limit//2,limit]))
            for at in indices:
                for context in range(4):
                    case=json.loads(json.dumps(request));case.update(id=f"panic-{len(pending)}",context=context,fail={field:at});pending.append(case)
        doubles=[{"extract":1,"fnDrop":1},{"keyDrop":1,"fnDrop":1}]
        if limits[1]:doubles.extend([{"lt":1,"keyDrop":1},{"lt":1,"fnDrop":1}])
        if limits[2]:doubles.extend([{"ne":1,"keyDrop":limits[1]+1},{"ne":1,"fnDrop":1}])
        if limits[3]:doubles.extend([{"le":1,"keyDrop":limits[1]+limits[2]+1},{"le":1,"fnDrop":1}])
        for fail in doubles:
            for context in range(4):
                case=json.loads(json.dumps(request));case.update(id=f"double-{len(pending)}",context=context,fail=fail);pending.append(case)
        for context in [1,2,3]:
            case=json.loads(json.dumps(request));case.update(id=f"context-{len(pending)}",context=context);pending.append(case)
    if len(records)+len(pending)>MAX_CASES:raise SystemExit("owned corpus exceeds case bound")
    # Prevent development-only abort proofs from creating core dumps.
    resource.setrlimit(resource.RLIMIT_CORE,(0,0))
    child_env=environment.copy();child_env["FERRUGO_BINARY_OWNED_STREAM"]="1"
    for index,request in enumerate(pending):
        result=subprocess.run([str(binary)],input=(line(request)+"\n").encode(),env=child_env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
        if len(result.stdout)>2*1024*1024:raise SystemExit("native child output bound exceeded")
        output=result.stdout.decode().splitlines()
        if result.returncode==0:
            if not output or not output[-1].startswith("RESULT\t"):raise SystemExit("native child omitted outcome")
            response=json.loads(output[-1].split("\t",1)[1])
            if output[:-1]!=response["trace"]:raise SystemExit("native streamed events differ")
        elif result.returncode==-signal.SIGABRT:
            response={"id":request["id"],"status":"abort","message":"","view":None,"trace":output,"snapshot":None}
        else:raise SystemExit(f"unexpected native exit {result.returncode} for {request}\n{result.stderr.decode(errors='replace')}")
        records.append({"request":request,"expected":response})
    raw=b"".join((json.dumps(record,separators=(",",":"))+"\n").encode() for record in records)
    fixture=root/"port/rust/compiler/rustc_data_structures/src/binary_search_util/testdata/owned_oracle.jsonl.gz";manifest=fixture.parent.parent/"owned_oracle_manifest.json";compressed=None
    if fixture.exists():
        current=fixture.read_bytes()
        try:
            if gzip.decompress(current)==raw:compressed=current
        except (OSError,EOFError):pass
    if args.check and compressed is None:raise SystemExit("owned native fixture missing, corrupt or different")
    if compressed is None:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
    legacy=[root/"port/rust/compiler/rustc_data_structures/src/binary_search_util"/name for name in ["testdata/oracle.bin.gz","oracle_manifest.json"]]
    metadata={"rust_revision":PIN,"source_path":SOURCE,"source_git_blob":BLOB,"source_sha256":SHA,"compiler":version,"pointer_bits":bits,"original_source_unchanged":True,"native_backend":"Actual std partition_point from compiler profile; no copied search implementation; safe Vec<()> supports usize::MAX without allocation.","cases":len(records),"outcomes":dict(Counter(record["expected"]["status"] for record in records)),"contexts":dict(Counter(record["request"]["context"] for record in records)),"max_cases":MAX_CASES,"max_data_length":128,"max_native_events":2048,"native_child_timeout_seconds":10,"raw_sha256":sha(raw),"fixture_sha256":sha(compressed),"wrapper_sha256":sha(wrapper.read_bytes()),"generator_sha256":sha(Path(__file__).read_bytes()),"legacy_preserved":[{"path":str(path.relative_to(root)),"sha256":sha(path.read_bytes())} for path in legacy],"scope":"Independent original-source ownership/probe-order/borrow/ZST oracle only. Primary translated panic sites use Context.Panic; raw Go callback-local timing is a separate convenience boundary."}
    encoded=(json.dumps(metadata,indent=2)+"\n").encode()
    if args.check:
        if manifest.read_bytes()!=encoded:raise SystemExit("owned native manifest differs")
    else:
        fixture.parent.mkdir(parents=True,exist_ok=True);fixture.write_bytes(compressed);manifest.write_bytes(encoded)
    print(f"owned binary search: {len(records)} native records; {metadata['outcomes']}; {bits}-bit safe ZST MAX validated")

if __name__=="__main__":main()
