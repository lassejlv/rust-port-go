#!/usr/bin/env python3
"""Build unchanged edition derives with real macro, codec, and hash algorithms."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import stat
import tarfile
import tomllib
from pathlib import PurePosixPath

PIN = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
ACTUAL_CRATES = ("rustc_macros", "rustc_serialize", "rustc_hashes")
CONTRACT_PATHS = (
    "compiler/rustc_span/src/lib.rs",
    "compiler/rustc_span/src/edition.rs",
    "compiler/rustc_data_structures/src/stable_hash.rs",
)
GIT_ENV = os.environ.copy()


def git_bytes(upstream, *arguments):
    return subprocess.check_output(["git", "-C", str(upstream), *arguments], env=GIT_ENV)


def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists() or path.read_bytes() != data:
        path.write_bytes(data)


def block(source, marker):
    start = source.index(marker)
    opening = source.index("{", start)
    depth, i = 0, opening
    while i < len(source):
        if source.startswith("//", i):
            end = source.find("\n", i)
            i = len(source) if end < 0 else end + 1
            continue
        if source.startswith("/*", i):
            end = source.index("*/", i + 2)
            i = end + 2
            continue
        if source[i] == '"':
            i += 1
            while i < len(source):
                if source[i] == "\\":
                    i += 2
                elif source[i] == '"':
                    i += 1
                    break
                else:
                    i += 1
            continue
        if source[i] == "{":
            depth += 1
        elif source[i] == "}":
            depth -= 1
            if depth == 0:
                return start, i + 1, source[start:i + 1]
        i += 1
    raise ValueError(f"unterminated original source block {marker}")


def line(source, marker):
    start = source.index(marker)
    end = source.index(";", start) + 1
    return start, end, source[start:end]


def contracts(destination, originals):
    fragments = []

    def take(path, marker, simple=False):
        original = originals[path].decode()
        start, end, text = (line if simple else block)(original, marker)
        fragments.append({"path": path, "marker": marker, "start_line": original.count("\n", 0, start) + 1, "end_line": original.count("\n", 0, end) + 1, "sha256": hashlib.sha256(text.encode()).hexdigest(), "bytes": len(text.encode())})
        return text

    path = "compiler/rustc_data_structures/src/stable_hash.rs"
    stable = ["use crate::fingerprint::Fingerprint;", "pub use rustc_stable_hash::{FromStableHash, SipHasher128Hash as StableHasherHash, StableSipHasher128 as StableHasher};"]
    for marker, simple in (
        ("pub trait StableHashCtxt {", False),
        ("pub struct DummyStableHashCtxt;", True),
        ("impl StableHashCtxt for DummyStableHashCtxt {", False),
        ("pub struct RawSpan(pub u64);", True),
        ("pub struct RawDefId(pub u32, pub u32);", True),
        ("pub trait StableHash {", False),
        ("pub trait StableOrd: Ord {", False),
        ("macro_rules! impl_stable_traits_for_trivial_type {", False),
        ("impl_stable_traits_for_trivial_type!(u8);", True),
        ("#[derive(Clone, Copy, Hash, Eq, PartialEq, Debug)]\npub struct StableHashControls {", False),
    ):
        stable.append(take(path, marker, simple))
    write(destination / "crates/rustc_data_structures/src/stable_hash.rs", ("\n\n".join(stable) + "\n").encode())
    span = []
    for marker in ("pub trait SpanEncoder: Encoder {", "pub trait BlobDecoder: Decoder {"):
        span.append(take("compiler/rustc_span/src/lib.rs", marker))
    write(destination / "crates/rustc_span/src/codec_traits.rs", ("\n\n".join(span) + "\n").encode())
    return fragments


def prepare(root, destination):
    upstream = root / ".toolchain/upstream-rust"
    if git_bytes(upstream, "rev-parse", "HEAD").decode().strip() != PIN:
        raise SystemExit("source checkout does not match pinned Rust revision")
    paths = list(CONTRACT_PATHS)
    for crate in ACTUAL_CRATES:
        paths.extend(git_bytes(upstream, "ls-tree", "-r", "--name-only", PIN, f"compiler/{crate}").decode().splitlines())
    originals, provenance = {}, []
    for path in sorted(paths):
        blob = git_bytes(upstream, "rev-parse", f"{PIN}:{path}").decode().strip()
        data = git_bytes(upstream, "show", blob)
        if hashlib.sha1(f"blob {len(data)}\0".encode() + data).hexdigest() != blob:
            raise SystemExit(f"original source blob mismatch: {path}")
        originals[path] = data
        write(destination / "original" / path, data)
        for crate in ACTUAL_CRATES:
            prefix = f"compiler/{crate}/"
            if path.startswith(prefix) and path.removeprefix(prefix) != "Cargo.toml":
                write(destination / "crates" / crate / path.removeprefix(prefix), data)
        provenance.append({"path": path, "git_blob": blob, "sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)})
    for crate in (*ACTUAL_CRATES, "rustc_span", "rustc_data_structures"):
        write(destination / "crates" / crate / "Cargo.toml", (HERE / f"{crate}.Cargo.toml").read_bytes())
    write(destination / "Cargo.toml", (HERE / "Cargo.toml").read_bytes())
    for name in ("lib.rs", "main.rs"):
        write(destination / "wrapper" / name, (HERE / name).read_bytes())
    write(destination / "crates/rustc_data_structures/src/lib.rs", (HERE / "data_structures_lib.rs").read_bytes())
    write(destination / "crates/rustc_span/src/lib.rs", (HERE / "span_lib.rs").read_bytes())
    write(destination / "crates/rustc_span/src/audit.rs", (HERE / "audit.rs").read_bytes())
    write(destination / "crates/rustc_span/src/edition.rs", originals["compiler/rustc_span/src/edition.rs"])
    selected = contracts(destination, originals)
    lock = HERE / "Cargo.lock"
    if lock.exists():
        write(destination / "Cargo.lock", lock.read_bytes())
    elif not (destination / "Cargo.lock").exists():
        write(destination / "Cargo.lock", git_bytes(upstream, "show", f"{PIN}:Cargo.lock"))
    source_profile = {"rust_revision": PIN, "port_credit": False, "actual_complete_crates": list(ACTUAL_CRATES), "reference_only_facades": ["rustc_span", "rustc_data_structures"], "sources": provenance, "unchanged_selected_contracts": selected}
    write(destination / "source-provenance.json", (json.dumps(source_profile, indent=2) + "\n").encode())
    return upstream


def verify_registry_source(archive, cargo_home, name, version):
    prefix = f"{name}-{version}"
    source = cargo_home / "registry/src" / archive.parent.name / prefix
    for directory in (cargo_home / "registry", cargo_home / "registry/src", source.parent, source):
        if not directory.exists() or directory.is_symlink() or not stat.S_ISDIR(directory.lstat().st_mode):
            raise SystemExit(f"missing or non-directory registry source path: {directory}")
    expected, expected_directories = {}, set()
    with tarfile.open(archive, "r:gz") as package:
        for member in package:
            if member.isdir() and member.name.rstrip("/") == prefix:
                continue
            if not member.name.startswith(prefix + "/"):
                raise SystemExit(f"unexpected registry archive prefix: {member.name}")
            relative = member.name[len(prefix) + 1:].rstrip("/")
            path = PurePosixPath(relative)
            if not relative or path.is_absolute() or str(path) != relative or ".." in path.parts:
                raise SystemExit(f"unsafe registry archive member: {member.name}")
            if member.isdir():
                expected_directories.add(relative)
            elif member.isfile():
                if relative in expected:
                    raise SystemExit(f"duplicate registry archive file: {member.name}")
                expected[relative] = package.extractfile(member).read()
            else:
                raise SystemExit(f"non-regular registry archive member: {member.name}")
            for parent in path.parents:
                if str(parent) != ".":
                    expected_directories.add(str(parent))
    actual = set()
    for current, directories, files in os.walk(source, followlinks=False):
        current = Path(current)
        for directory in directories:
            child = current / directory
            relative = child.relative_to(source).as_posix()
            if child.is_symlink() or not stat.S_ISDIR(child.lstat().st_mode) or relative not in expected_directories:
                raise SystemExit(f"unexpected or symlink registry source directory: {child}")
        for filename in files:
            child = current / filename
            relative = child.relative_to(source).as_posix()
            if child.is_symlink() or not stat.S_ISREG(child.lstat().st_mode):
                raise SystemExit(f"non-regular registry source file: {child}")
            if relative == ".cargo-ok" and relative not in expected:
                if child.read_bytes() not in (b'{"v":1}', b"ok", b""):
                    raise SystemExit(f"unexpected Cargo extraction marker: {child}")
                continue
            if relative not in expected:
                raise SystemExit(f"unexpected registry source file: {child}")
            actual.add(relative)
            if child.read_bytes() != expected[relative]:
                raise SystemExit(f"registry source differs from verified archive: {child}")
    if actual != set(expected):
        raise SystemExit(f"registry source files missing for {name} {version}: {sorted(set(expected) - actual)}")
    tree = hashlib.sha256()
    for relative in sorted(expected):
        tree.update(relative.encode() + b"\0" + hashlib.sha256(expected[relative]).digest())
    return {"compiled_source_files": len(expected), "compiled_source_tree_sha256": tree.hexdigest()}


def registry_provenance(upstream, destination, cargo_home, require_archives):
    original = tomllib.loads(git_bytes(upstream, "show", f"{PIN}:Cargo.lock").decode())
    pinned = {(package["name"], package["version"]): package.get("checksum") for package in original["package"] if "source" in package}
    entries = []
    for package in tomllib.loads((destination / "Cargo.lock").read_text())["package"]:
        if "source" not in package:
            continue
        if pinned.get((package["name"], package["version"])) != package.get("checksum"):
            raise SystemExit(f"reference dependency differs from pinned lock: {package['name']} {package['version']}")
        archives = list((cargo_home / "registry/cache").glob(f"*/{package['name']}-{package['version']}.crate"))
        if require_archives:
            if len(archives) != 1 or archives[0].is_symlink() or not stat.S_ISREG(archives[0].lstat().st_mode) or hashlib.sha256(archives[0].read_bytes()).hexdigest() != package["checksum"]:
                raise SystemExit(f"missing or mismatched official registry archive: {package['name']} {package['version']}")
        entry = {"name": package["name"], "version": package["version"], "source": package["source"], "sha256": package["checksum"]}
        if require_archives:
            entry.update(verify_registry_source(archives[0], cargo_home, package["name"], package["version"]))
        entries.append(entry)
    return entries


def validate(upstream, destination, cargo, environment, offline):
    tail = ["--locked", "--manifest-path", str(destination / "Cargo.toml")]
    if offline:
        tail.append("--offline")
    logs = destination / "logs"
    logs.mkdir(exist_ok=True)
    commands = [
        ("proof", [cargo, "run", *tail, "--bin", "edition-reference-proof"]),
        ("expanded", [cargo, "rustc", *tail, "-p", "rustc_span", "--lib", "--", "-Zunpretty=expanded"]),
        ("artifacts", [cargo, "build", *tail, "--lib", "--message-format=json"]),
    ]
    runs, artifacts = [], {}
    for name, command in commands:
        result = subprocess.run(command, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        write(logs / f"{name}.stdout", result.stdout)
        write(logs / f"{name}.stderr", result.stderr)
        if result.returncode:
            raise SystemExit(f"edition reference {name} failed:\n{result.stderr.decode(errors='replace')}")
        if name == "expanded":
            write(destination / "expanded-edition.rs", result.stdout)
        if name == "proof":
            if b"all 5 editions and 251 invalid tags passed; unused interface calls=0" not in result.stdout:
                raise SystemExit("proof did not validate all used operations and unreachable interfaces")
            print(result.stdout.decode(), end="")
        if name == "artifacts":
            for line in result.stdout.splitlines():
                event = json.loads(line)
                if event.get("reason") == "compiler-artifact":
                    artifacts[event["target"]["name"]] = {"features": event["features"], "filenames": event["filenames"]}
        runs.append({"name": name, "command": command, "stdout_sha256": hashlib.sha256(result.stdout).hexdigest(), "stderr_sha256": hashlib.sha256(result.stderr).hexdigest(), "exit_code": result.returncode})
    profile = {
        "rust_revision": PIN,
        "pinned_source_version": git_bytes(upstream, "show", f"{PIN}:src/version").decode().strip(),
        "compiler": subprocess.check_output([environment["RUSTC"], "-vV"], text=True).strip(),
        "development_only": True,
        "rustc_bootstrap": environment["RUSTC_BOOTSTRAP"],
        "rustflags": environment["RUSTFLAGS"],
        "port_credit": False,
        "scope": "Unchanged Edition with all actual rustc_macros derives, complete actual serialization/hash crates and real StableSipHasher; original relevant traits and u8 StableHash implementation; opaque unused span/def-id/context interfaces panic and must have zero calls. Does not claim complete rustc_span/rustc_data_structures build or whole pinned-compiler ABI/layout validation.",
        "used_method_audit": {"encode": ["emit_u8"], "decode_valid_and_invalid": ["read_u8"], "std_hash": ["write_isize"], "stable_hash": ["u8 StableHash -> std Hash -> actual StableSipHasher"], "unused_interfaces_called": 0},
        "source_provenance_sha256": hashlib.sha256((destination / "source-provenance.json").read_bytes()).hexdigest(),
        "registry_dependencies": registry_provenance(upstream, destination, Path(environment["CARGO_HOME"]), True),
        "runs": runs,
        "artifacts": artifacts,
    }
    library = next((name for name in artifacts["edition_reference"]["filenames"] if name.endswith(".rlib")), None)
    if library is None:
        raise SystemExit("Cargo did not report the actual edition_reference library artifact")
    profile["library"] = library
    profile["link_arguments"] = ["--extern", f"edition_reference={library}", "-L", f"dependency={destination / 'target/debug/deps'}"]
    write(destination / "build-profile.json", (json.dumps(profile, indent=2) + "\n").encode())


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
    if arguments.offline:
        GIT_ENV["GIT_NO_LAZY_FETCH"] = "1"
    root = arguments.root.resolve()
    destination = (arguments.destination or root / ".toolchain/oracles/edition-reference").resolve()
    upstream = prepare(root, destination)
    cargo = arguments.cargo or str(root / ".toolchain/rust/bin/cargo")
    rustc = arguments.rustc or str(root / ".toolchain/rust/bin/rustc")
    environment = os.environ.copy()
    environment["CARGO_HOME"] = str(Path(environment.get("CARGO_HOME", root / ".toolchain/port-cargo-home")).resolve())
    environment["RUSTC"] = shutil.which(rustc) or str(Path(rustc).resolve())
    environment["RUSTC_BOOTSTRAP"] = "1"
    environment["RUSTFLAGS"] = "-Zcrate-attr=feature(never_type)"
    environment.pop("CARGO_ENCODED_RUSTFLAGS", None)
    if arguments.fetch:
        command = [cargo, "fetch", "--manifest-path", str(destination / "Cargo.toml")]
        if (HERE / "Cargo.lock").exists():
            command.append("--locked")
        if arguments.offline:
            command.append("--offline")
        subprocess.run(command, env=environment, check=True)
    registry_provenance(upstream, destination, Path(environment["CARGO_HOME"]), arguments.fetch or arguments.validate)
    if arguments.write_lock:
        write(HERE / "Cargo.lock", (destination / "Cargo.lock").read_bytes())
    if arguments.validate:
        validate(upstream, destination, cargo, environment, arguments.offline)
    print(destination)


if __name__ == "__main__":
    main()
