#!/usr/bin/env python3
"""Retain exact native panic-operation evidence before neutral integration."""
import argparse
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import resource
import signal
import subprocess
import tempfile

HERE = Path(__file__).absolute().parent
ROOT = HERE.parents[1]
REFERENCE = ROOT / "tools/port-rust-span-reference"
spec = importlib.util.spec_from_file_location("panic_runtime_reference", REFERENCE / "prepare.py")
reference = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reference)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def no_core():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--rustc")
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    rustc = reference.executable(args.rustc, "FERRUGO_RUSTC", Path(".toolchain/rust-nightly-2026-11-04/bin/rustc"))
    selection = json.loads((REFERENCE / "toolchain.json").read_text())
    version = subprocess.run([rustc, "--version"], cwd=ROOT, check=True, capture_output=True, timeout=10).stdout.decode().strip()
    if version != selection["rustc"]:
        raise SystemExit("panic prototype requires the exact retained official nightly")
    requests = [(mode, action, "observe") for mode in ("normal", "inherited_caught", "inherited_escape") for action in ("return", "static", "owned", "opaque", "expect", "debug_panic", "resume_static", "resume_opaque", "double")]
    requests += [("normal", action, hook) for action in ("return", "static", "owned", "opaque", "expect", "resume_static") for hook in ("panic", "resume", "drop_panic")]
    rows = []
    with tempfile.TemporaryDirectory(prefix="ferrugo-panic-runtime-") as tmp:
        binary = Path(tmp) / "prototype"
        result = subprocess.run([rustc, "--edition=2024", "-C", "panic=unwind", str((HERE / "prototype.rs").relative_to(ROOT)), "-o", str(binary)], cwd=ROOT, capture_output=True, timeout=60)
        if result.returncode:
            raise SystemExit(result.stderr.decode(errors="replace"))
        for mode, action, hook in requests:
            result = subprocess.run([str(binary), mode, action, hook], cwd=ROOT, capture_output=True, timeout=10, preexec_fn=no_core)
            if result.returncode not in (0, -signal.SIGABRT):
                raise SystemExit(f"unexpected native status {result.returncode}: {mode}/{action}/{hook}\n{result.stderr.decode(errors='replace')}")
            rows.append({"request": {"mode": mode, "action": action, "hook": hook}, "want": {"abort": result.returncode == -signal.SIGABRT, "output": result.stdout.decode()}})
    raw = b"".join((json.dumps(row, separators=(",", ":")) + "\n").encode() for row in rows)
    fixture = HERE / "prototype.jsonl.gz"
    encoded = gzip.compress(raw, mtime=0)
    if fixture.exists() and gzip.decompress(fixture.read_bytes()) == raw:
        encoded = fixture.read_bytes()
    metadata = {
        "complete": False,
        "scope": "Bounded integrated operation protocol: ordinary versus resume operations, exact payload kinds/locations and identity, formatting before hook, inherited catch/escape, hook recursion, local destruction and actual compiler-generated cleanup abort hooks. Explicit neutral implementation, complete authored caller/Drop witnesses and independent source-component integration are retained. No std file or crate credit.",
        "rustc": version,
        "toolchain_selection_sha256": sha((REFERENCE / "toolchain.json").read_bytes()),
        "prototype_sha256": sha((HERE / "prototype.rs").read_bytes()),
        "generator_sha256": sha(Path(__file__).read_bytes()),
        "go_bindings": {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in [ROOT / "port/support/rustfmt/formatter.go", ROOT / "port/support/rustfmt/panic.go", ROOT / "port/support/rustfmt/panic_test.go"]},
        "cases": len(rows),
        "aborts": sum(row["want"]["abort"] for row in rows),
        "raw_sha256": sha(raw),
        "fixture_sha256": sha(encoded),
    }
    manifest = HERE / "prototype-manifest.json"
    proof = (json.dumps(metadata, indent=2) + "\n").encode()
    if args.check:
        if fixture.read_bytes() != encoded or manifest.read_bytes() != proof:
            raise SystemExit("native panic design evidence differs")
    else:
        fixture.write_bytes(encoded)
        manifest.write_bytes(proof)
    print(f"{len(rows)} native panic operations, {metadata['aborts']} actual SIGABRT results")


if __name__ == "__main__":
    main()
