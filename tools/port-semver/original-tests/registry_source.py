"""Verify registry compiler inputs against exact official crate archive bytes."""
import hashlib
import os
from pathlib import Path, PurePosixPath
import stat
import tarfile

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
