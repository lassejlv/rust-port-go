#!/usr/bin/env python3
"""Assemble unchanged pinned Rust sources into an isolated reference workspace."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tomllib

PIN = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
CRATES = ("rustc_index_macros", "rustc_serialize", "rustc_hashes")
INDEX_FILES = ("idx.rs", "slice.rs", "vec.rs", "vec/tests.rs")


def execute(arguments, **kwargs):
    return subprocess.run(arguments, check=True, **kwargs)


def git_bytes(upstream, *arguments):
    return subprocess.check_output(["git", "-C", str(upstream), *arguments])


def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists() or path.read_bytes() != data:
        path.write_bytes(data)


def prepare(root, destination):
    upstream = root / ".toolchain/upstream-rust"
    if git_bytes(upstream, "rev-parse", "HEAD").decode().strip() != PIN:
        raise SystemExit("upstream Rust checkout does not match the pinned revision")
    paths = [f"compiler/rustc_index/src/{name}" for name in INDEX_FILES]
    for crate in CRATES:
        paths.extend(git_bytes(upstream, "ls-tree", "-r", "--name-only", PIN, f"compiler/{crate}").decode().splitlines())
    provenance = []
    for path in sorted(paths):
        blob = git_bytes(upstream, "rev-parse", f"{PIN}:{path}").decode().strip()
        source = git_bytes(upstream, "show", blob)
        actual = hashlib.sha1(f"blob {len(source)}\0".encode() + source).hexdigest()
        if actual != blob:
            raise SystemExit(f"Git blob verification failed for {path}")
        write(destination / "original" / path, source)
        if path.startswith("compiler/rustc_index/src/"):
            write(destination / "wrapper" / path.removeprefix("compiler/rustc_index/src/"), source)
        for crate in CRATES:
            prefix = f"compiler/{crate}/"
            if path.startswith(prefix):
                relative = path.removeprefix(prefix)
                if relative != "Cargo.toml":
                    write(destination / "crates" / crate / relative, source)
        provenance.append({"path": path, "git_blob": blob, "sha256": hashlib.sha256(source).hexdigest(), "bytes": len(source)})
    for crate in CRATES:
        write(destination / "crates" / crate / "Cargo.toml", (HERE / f"{crate}.Cargo.toml").read_bytes())
    write(destination / "Cargo.toml", (HERE / "Cargo.toml").read_bytes())
    for name in ("lib.rs", "main.rs"):
        write(destination / "wrapper" / name, (HERE / name).read_bytes())
    lock = HERE / "Cargo.lock"
    if lock.exists():
        write(destination / "Cargo.lock", lock.read_bytes())
    elif not (destination / "Cargo.lock").exists():
        write(destination / "Cargo.lock", git_bytes(upstream, "show", f"{PIN}:Cargo.lock"))
    write(destination / "source-provenance.json", (json.dumps({"rust_revision": PIN, "sources": provenance}, indent=2) + "\n").encode())
    return upstream


def verify_lock(upstream, destination):
    root_lock = tomllib.loads(git_bytes(upstream, "show", f"{PIN}:Cargo.lock").decode())
    pinned = {(package["name"], package["version"]): package.get("checksum") for package in root_lock["package"] if "source" in package}
    lock_path = destination / "Cargo.lock"
    for package in tomllib.loads(lock_path.read_text())["package"]:
        if "source" in package and pinned.get((package["name"], package["version"])) != package.get("checksum"):
            raise SystemExit(f"standalone dependency differs from root Cargo.lock: {package['name']} {package['version']}")


def registry_provenance(destination, cargo_home):
    dependencies = []
    for package in tomllib.loads((destination / "Cargo.lock").read_text())["package"]:
        if "source" not in package:
            continue
        matches = list((cargo_home / "registry/cache").glob(f"*/{package['name']}-{package['version']}.crate"))
        if len(matches) != 1:
            raise SystemExit(f"missing or ambiguous registry archive: {package['name']} {package['version']}")
        archive_hash = hashlib.sha256(matches[0].read_bytes()).hexdigest()
        if archive_hash != package["checksum"]:
            raise SystemExit(f"registry archive checksum mismatch: {package['name']} {package['version']}")
        dependencies.append({"name": package["name"], "version": package["version"], "source": package["source"], "sha256": archive_hash})
    return dependencies


def validate(upstream, destination, cargo, environment, offline):
    environment = environment.copy()
    environment["RUSTC_BOOTSTRAP"] = "1"
    environment.pop("CARGO_ENCODED_RUSTFLAGS", None)
    compiler = subprocess.check_output([environment["RUSTC"], "-vV"], text=True).strip()
    cargo_version = subprocess.check_output([cargo, "--version"], text=True).strip()
    profiles = []
    # Cargo subcommands precede global command options; keep the reusable tail separate.
    tail = ["--locked", "--manifest-path", str(destination / "Cargo.toml")]
    if offline:
        tail.append("--offline")
    logs = destination / "logs"
    logs.mkdir(exist_ok=True)

    def run_profile(profile, features, flags):
        profile_environment = environment.copy()
        profile_environment["RUSTFLAGS"] = flags
        feature_flags = ["--features", ",".join(features)] if features else []
        commands = [
            ("tests", [cargo, "test", *tail, *feature_flags, "--lib", "--", "--test-threads=1"]),
            ("proof", [cargo, "run", *tail, *feature_flags, "--bin", "storage-reference-proof"]),
            ("expanded", [cargo, "rustc", *tail, *feature_flags, "--lib", "--profile", "test", "--", "-Zunpretty=expanded"]),
        ]
        runs = []
        for kind, command in commands:
            result = subprocess.run(command, env=profile_environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            log = result.stdout + result.stderr
            log_path = logs / f"{profile}-{kind}.log"
            write(log_path, log)
            if result.returncode:
                raise SystemExit(f"reference {profile} {kind} failed: {log_path}\n{log.decode(errors='replace')}")
            if kind == "tests" and b"5 passed; 0 failed" not in result.stdout:
                raise SystemExit("reference did not execute all five unchanged vec tests")
            if kind == "expanded":
                write(destination / f"expanded-{profile}.rs", result.stdout)
            runs.append({"kind": kind, "command": command, "exit_code": result.returncode, "log_sha256": hashlib.sha256(log).hexdigest()})
        profiles.append({"name": profile, "features": features, "rustc_bootstrap": "1", "rustflags": shlex.split(flags), "runs": runs})

    run_profile("normal", [], "")
    run_profile("nightly", ["nightly"], "-Zcrate-attr=feature(never_type)")

    build_environment = environment.copy()
    build_environment["RUSTFLAGS"] = "-Zcrate-attr=feature(never_type)"
    build = subprocess.run([cargo, "build", *tail, "--features", "nightly", "--lib", "--message-format=json"], env=build_environment, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    library = None
    for line in build.stdout.splitlines():
        event = json.loads(line)
        if event.get("reason") == "compiler-artifact" and event["target"]["name"] == "storage_reference":
            library = next((name for name in event["filenames"] if name.endswith(".rlib")), None)
    if library is None:
        raise SystemExit("Cargo did not report the actual storage_reference library artifact")
    stage0 = {}
    for line in git_bytes(upstream, "show", f"{PIN}:src/stage0").decode().splitlines():
        if line.startswith("compiler_"):
            key, value = line.split("=", 1)
            stage0[key] = value
    provenance = {
        "rust_revision": PIN,
        "pinned_source_version": git_bytes(upstream, "show", f"{PIN}:src/version").decode().strip(),
        "compiler": compiler,
        "cargo": cargo_version,
        "development_only": True,
        "scope": "Untouched selected index modules, complete index macro crate and complete serialization/hash implementation modules under the recorded compiler profiles; does not certify the ABI or layout of the complete pinned compiler",
        "compatibility_gate": "Rust 1.99.1 still gates the never type; only the feature gate is injected with -Zcrate-attr; original source bytes are unchanged",
        "declared_stage0": stage0,
        "registry_dependencies": registry_provenance(destination, Path(environment["CARGO_HOME"])),
        "source_provenance_sha256": hashlib.sha256((destination / "source-provenance.json").read_bytes()).hexdigest(),
        "standalone_lock_sha256": hashlib.sha256((destination / "Cargo.lock").read_bytes()).hexdigest(),
        "profiles": profiles,
        "nightly_library": library,
        "nightly_link_arguments": ["--extern", f"storage_reference={library}", "-L", f"dependency={destination / 'target/debug/deps'}"],
    }
    write(destination / "build-profile.json", (json.dumps(provenance, indent=2) + "\n").encode())
    print("normal/nightly: all five untouched vec tests, storage proof, codec proof and macro expansion passed")
    print(f"nightly library: {library}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--destination", type=Path)
    parser.add_argument("--cargo", default=os.environ.get("FERRUGO_CARGO"))
    parser.add_argument("--rustc", default=os.environ.get("FERRUGO_RUSTC"))
    parser.add_argument("--fetch", action="store_true")
    parser.add_argument("--offline", action="store_true")
    parser.add_argument("--write-lock", action="store_true")
    parser.add_argument("--validate", action="store_true")
    arguments = parser.parse_args()
    root = arguments.root.resolve()
    destination = (arguments.destination or root / ".toolchain/oracles/index-storage-reference").resolve()
    upstream = prepare(root, destination)
    cargo = arguments.cargo or str(root / ".toolchain/rust/bin/cargo")
    rustc = arguments.rustc or str(root / ".toolchain/rust/bin/rustc")
    environment = os.environ.copy()
    environment["CARGO_HOME"] = str(Path(environment.get("CARGO_HOME", root / ".toolchain/port-cargo-home")).resolve())
    environment["RUSTC"] = shutil.which(rustc) or str(Path(rustc).resolve())
    if arguments.fetch:
        command = [cargo, "fetch", "--manifest-path", str(destination / "Cargo.toml")]
        if (HERE / "Cargo.lock").exists():
            command.append("--locked")
        if arguments.offline:
            command.append("--offline")
        execute(command, env=environment)
    verify_lock(upstream, destination)
    if arguments.write_lock:
        write(HERE / "Cargo.lock", (destination / "Cargo.lock").read_bytes())
    if arguments.validate:
        validate(upstream, destination, cargo, environment, arguments.offline)
    print(destination)


if __name__ == "__main__":
    main()
