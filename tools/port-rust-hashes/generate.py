#!/usr/bin/env python3
"""Compare unchanged compiler hash wrappers, including native formatter writes."""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import random
import shutil
import subprocess
import sys
import zlib

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PIN = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
SOURCE = "compiler/rustc_hashes/src/lib.rs"
BLOB = "c8f3057dac5fc26da8c243f95d8a978ee8cc00b7"
SHA = "ecb7777ffca8e89ad7056cc864771983a3eb4384049d739fbd5b562ed2d53850"
RUSTC = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
FORMATS = [
    ("{:?}",{}), ("{:#?}",{"Alternate":True}),
    ("{:>24?}",{"Width":24,"Alignment":2}), ("{:<24?}",{"Width":24,"Alignment":1}),
    ("{:^24?}",{"Width":24,"Alignment":3}), ("{:+?}",{"Plus":True}),
    ("{:024?}",{"Width":24,"ZeroPad":True}), ("{:+024?}",{"Width":24,"Plus":True,"ZeroPad":True}),
    ("{:x?}",{"DebugRadix":1}), ("{:#x?}",{"DebugRadix":1,"Alternate":True}),
    ("{:X?}",{"DebugRadix":2}), ("{:#X?}",{"DebugRadix":2,"Alternate":True}),
    ("{:*^48x?}",{"Width":48,"Alignment":3,"Fill":42,"DebugRadix":1}),
    ("{:.3?}",{}), ("{:x}",{}), ("{:#x}",{"Alternate":True}),
    ("{:+#048x}",{"Plus":True,"Alternate":True,"Width":48,"ZeroPad":True}),
    ("{:>48x}",{"Width":48,"Alignment":2}),
    ("{:<#48x}",{"Width":48,"Alignment":1,"Alternate":True}),
    ("{:🦀^48x}",{"Width":48,"Alignment":3,"Fill":129408}),
    ("{:\0^24x}",{"Width":24,"Alignment":3,"Fill":0,"FillSet":True}),
    ("{:-#24x}",{"Width":24,"Alternate":True}),
    ("{:<048x}",{"Width":48,"Alignment":1,"ZeroPad":True}),
]

def sha(data): return hashlib.sha256(data).hexdigest()

def executable(value, root):
    if not value: raise SystemExit("explicit executable must not be empty")
    if "/" in value:
        path=Path(value)
        result=str(path if path.is_absolute() else root/path)
    else:
        result=shutil.which(value)
    if not result or not os.access(result,os.X_OK): raise SystemExit(f"unavailable executable: {value}")
    return result

