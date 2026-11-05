#!/usr/bin/env python3
"""Check source allocation relationships without comparing Rust/Go addresses."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys

import native_profile
import symbol_reference

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def run(command, env):
    result = subprocess.run(command, env=env, capture_output=True, timeout=180)
    if result.returncode:
        raise SystemExit(f"command failed: {command}\n{result.stderr.decode(errors='replace')[:4096]}")
    return result.stdout


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--prepared", action="store_true")
    args = parser.parse_args()
    cache = ROOT / ".toolchain/oracles/symbol-borrows"
    cache.mkdir(parents=True, exist_ok=True)
    reference = ROOT / ".toolchain/oracles/rustc-span-reference"
    outputs, profiles = [], []
    for debug in (False, True):
        for overflow in (False, True):
            profile, env, compiler = symbol_reference.load(debug, overflow)
            rustc = profile['rustc']
            name = f"d{int(debug)}o{int(overflow)}"
            if profile["debug_assertions"] != debug or profile["overflow_checks"] != overflow:
                raise SystemExit("reference profile mismatch: " + name)
            if profile["cfg_release"] != "1.101.0":
                raise SystemExit("unexpected compiler release environment")
            binary = cache / ("oracle-" + name)
            run([rustc, str(HERE / "borrow_oracle.rs"), *profile["link_arguments"], "-o", str(binary)], env)
            outputs.append(run([str(binary)], env))
            provenance = Path(profile["provenance_path"])
            profiles.append({"debug_assertions": debug, "overflow_checks": overflow,
                             "source_provenance_sha256": sha(provenance.read_bytes())})
    if len(set(outputs)) != 1:
        raise SystemExit("source borrow relationships differ between profiles")
    raw = outputs[0]
    if len(raw.splitlines()) != 7:
        raise SystemExit("source borrow observation count changed")
    base = ROOT / "port/rust/compiler/rustc_span/src"
    bindings = [HERE / "generate_borrow_contract.py", HERE / "borrow_oracle.rs", HERE / "native_profile.py",
                base / "symbol.go", base / "symbol_context_ord.go", base / "symbol_borrow_test.go"]
    source = native_profile.source_bytes("compiler/rustc_span/src/symbol.rs")
    metadata = {"scope": "Fresh interner storage, occupied and cross-kind borrows, static prefill and STDLIB_STABLE_CRATES storage; no cross-language address or allocator ABI claim.",
                "rust_revision": native_profile.REVISION, **compiler,
                "source_sha256": sha(source), "profiles": profiles, "reference_inputs": symbol_reference.inputs(), "observations_per_profile": 7,
                "fixture_sha256": sha(raw),
                "files": [{"path": str(path.relative_to(ROOT)), "sha256": sha(path.read_bytes())} for path in bindings]}
    encoded = (json.dumps(metadata, indent=2) + "\n").encode()
    fixture = base / "testdata/symbol_borrows.txt"
    manifest = HERE / "borrow-validation.json"
    if args.check:
        if not fixture.exists() or fixture.read_bytes() != raw or not manifest.exists() or manifest.read_bytes() != encoded:
            raise SystemExit("source borrow evidence differs")
    else:
        fixture.parent.mkdir(parents=True, exist_ok=True)
        fixture.write_bytes(raw)
        manifest.write_bytes(encoded)
    print("Symbol storage: 7 native observations match in all 4 profiles")


if __name__ == "__main__":
    main()
