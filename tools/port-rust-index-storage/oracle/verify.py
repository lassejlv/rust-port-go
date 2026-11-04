#!/usr/bin/env python3
"""Verify the dependencies Cargo actually selected before compiling them."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tomllib

root = Path(sys.argv[1]).resolve()
cargo = sys.argv[2]
manifest = root / "tools/port-rust-index-storage/oracle/Cargo.toml"
pin = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
original = subprocess.check_output(["git", "-C", str(root / ".toolchain/upstream-rust"), "show", f"{pin}:Cargo.lock"])
locked = {(p["name"], p["version"]): p.get("checksum") for p in tomllib.loads(original.decode())["package"] if "source" in p}
selected = tomllib.loads(manifest.with_name("Cargo.lock").read_text())["package"]
checksums = {}
for package in selected:
    if "source" not in package:
        continue
    key = package["name"], package["version"]
    if locked.get(key) != package.get("checksum"):
        raise SystemExit(f"oracle dependency differs from pinned root lock: {key}")
    checksums[key] = package["checksum"]
metadata = json.loads(subprocess.check_output([cargo, "metadata", "--locked", "--offline", "--all-features", "--format-version=1", "--manifest-path", str(manifest)]))
home = Path(os.environ["CARGO_HOME"])
provenance = []
for package in metadata["packages"]:
    if not (package.get("source") or "").startswith("registry+"):
        continue
    name, version = package["name"], package["version"]
    key = name, version
    checksum = checksums[key]
    archives = list((home / "registry/cache").glob(f"*/{name}-{version}.crate"))
    if len(archives) != 1 or hashlib.sha256(archives[0].read_bytes()).hexdigest() != checksum:
        raise SystemExit(f"registry archive missing/ambiguous/modified: {name} {version}")
    source = Path(package["manifest_path"]).parent
    files = 0
    with tarfile.open(archives[0], "r:gz") as archive:
        prefix = f"{name}-{version}/"
        for member in archive.getmembers():
            if not member.isfile():
                continue
            if not member.name.startswith(prefix):
                raise SystemExit(f"unexpected registry archive member: {member.name}")
            relative = member.name.removeprefix(prefix)
            expected = archive.extractfile(member).read()
            path = source / relative
            if not path.is_file() or path.read_bytes() != expected:
                raise SystemExit(f"selected expanded registry source differs from archive: {name} {version}/{relative}")
            files += 1
    provenance.append({"name": name, "version": version, "archive_sha256": checksum, "verified_files": files})
print(json.dumps(sorted(provenance, key=lambda p: (p["name"], p["version"])), sort_keys=True, separators=(",", ":")))