def run(command, **kwargs):
    return subprocess.run(command,check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120,**kwargs).stdout

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root",type=Path,default=ROOT)
    parser.add_argument("--rustc",default=os.environ.get("FERRUGO_RUSTC",".toolchain/rust/bin/rustc"))
    parser.add_argument("--cargo",default=os.environ.get("FERRUGO_CARGO",".toolchain/rust/bin/cargo"))
    parser.add_argument("--fetch",action="store_true")
    parser.add_argument("--check",action="store_true")
    args=parser.parse_args()
    root=args.root.resolve()
    compiler=executable(args.rustc,root)
    cargo=executable(args.cargo,root)
    env=os.environ.copy()
    env.update(RUSTC=compiler,GIT_NO_LAZY_FETCH="1")
    home=Path(env.get("CARGO_HOME",root/".toolchain/port-cargo-home"))
    home=home if home.is_absolute() else root/home
    env["CARGO_HOME"]=str(home)
    version=run([compiler,"--version"],env=env).decode().strip()
    if version!=RUSTC: raise SystemExit(f"expected {RUSTC}; found {version}")
    details=run([compiler,"-vV"],env=env).decode()
    host=next((line.removeprefix("host: ") for line in details.splitlines() if line.startswith("host: ")),"")
    if host!="x86_64-unknown-linux-gnu": raise SystemExit(f"reference regeneration requires x86_64-unknown-linux-gnu, found {host}")
    upstream=root/".toolchain/upstream-rust"
    def git(*tail): return run(["git","-C",str(upstream),*tail],env=env)
    if git("rev-parse","HEAD").decode().strip()!=PIN: raise SystemExit("source pin mismatch")
    if git("rev-parse",f"{PIN}:{SOURCE}").decode().strip()!=BLOB: raise SystemExit("source blob mismatch")
    source=git("show",BLOB)
    if sha(source)!=SHA: raise SystemExit("original source SHA-256 mismatch")
    wrapper=(HERE/"oracle.rs").read_bytes()
    arms="\n".join(f"{n} => fmt::write(out,format_args!({json.dumps(pattern,ensure_ascii=False).replace(chr(92)+'u0000',chr(92)+'0')}, value))," for n,(pattern,_) in enumerate(FORMATS))
    original=wrapper.replace(b"// PINNED_HASHES",source).replace(b"// FORMAT_ARMS",arms.encode())
    if original.count(source)!=1: raise SystemExit("complete original file must occur once verbatim")
    cache=root/".toolchain/oracles/hashes-reference"
    cache.mkdir(parents=True,exist_ok=True)
    for name,data in {"oracle.rs":original,"Cargo.toml":(HERE/"Cargo.toml").read_bytes(),"Cargo.lock":(HERE/"Cargo.lock").read_bytes()}.items():
        path=cache/name
        if not path.exists() or path.read_bytes()!=data: path.write_bytes(data)
    tail=["--locked","--manifest-path",str(cache/"Cargo.toml")]
    if args.fetch: run([cargo,"fetch",*tail],env=env)
    # Reuse the audited archive/extraction verifier; no original algorithm is
    # supplied by this development-only helper.
    spec=importlib.util.spec_from_file_location("edition_prepare",root/"tools/port-rust-edition/reference-build/prepare.py")
    helper=importlib.util.module_from_spec(spec); spec.loader.exec_module(helper)
    archives=list((home/"registry/cache").glob("*/rustc-stable-hash-0.1.2.crate"))
    expected="781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08"
    if len(archives)!=1 or sha(archives[0].read_bytes())!=expected: raise SystemExit("exact locked stable-hash archive missing/mismatched")
    extraction=helper.verify_registry_source(archives[0],home,"rustc-stable-hash","0.1.2")
    run([cargo,"build",*tail,"--offline","--release"],env=env)
    rng=random.Random(0xF377060)
    edge=[0,1,2,9,10,15,16,255,256,(1<<31)-1,1<<31,(1<<32)-1,1<<32,(1<<63)-1,1<<63,(1<<64)-2,(1<<64)-1]
    requests=[]
    for a in edge:
        for b in edge: requests.append(f"V\t{a}\t{b}\t{b}\t{a}")
    for _ in range(6000): requests.append("V\t"+"\t".join(str(rng.getrandbits(64)) for _ in range(4)))
    for a,b in [(v,0) for v in edge]+[(0,v) for v in edge]+[(v,v) for v in edge]:
        for n in range(len(FORMATS)):
            for width in (64,128): requests.append(f"F\t{a}\t{b}\t{n}\t{width}")
    request=("\n".join(requests)+"\n").encode()
    raw=run([str(cache/"target/release/oracle")],input=request,env=env)
    fixture=root/"port/rust/compiler/rustc_hashes/src/testdata/oracle.tsv.gz"
    manifest=fixture.parent.parent/"oracle_manifest.json"
    compressed=None
    if fixture.exists():
        try:
            current=fixture.read_bytes()
            with gzip.GzipFile(fileobj=io.BytesIO(current)) as stream:
                if stream.read(len(raw)+1)==raw: compressed=current
        except (OSError,EOFError,zlib.error): pass
    if args.check and compressed is None: raise SystemExit("original hash fixture differs, missing or corrupt")
    if compressed is None: compressed=gzip.compress(raw,compresslevel=9,mtime=0)
    metadata={"rust_revision":PIN,"source_path":SOURCE,"source_git_blob":BLOB,"source_sha256":SHA,"rustc":version,"host":host,"source_version":git("show",f"{PIN}:src/version").decode().strip(),"profile":"Linux x86-64; original unsigned std::fmt and typed Hasher delegation", "dependency":{"name":"rustc-stable-hash","version":"0.1.2","sha256":expected,**extraction},"wrapper_sha256":sha(wrapper),"generator_sha256":sha(Path(__file__).read_bytes()),"extraction_verifier_sha256":sha((root/"tools/port-rust-edition/reference-build/prepare.py").read_bytes()),"cargo_manifest_sha256":sha((HERE/"Cargo.toml").read_bytes()),"cargo_lock_sha256":sha((HERE/"Cargo.lock").read_bytes()),"formats":[{"Pattern":p,"Options":o,"LowerHex":p.endswith("x}")} for p,o in FORMATS],"requests":len(requests),"records":raw.count(b"\n"),"request_sha256":sha(request),"raw_sha256":sha(raw),"fixture_sha256":sha(compressed)}
    encoded=(json.dumps(metadata,indent=2,ensure_ascii=False)+"\n").encode()
    if args.check:
        if manifest.read_bytes()!=encoded: raise SystemExit("hash provenance manifest differs")
    else:
        fixture.parent.mkdir(parents=True,exist_ok=True)
        fixture.write_bytes(compressed); manifest.write_bytes(encoded)
    print(f"hashes: {len(requests)} original requests; {metadata['records']} arithmetic, trait and formatter/failure records")

if __name__=="__main__":
    try: main()
    except subprocess.CalledProcessError as error:
        sys.stderr.write(error.stderr.decode(errors="replace")); raise SystemExit(error.returncode)
