#!/usr/bin/env python3
"""Build unchanged Fingerprint and real codec/hash dependencies for comparison."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tomllib

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
PIN = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
SHARED = ROOT / "tools/port-rust-edition/reference-build"
SPEC = importlib.util.spec_from_file_location("fingerprint_reference_support", SHARED / "prepare.py")
SUPPORT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SUPPORT)
CRATES = ("rustc_serialize", "rustc_hashes")
SOURCE_PATHS = (
    "compiler/rustc_data_structures/src/fingerprint.rs",
    "compiler/rustc_data_structures/src/fingerprint/tests.rs",
    "compiler/rustc_data_structures/src/unhash.rs",
    "compiler/rustc_data_structures/src/stable_hash.rs",
)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--offline", action="store_true")
    parser.add_argument("--fetch", action="store_true")
    parser.add_argument("--write-lock", action="store_true")
    parser.add_argument("--release-assertions", action="store_true")
    args = parser.parse_args()
    root = args.root.resolve()
    upstream = root / ".toolchain/upstream-rust"
    destination = root / ".toolchain/oracles/fingerprint-reference"
    environment = os.environ.copy()
    if args.offline:
        environment["GIT_NO_LAZY_FETCH"] = "1"
    cargo = environment.get("FERRUGO_CARGO") or str(root / ".toolchain/rust/bin/cargo")
    rustc = environment.get("FERRUGO_RUSTC") or str(root / ".toolchain/rust/bin/rustc")
    environment["RUSTC"] = shutil.which(rustc) or str(Path(rustc).resolve())
    environment["CARGO_HOME"] = str(Path(environment.get("CARGO_HOME", root / ".toolchain/port-cargo-home")).resolve())
    environment["RUSTC_BOOTSTRAP"] = "1"
    environment["RUSTFLAGS"] = "-Zcrate-attr=feature(never_type) -Cdebug-assertions=" + ("false" if args.release_assertions else "true")
    environment.pop("CARGO_ENCODED_RUSTFLAGS", None)

    def git(*arguments):
        return subprocess.check_output(["git", "-C", str(upstream), *arguments], env=environment)

    if git("rev-parse", "HEAD").decode().strip() != PIN:
        raise SystemExit("upstream source revision differs")
    paths = list(SOURCE_PATHS)
    for crate in CRATES:
        paths += git("ls-tree", "-r", "--name-only", PIN, f"compiler/{crate}").decode().splitlines()
    originals, provenance = {}, []
    for path in sorted(paths):
        blob = git("rev-parse", f"{PIN}:{path}").decode().strip()
        data = git("show", blob)
        if hashlib.sha1(f"blob {len(data)}\0".encode() + data).hexdigest() != blob:
            raise SystemExit("source Git blob differs: " + path)
        originals[path] = data
        SUPPORT.write(destination / "original" / path, data)
        for crate in CRATES:
            prefix = f"compiler/{crate}/"
            if path.startswith(prefix) and path.removeprefix(prefix) != "Cargo.toml":
                SUPPORT.write(destination / "crates" / crate / path.removeprefix(prefix), data)
        provenance.append({"path": path, "git_blob": blob, "sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)})
    for crate in CRATES:
        SUPPORT.write(destination / "crates" / crate / "Cargo.toml", (SHARED / f"{crate}.Cargo.toml").read_bytes())
    for name in ("Cargo.toml", "lib.rs"):
        SUPPORT.write(destination / ("wrapper/lib.rs" if name == "lib.rs" else name), (HERE / name).read_bytes())
    for name in ("fingerprint.rs", "fingerprint/tests.rs", "unhash.rs"):
        SUPPORT.write(destination / "wrapper" / name, originals["compiler/rustc_data_structures/src/" + name])
    source = originals["compiler/rustc_data_structures/src/stable_hash.rs"].decode()
    stable = ["use crate::fingerprint::Fingerprint;", "pub use rustc_stable_hash::{FromStableHash, SipHasher128Hash as StableHasherHash, StableSipHasher128 as StableHasher};"]
    fragments = []
    for marker, simple in (
        ("pub trait StableHashCtxt {", False),
        ("pub struct DummyStableHashCtxt;", True),
        ("impl StableHashCtxt for DummyStableHashCtxt {", False),
        ("pub struct RawSpan(pub u64);", True),
        ("pub struct RawDefId(pub u32, pub u32);", True),
        ("pub trait StableHash {", False),
        ("pub trait StableOrd: Ord {", False),
        ("pub trait StableCompare {", False),
        ("impl<T: StableOrd> StableCompare for T {", False),
        ("macro_rules! impl_stable_traits_for_trivial_type {", False),
        ("#[derive(Clone, Copy, Hash, Eq, PartialEq, Debug)]\npub struct StableHashControls {", False),
    ):
        start, end, text = (SUPPORT.line if simple else SUPPORT.block)(source, marker)
        stable.append(text)
        fragments.append({"source": "compiler/rustc_data_structures/src/stable_hash.rs", "marker": marker, "start_line": source.count("\n", 0, start) + 1, "end_line": source.count("\n", 0, end) + 1, "sha256": hashlib.sha256(text.encode()).hexdigest()})
    stable.append("pub(crate) use impl_stable_traits_for_trivial_type;")
    SUPPORT.write(destination / "wrapper/stable_hash.rs", ("\n\n".join(stable) + "\n").encode())
    SUPPORT.write(destination / "source-provenance.json", (json.dumps({"rust_revision": PIN, "sources": provenance, "stable_hash_fragments": fragments, "scope": "Unchanged complete fingerprint/test/unhash modules; unchanged complete rustc_serialize/rustc_hashes crates; exact relevant stable-hash trait/macro fragments. No full stable_hash.rs or unhash container port credit."}, indent=2) + "\n").encode())
    lock = HERE / "Cargo.lock"
    SUPPORT.write(destination / "Cargo.lock", lock.read_bytes() if lock.exists() else git("show", f"{PIN}:Cargo.lock"))
    tail = ["--manifest-path", str(destination / "Cargo.toml")]
    if args.offline:
        tail.append("--offline")
    if args.fetch or args.write_lock:
        subprocess.run([cargo, "fetch", *tail, *(["--locked"] if lock.exists() else [])], env=environment, check=True)
    if args.write_lock:
        SUPPORT.write(lock, (destination / "Cargo.lock").read_bytes())
    pinned = {(p["name"], p["version"]): p.get("checksum") for p in tomllib.loads(git("show", f"{PIN}:Cargo.lock").decode())["package"] if "source" in p}

    def registry():
        entries = []
        for package in tomllib.loads((destination / "Cargo.lock").read_text())["package"]:
            if "source" not in package:
                continue
            if pinned.get((package["name"], package["version"])) != package.get("checksum"):
                raise SystemExit("dependency differs from pinned root lock: " + package["name"])
            matches = list((Path(environment["CARGO_HOME"]) / "registry/cache").glob(f"*/{package['name']}-{package['version']}.crate"))
            if len(matches) != 1 or matches[0].is_symlink() or hashlib.sha256(matches[0].read_bytes()).hexdigest() != package["checksum"]:
                raise SystemExit("registry archive differs: " + package["name"])
            entry = {"name": package["name"], "version": package["version"], "sha256": package["checksum"]}
            entry.update(SUPPORT.verify_registry_source(matches[0], Path(environment["CARGO_HOME"]), package["name"], package["version"]))
            entries.append(entry)
        return entries

    dependencies = registry()
    artifacts = {}
    for name, extra in (("original-tests", ["test", "--lib", "--", "--test-threads=1"]), ("artifacts", ["build", "--lib", "--message-format=json"])):
        command = [cargo, extra[0], *tail, "--locked", *extra[1:]]
        result = subprocess.run(command, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        SUPPORT.write(destination / "logs" / (name + ".stdout"), result.stdout)
        SUPPORT.write(destination / "logs" / (name + ".stderr"), result.stderr)
        if result.returncode:
            raise SystemExit(result.stderr.decode(errors="replace"))
        if name == "artifacts":
            for line in result.stdout.splitlines():
                event = json.loads(line)
                if event.get("reason") == "compiler-artifact":
                    artifacts[event["target"]["name"]] = event["filenames"]
        else:
            print(result.stdout.decode(), end="")
    if registry() != dependencies:
        raise SystemExit("registry source changed during build")
    library = next(p for p in artifacts["fingerprint_reference"] if p.endswith(".rlib"))
    profile = {"rust_revision": PIN, "pinned_source_version": git("show", f"{PIN}:src/version").decode().strip(), "compiler": subprocess.check_output([environment["RUSTC"], "-vV"], text=True).strip(), "development_only": True, "rustc_bootstrap": "1", "rustflags": environment["RUSTFLAGS"], "library": library, "link_arguments": ["--extern", "fingerprint_reference=" + library, "-L", "dependency=" + str(destination / "target/debug/deps")], "registry_dependencies": dependencies, "source_provenance_sha256": hashlib.sha256((destination / "source-provenance.json").read_bytes()).hexdigest(), "helper_dependencies": [{"path": str(p.relative_to(root)), "sha256": hashlib.sha256(p.read_bytes()).hexdigest()} for p in [SHARED / "prepare.py", *[SHARED / f"{crate}.Cargo.toml" for crate in CRATES]]]}
    SUPPORT.write(destination / "build-profile.json", (json.dumps(profile, indent=2) + "\n").encode())
    print(destination)


if __name__ == "__main__":
    main()
