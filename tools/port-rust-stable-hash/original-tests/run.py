#!/usr/bin/env python3
"""Run unchanged official stable-hash tests and examples in every profile."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
CHECKSUM = "781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08"


def source_files(root):
    home = Path(os.environ.get("CARGO_HOME", root / ".toolchain/port-cargo-home")).resolve()
    matches = list((home / "registry/cache").glob("*/rustc-stable-hash-0.1.2.crate"))
    if len(matches) != 1 or matches[0].is_symlink() or hashlib.sha256(matches[0].read_bytes()).hexdigest() != CHECKSUM:
        raise SystemExit("missing or mismatched official rustc-stable-hash0.1.2 archive")
    with tarfile.open(matches[0]) as archive:
        return {m.name.removeprefix("rustc-stable-hash-0.1.2/"): archive.extractfile(m).read() for m in archive if m.isfile()}


def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists() or path.read_bytes() != data:
        path.write_bytes(data)


def executable(value):
    resolved = os.path.abspath(shutil.which(value) or value)
    if not os.access(resolved, os.X_OK):
        raise SystemExit("unavailable executable: " + value)
    return resolved


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--offline", action="store_true")
    parser.add_argument("--target")
    parser.add_argument("--compile-only", action="store_true")
    args = parser.parse_args()
    root = args.root.resolve()
    originals = source_files(root)
    destination = root / ".toolchain/oracles/stable-hash-original-tests"
    for name, data in originals.items():
        write(destination / "original" / name, data)
    for name in ("src/sip128/tests.rs", "src/stable_hasher/tests.rs"):
        if (HERE / "fixtures" / name).read_bytes() != originals[name]:
            raise SystemExit("retained original test fixture differs: " + name)
    environment = os.environ.copy()
    environment["CARGO_HOME"] = str(Path(environment.get("CARGO_HOME", root / ".toolchain/port-cargo-home")).resolve())
    environment["RUSTC"] = executable(environment.get("FERRUGO_RUSTC", str(root / ".toolchain/rust/bin/rustc")))
    environment["RUSTDOC"] = executable(environment.get("FERRUGO_RUSTDOC") or environment.get("RUSTDOC") or str(Path(environment["RUSTC"]).with_name("rustdoc")))
    environment["RUSTC_BOOTSTRAP"] = "1"
    environment["CARGO_TARGET_DIR"] = str(destination / "target")
    environment.pop("CARGO_ENCODED_RUSTFLAGS", None)
    cargo = executable(environment.get("FERRUGO_CARGO", str(root / ".toolchain/rust/bin/cargo")))
    tail = ["--manifest-path", str(destination / "original/Cargo.toml"), "--locked"]
    if args.offline:
        tail.append("--offline")
    if args.target:
        tail += ["--target", args.target]
    compiler = subprocess.check_output([environment["RUSTC"], "-vV"], text=True).strip()
    host = next(line.removeprefix("host: ") for line in compiler.splitlines() if line.startswith("host: "))
    target = args.target or host
    target_cfg = subprocess.check_output([environment["RUSTC"], "--print", "cfg", "--target", target], text=True).splitlines()
    target_arch = next(line.removeprefix('target_arch="').removesuffix('"') for line in target_cfg if line.startswith('target_arch="'))
    expected = 15 if target_arch in ("x86_64", "x86", "arm") else 14
    profiles = []
    for nightly in (False, True):
        for checked in (False, True):
            for debug in (False, True):
                name = f"{'nightly' if nightly else 'default'}-overflow-{str(checked).lower()}-debug-{str(debug).lower()}"
                current = environment.copy()
                current["RUSTFLAGS"] = f"-Coverflow-checks={str(checked).lower()} -Cdebug-assertions={str(debug).lower()}"
                current["RUSTDOCFLAGS"] = current["RUSTFLAGS"]
                features = ["--features", "nightly"] if nightly else []
                entry = {"name": name, "nightly": nightly, "overflow_checks": checked, "debug_assertions": debug, "rustflags": current["RUSTFLAGS"], "target": target, "target_arch": target_arch, "compile_only": args.compile_only}
                for kind in (("tests",) if args.compile_only else ("tests", "docs")):
                    command = [cargo, "test", *tail, *features, "--lib" if kind == "tests" else "--doc"]
                    if args.compile_only:
                        command.append("--no-run")
                    else:
                        command += ["--", "--test-threads=1"]
                    result = subprocess.run(command, env=current, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                    write(destination / "logs" / f"{name}.{kind}.stdout", result.stdout)
                    write(destination / "logs" / f"{name}.{kind}.stderr", result.stderr)
                    if result.returncode:
                        raise SystemExit(f"original stable-hash {name}/{kind} failed:\n{result.stdout.decode(errors='replace')}\n{result.stderr.decode(errors='replace')}")
                    passed = [line[5:-7] for line in result.stdout.decode().splitlines() if line.startswith("test ") and line.endswith(" ... ok")]
                    if not args.compile_only and len(passed) != (expected if kind == "tests" else 3):
                        raise SystemExit(f"original {name}/{kind} ran {len(passed)} tests, expected {expected if kind == 'tests' else 3}")
                    entry[kind] = {"passed": len(passed), "names": passed, "stdout_sha256": hashlib.sha256(result.stdout).hexdigest(), "stderr_sha256": hashlib.sha256(result.stderr).hexdigest()}
                profiles.append(entry)
                print(name + (": original tests compiled without execution" if args.compile_only else f": {expected} original tests and 3 doc examples passed"))
    for name, data in originals.items():
        if (destination / "original" / name).read_bytes() != data:
            raise SystemExit("original source changed during execution: " + name)
    witness = HERE / "input_dispatch.rs"
    witness_binary = destination / "input-dispatch"
    command = [environment["RUSTC"], "--edition=2021", str(witness), "-o", str(witness_binary)]
    if args.target:
        command += ["--target", args.target]
    subprocess.run(command, env=environment, check=True)
    dispatch = {"source_sha256": hashlib.sha256(witness.read_bytes()).hexdigest(), "target_arch": target_arch, "compile_only": args.compile_only}
    if not args.compile_only:
        events = subprocess.check_output([str(witness_binary)], env=environment, text=True).splitlines()
        dispatch["events"] = {name: trace.split("|") for name, trace in (line.split("\t") for line in events)}
        fixture = root / "port/dependencies/rustc-stable-hash/v0_1_2/tests/testdata/input_dispatch.json"
        write(fixture, (json.dumps(dispatch, indent=2) + "\n").encode())
    evidence = {"crate": "rustc-stable-hash", "version": "0.1.2", "archive_sha256": CHECKSUM, "compiler": compiler, "source_files": [{"path": name, "sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)} for name, data in sorted(originals.items())], "profiles": profiles, "compiler_hash_input_dispatch": dispatch, "scope": "All original crate bytes and Cargo manifest unchanged; complete original unit tests and source doctests. RUSTC_BOOTSTRAP only enables the original nightly feature. No Go implementation credit is implied by this Rust test proof."}
    write(destination / "evidence.json", (json.dumps(evidence, indent=2) + "\n").encode())


if __name__ == "__main__":
    main()
