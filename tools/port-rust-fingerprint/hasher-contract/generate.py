#!/usr/bin/env python3
"""Prove complete/default Hasher calls without changing the old large corpus."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import resource
import shutil
import subprocess
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def executable(flag, env_name, local, root):
    value = flag if flag is not None else os.environ.get(env_name)
    if value is None:
        value = str(root / local) if (root / local).is_file() else Path(local).name
    if not value:
        raise SystemExit("empty executable override: " + env_name)
    candidate = os.path.abspath(root / value) if "/" in value else shutil.which(value)
    if not candidate or not os.path.isfile(candidate) or not os.access(candidate, os.X_OK):
        raise SystemExit("unavailable executable: " + value)
    return os.path.abspath(candidate)


def run(command, **kwargs):
    p = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180, **kwargs)
    if p.returncode:
        raise SystemExit(f"command failed: {command}\n{p.stderr.decode(errors='replace')}")
    return p.stdout


def main():
    # The proof observes abort class and callback prefixes, not core files.
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--rustc")
    parser.add_argument("--cargo")
    parser.add_argument("--go")
    parser.add_argument("--offline", action="store_true")
    parser.add_argument("--fetch", action="store_true")
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    root = args.root.resolve()
    env = os.environ.copy()
    for key, value in (("FERRUGO_RUSTC", executable(args.rustc, "FERRUGO_RUSTC", ".toolchain/rust/bin/rustc", root)),
                       ("FERRUGO_CARGO", executable(args.cargo, "FERRUGO_CARGO", ".toolchain/rust/bin/cargo", root)),
                       ("FERRUGO_GO", executable(args.go, "FERRUGO_GO", ".toolchain/go/bin/go", root))):
        env[key] = value
    env.setdefault("GOCACHE", "/tmp/ferrugo-go-cache")
    env["RUSTC_BOOTSTRAP"] = "1"
    cache = root / ".toolchain/oracles/def-id-reference"
    records, builds = [], []
    pairs = [(0, 0), (0, 1), (1, 0), (1, 2), ((1 << 64)-1, 1), (1 << 63, (1 << 64)-1)]
    for debug in (False, True):
        reference = root / "tools/port-rust-def-id/reference-build"
        run([sys.executable, str(reference / "prepare.py"), "--root", str(root), "--debug", str(debug).lower(),
             *(["--offline"] if args.offline else []), *(["--fetch"] if args.fetch else [])], env=env)
        profile = json.loads((cache / "build-profile.json").read_text())
        deps = Path(profile["library"]).parent / "deps"
        def library(name):
            paths = list(deps.glob("lib"+name+"-*.rlib"))
            if len(paths) != 1:
                raise SystemExit("ambiguous artifact: "+name)
            return str(paths[0])
        binary = cache / ("hasher-contract-debug" if debug else "hasher-contract-release")
        run([env["FERRUGO_RUSTC"], "--edition=2024", "-C", "opt-level=1", "-C", f"debug-assertions={str(debug).lower()}",
             str(HERE / "oracle.rs"), *profile["link_arguments"], "--extern", "serde_json="+library("serde_json"),
             "--extern", "rustc_hashes="+library("rustc_hashes"), "-o", str(binary)], env=env)
        builds.append({"debug_assertions": debug, "rust_revision": profile["revision"], "compiler": profile["compiler"],
                       "source_provenance_sha256": profile["source_provenance_sha256"], "registry_dependencies": profile["registry_dependencies"]})
        for kind in ("hash64", "hash128", "fingerprint", "packed", "hash64_slice", "hash128_slice", "fingerprint_slice", "packed_slice", "hash64_empty", "hash128_empty", "fingerprint_empty", "packed_empty"):
            for hasher in ("raw", "typed", "unhash", "ref_unhash", "wrapper_unhash"):
                if hasher in ("raw", "typed") and not debug:
                    continue  # custom impls contain no debug-conditional body
                for first, second in pairs:
                    if kind.endswith("_empty") and (first, second) != pairs[0]:
                        continue
                    for context in ("normal", "inherited", "processing"):
                        for fail in ((-1, 0, 1) if hasher in ("raw", "typed") else (-1,)):
                            for seed in ((7,) if hasher in ("raw", "typed") else (0, 7)):
                                q = dict(kind=kind, hasher=hasher, first=first, second=second, seed=seed,
                                         context=context, fail=fail, debug=debug)
                                p = subprocess.run([str(binary)], input=(json.dumps(q)+"\n").encode(), env=env,
                                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10)
                                events, result = [], None
                                for line in p.stdout.decode().splitlines():
                                    if line.startswith("E:"):
                                        events.append(line[2:])
                                    elif line.startswith("R:"):
                                        result = json.loads(line[2:])
                                if p.returncode:
                                    if p.returncode not in (-6, 134) or context != "processing" or result is not None:
                                        raise SystemExit(f"unexpected native failure: {q}\n{p.stderr.decode(errors='replace')}")
                                    want = {"events": events, "aborted": True}
                                else:
                                    if result is None or result["events"] != events:
                                        raise SystemExit("missing/inconsistent original observation")
                                    want = result
                                for adapter in ("direct", "dictionary"):
                                    records.append({"request": dict(q, adapter=adapter), "want": want})
    raw = b"".join((json.dumps(row, separators=(",", ":"), ensure_ascii=False)+"\n").encode() for row in records)
    fixture = root / "port/rust/compiler/rustc_data_structures/src/testdata/fingerprint_hash_contract.jsonl.gz"
    data = gzip.compress(raw, compresslevel=9, mtime=0)
    if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw:
        data = fixture.read_bytes()
    helpers = [HERE / "generate.py", HERE / "oracle.rs", *[p for p in sorted(reference.glob("*")) if p.is_file()], root / "tools/port-rust-span-encoding/prepare.py"]
    ports = [root / "port/rust/compiler/rustc_hashes/src/hash_primary.go", root / "port/rust/compiler/rustc_data_structures/src/fingerprint_hash_primary.go", root / "port/rust/compiler/rustc_data_structures/src/fingerprint_hash_primary_test.go"]
    metadata = {"scope": "Complete required/default Hasher dispatch for existing Hash64/Hash128/Fingerprint/PackedFingerprint Hash; exact Unhasher specialization and reference-wrapper distinction. No new source-file credit.",
                "builds": builds, "native_cases": len(records)//2, "go_comparisons": len(records),
                "native_aborts": sum(bool(r["want"].get("aborted")) for r in records)//2,
                "go_abort_replays": sum(bool(r["want"].get("aborted")) for r in records),
                "raw_sha256": sha(raw), "fixture_sha256": sha(data),
                "files": [{"path": str(p.relative_to(root)), "sha256": sha(p.read_bytes())} for p in [*helpers, *ports]]}
    encoded = (json.dumps(metadata, indent=2)+"\n").encode()
    manifest = HERE / "validation.json"
    if args.check:
        if not fixture.exists() or fixture.read_bytes() != data or not manifest.exists() or manifest.read_bytes() != encoded:
            raise SystemExit("Hasher contract evidence differs")
    else:
        fixture.parent.mkdir(parents=True, exist_ok=True)
        fixture.write_bytes(data)
        manifest.write_bytes(encoded)
    run([env["FERRUGO_GO"], "test", "./port/rust/compiler/rustc_data_structures/src", "-run", "^TestFullHasherContract$", "-count=1"], cwd=root, env=env)
    print(f"Hasher contract: {metadata['native_cases']} native cases, {metadata['native_aborts']} native aborts; {len(records)} direct/dictionary comparisons, {metadata['go_abort_replays']} abort replays match")


if __name__ == "__main__":
    main()
