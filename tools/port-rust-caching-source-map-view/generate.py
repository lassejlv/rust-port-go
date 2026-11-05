#!/usr/bin/env python3
"""Compare the caching view against the complete unchanged rustc_span crate."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import random
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT / "tools/port-rust-symbol"))
import native_profile
sys.path.insert(0, str(ROOT / "tools/port-rust-span-lib"))
import reference
import span_wrapper_evidence


def sha(data):
    return hashlib.sha256(data).hexdigest()


def run(command, env, data=None):
    result = subprocess.run(command, env=env, input=data, capture_output=True, timeout=180)
    if result.returncode:
        raise SystemExit(f"command failed: {command}\n{result.stderr.decode(errors='replace')[:4096]}")
    return result.stdout


def cases():
    result = ["0|"]
    points = [*range(57), 63, 255, (1 << 16)-1, (1 << 32)-2, (1 << 32)-1]
    for count in range(1, 10):
        result.append(f"{count}|" + ";".join(f"b,{pos}" for pos in points + points[::-1]))
        for pos in points:
            result.append(f"{count}|b,{pos};b,0;b,{pos};s,{pos},{pos};b,{pos}")
    # Every endpoint pair includes reversed spans, UTF-8 continuation bytes,
    # CRLF/BOM normalization, empty/trailing-newline files and EOF misses.
    for lo in points:
        for hi in points:
            prime = (lo * 17 + hi * 3) % 57
            result.append(f"9|b,{prime};s,{lo},{hi};b,{hi};s,{hi},{lo};b,{prime}")
    rng = random.Random(0xCA5E20261104)
    for _ in range(256):
        count = rng.randrange(1, 10)
        ops = []
        current = count
        for _ in range(48):
            kind = rng.randrange(8)
            if kind == 0 and current < 9:
                ops.append("a")
                current += 1
            elif kind < 4:
                ops.append(f"b,{rng.choice(points)}")
            else:
                ops.append(f"s,{rng.choice(points)},{rng.choice(points)}")
        result.append(f"{count}|" + ";".join(ops))
    return result


def metadata_cases():
    result = []
    maximum = (1 << 32) - 1
    vectors = [[], [0], [0, 2, 4], [0, 0, 0], [0, 256, 65536], [0, maximum],
               [2], [1, 3, 5], [5, 0, 2], [maximum, 0]]
    for prefix in (0, 2):
        for length in (0, 1, 4, 5, 256, 65536, maximum-1, maximum):
            start = 0 if prefix == 0 else 5
            points = sorted({0, 1, 2, 3, 4, 5, start, start+1, 255, 256, 65535, 65536,
                             min(maximum, start+length), min(maximum, start+length+1), maximum-1, maximum})
            script = ["m"]
            for pos in points + points[::-1]:
                script.extend([f"b,{pos}", "m", f"s,{start},{pos}", "m", f"s,{pos},{start}", "m"])
            for vector in vectors:
                spec = "~".join(map(str, vector)) or "-"
                modes = ["L", "F"]
                if not vector or vector[0] == 0 and all(a <= b for a, b in zip(vector, vector[1:])):
                    modes.append("D")
                for mode in modes:
                    result.append(f"I,{prefix},{length},{mode},{spec}|" + ";".join(script))
            for count, width, raw in [(0, 0, ""), (1, 0, ""), (1, 1, ""), (1, 2, ""),
                                      (1, 3, ""), (1, 4, ""), (3, 1, "0203"),
                                      (3, 2, "00010100"), (3, 4, "ffffffff02000000"),
                                      (3, 3, "010000020000")]:
                result.append(f"I,{prefix},{length},R,{count},{width},{raw}|" + ";".join(script))
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--prepared", action="store_true")
    parser.add_argument("--metadata", action="store_true")
    args = parser.parse_args()
    rustc, env, compiler = native_profile.compiler_profile()
    cache = ROOT / ".toolchain/oracles/caching-source-map-view"
    cache.mkdir(parents=True, exist_ok=True)
    reference_root = ROOT / ".toolchain/oracles/rustc-span-reference"
    requests = metadata_cases() if args.metadata else cases()
    input_bytes = ("\n".join(requests) + "\n").encode()
    records, profiles = [], []
    for debug in (False, True):
        for overflow in (False, True):
            name = f"d{int(debug)}o{int(overflow)}"
            profile, env = reference.load_profile(reference_root / f"build-profile-{name}.json")
            rustc = profile["rustc"]
            if profile["debug_assertions"] != debug or profile["overflow_checks"] != overflow:
                raise SystemExit("reference profile mismatch: " + name)
            binary = cache / ("oracle-" + name)
            run([rustc, str(HERE / "oracle.rs"), *profile["link_arguments"], "-o", str(binary)], env)
            output = run([str(binary)], env, input_bytes).decode().splitlines()
            if len(output) != len(requests):
                raise SystemExit("native request/output count differs")
            records.extend(f"{name}\t{request}\t{response}\n" for request, response in zip(requests, output))
            profiles.append({"debug_assertions": debug, "overflow_checks": overflow,
                             "source_provenance_sha256": sha(Path(profile["provenance_path"]).read_bytes())})
    raw = "".join(records).encode()
    base = ROOT / "port/rust/compiler/rustc_span/src"
    stem = "caching_source_map_view_metadata" if args.metadata else "caching_source_map_view"
    fixture = base / f"testdata/{stem}.tsv.gz"
    data = gzip.compress(raw, compresslevel=9, mtime=0)
    if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw:
        data = fixture.read_bytes()
    source = native_profile.source_bytes("compiler/rustc_span/src/caching_source_map_view.rs")
    bindings = sorted(set(span_wrapper_evidence.inputs() + [HERE / "generate.py", HERE / "oracle.rs",
                ROOT / "tools/port-rust-span-lib/native_source_helpers.rs",
                base / "caching_source_map_view.go", base / "caching_source_map_view_oracle_test.go"]))
    metadata = {"complete": False,
                "scope": "Actual complete-crate normal or imported/lazy caching operations, cache mutation and ownership counts in all four arithmetic profiles. Contextual internal failures are retained separately.",
                "rust_revision": native_profile.REVISION, **compiler,
                "source_sha256": sha(source), "profiles": profiles,
                "requests_per_profile": len(requests), "records": len(records),
                "steps": 4 * sum(len(request.split('|')[1].split(';')) for request in requests if request.split('|')[1]),
                "request_sha256": sha(input_bytes), "raw_sha256": sha(raw), "fixture_sha256": sha(data),
                "files": [{"path": str(path.relative_to(ROOT)), "sha256": sha(path.read_bytes())} for path in bindings]}
    encoded = (json.dumps(metadata, indent=2) + "\n").encode()
    manifest = HERE / ("metadata-manifest.json" if args.metadata else "manifest.json")
    if args.check:
        if not fixture.exists() or fixture.read_bytes() != data or not manifest.exists() or manifest.read_bytes() != encoded:
            raise SystemExit("caching view evidence differs")
    else:
        fixture.parent.mkdir(parents=True, exist_ok=True)
        fixture.write_bytes(data)
        manifest.write_bytes(encoded)
    print(json.dumps({key: metadata[key] for key in ["records", "steps", "complete"]}))


if __name__ == "__main__":
    main()
