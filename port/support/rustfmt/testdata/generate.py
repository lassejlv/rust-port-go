#!/usr/bin/env python3
"""Regenerate scoped native std debug-builder observations; never a Rust port."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[3]
VERSION="rustc 1.99.1 (58cf8f9c3 2026-10-13)"
def digest(data):return hashlib.sha256(data).hexdigest()
def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rustc",default=os.environ.get("FERRUGO_RUSTC",str(ROOT/".toolchain/rust/bin/rustc")))
    parser.add_argument("--check",action="store_true")
    args=parser.parse_args()
    compiler=shutil.which(args.rustc) if args.rustc else None
    if not compiler:raise SystemExit("explicit Rust compiler is missing or empty")
    version=subprocess.check_output([compiler,"--version"],text=True).strip()
    if version!=VERSION:raise SystemExit(f"expected {VERSION}; found {version}")
    source=(HERE/"native.rs").read_bytes()
    with tempfile.TemporaryDirectory(prefix="ferrugo-rustfmt-") as temporary:
        binary=str(Path(temporary)/"probe")
        subprocess.run([compiler,"--edition=2024",str(HERE/"native.rs"),"-o",binary],check=True,timeout=30)
        raw=subprocess.check_output([binary],timeout=30)
    fixture=HERE/"native.tsv.gz"
    compressed=None
    if fixture.exists():
        try:
            previous=fixture.read_bytes()
            if gzip.decompress(previous)==raw:compressed=previous
        except(OSError,EOFError):pass
    if args.check and compressed is None:raise SystemExit("native reference differs, invalid or missing")
    if compressed is None:compressed=gzip.compress(raw,compresslevel=9,mtime=0)
    meta={"reference_rustc":version,"reference_scope":"Observed std Debug builders only; supporting adapter, no standard-library source completion credit","cases":raw.count(b"\n"),"source_sha256":digest(source),"raw_sha256":digest(raw),"fixture_sha256":digest(compressed)}
    encoded=(json.dumps(meta,indent=2)+"\n").encode()
    manifest=HERE/"manifest.json"
    if args.check:
        if manifest.read_bytes()!=encoded:raise SystemExit("native reference manifest differs")
    else:fixture.write_bytes(compressed);manifest.write_bytes(encoded)
    print(f"rustfmt: {meta['cases']} native builder/argument/writer/failure observations")
if __name__=="__main__":main()
