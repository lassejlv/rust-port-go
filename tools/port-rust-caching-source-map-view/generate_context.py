#!/usr/bin/env python3
"""Run actual caching operations in normal, unwinding and panic-hook scopes."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import resource
import subprocess
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
sys.path.insert(0, str(HERE))
import generate


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--prepared", action="store_true")
    args = parser.parse_args()
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    rustc, env, compiler = generate.native_profile.compiler_profile()
    reference = ROOT / ".toolchain/oracles/rustc-span-reference"
    cache = ROOT / ".toolchain/oracles/caching-source-map-view"
    cache.mkdir(parents=True, exist_ok=True)
    examples = [("0", "N"), ("1", "N"), ("1", "b,0"), ("2", "b,2"), ("2", "s,1,4"),
                ("I,0,5,L,-", "b,0"), ("I,2,5,L,-", "b,5"),
                ("I,0,5,L,2", "s,0,1"), ("I,0,5,L,0~2~4", "s,0,5"),
                ("I,0,5,R,1,0,", "b,0"), ("I,0,5,R,1,3,", "s,0,1"),
                ("I,0,4294967295,R,3,4,ffffffff02000000", "b,0"),
                ("I,2,4294967295,L,0~2", "b,5"),
                ("I,0,4294967295,L,0~4294967295", "s,0,4294967295")]
    records, profiles = [], []
    for debug in (False, True):
        for overflow in (False, True):
            name = f"d{int(debug)}o{int(overflow)}"
            profile, env = generate.reference.load_profile(reference / f"build-profile-{name}.json")
            rustc = profile["rustc"]
            if profile["source_revision"] != generate.native_profile.REVISION or profile["debug_assertions"] != debug or profile["overflow_checks"] != overflow:
                raise SystemExit("native reference profile differs")
            profiles.append({"debug_assertions": debug, "overflow_checks": overflow,
                             "provenance_sha256": generate.sha(Path(profile["provenance_path"]).read_bytes())})
            binary = cache / ("context-" + name)
            generate.run([rustc, str(HERE / "context_oracle.rs"), *profile["link_arguments"], "-o", str(binary)], env)
            for mode in ("normal", "caught", "escaping", "processing"):
                for header, operation in examples:
                    result = subprocess.run([str(binary), mode, header, operation], env=env,
                                            capture_output=True, timeout=10)
                    if result.returncode not in (0, -6, 134):
                        raise SystemExit("unexpected native context failure: " + result.stderr.decode(errors="replace"))
                    lines = result.stdout.decode().splitlines()
                    if result.returncode == 0 and (not lines or not lines[-1].startswith("R:")):
                        raise SystemExit("missing native context result")
                    if result.returncode != 0 and any(line.startswith("R:") for line in lines):
                        raise SystemExit("abort unexpectedly returned")
                    records.append({"profile": name, "mode": mode, "header": header, "operation": operation,
                                    "aborted": result.returncode != 0, "lines": lines})
    raw = b"".join((json.dumps(row, separators=(",", ":")) + "\n").encode() for row in records)
    fixture = ROOT / "port/rust/compiler/rustc_span/src/testdata/caching_source_map_view_context.jsonl.gz"
    compressed = gzip.compress(raw, compresslevel=9, mtime=0)
    if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw:
        compressed = fixture.read_bytes()
    sha = generate.sha
    files = [HERE / name for name in ["generate_context.py", "generate.py", "context_oracle.rs", "oracle.rs"]]
    files.append(ROOT / "tools/port-rust-symbol/native_profile.py")
    files += [ROOT / "tools/port-rust-span-lib/native_source_helpers.rs",
              ROOT / "port/rust/compiler/rustc_span/src/caching_source_map_view.go",
              ROOT / "port/rust/compiler/rustc_span/src/caching_source_map_view_context_test.go"]
    files = sorted(set(files + generate.span_wrapper_evidence.inputs()))
    metadata = {"complete": False, "scope": "Actual-source internal failure and complete external cleanup/runtime witness; no std panic-hook or SIGABRT ABI port credit.",
                "rust_revision": generate.native_profile.REVISION, **compiler,
                "profiles": profiles,
                "records": len(records), "native_aborts": sum(row["aborted"] for row in records),
                "raw_sha256": sha(raw), "fixture_sha256": sha(compressed),
                "files": [{"path": str(path.relative_to(ROOT)), "sha256": sha(path.read_bytes())} for path in files]}
    manifest = HERE / "context-manifest.json"
    encoded = (json.dumps(metadata, indent=2) + "\n").encode()
    if args.check:
        if not fixture.exists() or fixture.read_bytes() != compressed or not manifest.exists() or manifest.read_bytes() != encoded:
            raise SystemExit("context evidence differs")
    else:
        fixture.write_bytes(compressed)
        manifest.write_bytes(encoded)
    print(f"Caching contexts: {len(records)} native records, {metadata['native_aborts']} isolated aborts")


if __name__ == "__main__":
    main()
