#!/usr/bin/env python3
"""Pinned, file-by-file upstream port provenance. Uses Python's standard library.

This tool checks structural evidence. It cannot certify semantic equivalence or
turn an existing bootstrap implementation into a port of an upstream file.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import sys
import tempfile

SCHEMA = 1
UPSTREAMS = ("rust", "cargo")
STATUSES = ("unported", "ported", "validated")
DATA = "tools/port-inventory/data"
PIN_FILE = "docs/upstream-inventory.json"
STATUS_FILE = "port-status.json"
PRESERVED = ("status", "port_evidence", "validation_evidence", "notes")


class InventoryError(Exception):
    pass


def git(cache, *args):
    environment = dict(os.environ, GIT_NO_LAZY_FETCH="1")
    result = subprocess.run(["git", "-C", str(cache), *args], stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=120, env=environment)
    if result.returncode:
        raise InventoryError("git {}: {}".format(" ".join(args), result.stderr.decode("utf-8", "replace").strip()))
    return result.stdout


def read_json(path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise InventoryError("{}: {}".format(path, error)) from error


def atomic_write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=path.parent, prefix=".inventory-", delete=False) as output:
            temporary = Path(output.name)
            output.write(data)
        temporary.chmod(0o644)
        os.replace(temporary, path)
    finally:
        if temporary is not None and temporary.exists():
            temporary.unlink()


def json_bytes(value):
    return (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def safe_relative(name):
    path = PurePosixPath(name)
    if not name or path.is_absolute() or any(part in ("", ".", "..") for part in name.split("/")):
        raise InventoryError("unsafe inventory path {!r}".format(name))
    return path


def local_file(root, relative):
    safe_relative(relative)
    path = root / relative
    try:
        path.resolve().relative_to(root.resolve())
    except ValueError as error:
        raise InventoryError("path escapes repository: {}".format(relative)) from error
    return path


def normalized_repository(url):
    value = url.rstrip("/")
    return value[:-4] if value.endswith(".git") else value


def read_pins(root):
    pins = read_json(root / PIN_FILE)
    for name in UPSTREAMS:
        pin = pins[name]
        revision = pin["revision"]
        if not isinstance(revision, str) or len(revision) not in (40, 64) or any(c not in "0123456789abcdef" for c in revision):
            raise InventoryError("{} revision must be a complete Git object SHA".format(name))
        if not isinstance(pin["repository"], str) or not pin["repository"]:
            raise InventoryError("{} repository URL is missing".format(name))
    return pins


def prepare(root):
    """Fetch only pinned Git trees and initial reference files; never push."""
    pins = read_pins(root)
    for name in UPSTREAMS:
        cache = root / ".toolchain" / ("upstream-" + name)
        if not (cache / ".git").is_dir():
            if cache.exists() and any(cache.iterdir()):
                raise InventoryError("refusing nonempty source-cache directory {}".format(cache))
            cache.mkdir(parents=True, exist_ok=True)
            subprocess.run(["git", "init", "-b", "main", str(cache)], check=True)
            origin = pins[name]["repository"].rstrip("/")
            if not origin.endswith(".git"):
                origin += ".git"
            git(cache, "remote", "add", "origin", origin)
        else:
            if git(cache, "status", "--porcelain", "--untracked-files=no"):
                raise InventoryError("refusing to change modified source cache {}".format(cache))
            origin = git(cache, "remote", "get-url", "origin").decode().strip()
            if normalized_repository(origin) != normalized_repository(pins[name]["repository"]):
                raise InventoryError("source-cache origin mismatch for {}".format(name))
        commands = [
            ["git", "-C", str(cache), "fetch", "--no-tags", "--depth=1", "--filter=blob:none", "origin", pins[name]["revision"]],
            ["git", "-C", str(cache), "sparse-checkout", "init", "--cone"],
            ["git", "-C", str(cache), "sparse-checkout", "set"] + (["compiler/rustc_lexer", "compiler/rustc_data_structures", "compiler/rustc_index", "library/core/src/unicode", "LICENSES"] if name == "rust" else ["src", "crates"]),
            ["git", "-C", str(cache), "checkout", "-B", "main", "FETCH_HEAD"],
        ]
        for command in commands:
            subprocess.run(command, check=True)
        # Cache worktrees are references, never an editing workspace. Preserve
        # executable bits; only rw permissions change, which Git does not track.
        for directory, children, files in os.walk(cache):
            children[:] = [child for child in children if child != ".git"]
            for filename in files:
                path = Path(directory) / filename
                if not path.is_symlink():
                    path.chmod(path.stat().st_mode & ~0o222)
        pinned_cache(root, name, pins)


def pinned_cache(root, name, pins):
    pin = pins[name]
    cache = root / ".toolchain" / ("upstream-" + name)
    head = git(cache, "rev-parse", "HEAD").decode().strip()
    if head != pin["revision"]:
        raise InventoryError("{} cache is at {}, expected {}".format(name, head, pin["revision"]))
    branch = git(cache, "branch", "--show-current").decode().strip()
    if branch != "main":
        raise InventoryError("{} source cache must stay on main, found {!r}".format(name, branch))
    origin = git(cache, "remote", "get-url", "origin").decode().strip()
    if normalized_repository(origin) != normalized_repository(pin["repository"]):
        raise InventoryError("{} cache origin does not match pinned repository".format(name))
    if git(cache, "status", "--porcelain", "--untracked-files=no"):
        raise InventoryError("{} source cache has modified tracked files".format(name))
    return cache


def component(upstream, path):
    if upstream == "cargo":
        return "cargo"
    first = path.split("/", 1)[0]
    return {"compiler": "compiler", "library": "standard_library", "tests": "tests"}.get(first, "tooling" if path.startswith("src/tools/") else "bootstrap_and_support")


def source_role(upstream, path):
    parts = PurePosixPath(path).parts
    base = parts[-1]
    # Rust compiler/tool test subjects must stay Rust: translating their
    # language programs to Go would stop testing Rust semantics entirely.
    fixture_parts = {"test_data", "test-data", "testdata", "fixtures", "fixture", "testcases", "test-auxiliary", "test-auxillary"}
    fixture = any(part in fixture_parts or part.endswith("_fixtures") for part in parts)
    if upstream == "rust":
        fixture = fixture or (path.startswith("tests/") and base != "rmake.rs")
        fixture = fixture or (path.startswith("src/tools/clippy/tests/") and any(part.startswith("ui") or part == "example_integration_test" for part in parts))
        fixture = fixture or (path.startswith("src/tools/miri/tests/") and len(parts) > 5)
        fixture = fixture or (path.startswith("src/tools/rustfmt/tests/") and any(part in ("source", "target") for part in parts))
        if base == "runner.rs" and "generated" in parts:
            return "test_harness"
    else:
        fixture = fixture or any(part in ("in", "out", "mock-std") or part.endswith((".in", ".out")) for part in parts)
        fixture = fixture or (path.startswith("tests/") and "src" in parts[1:])
        fixture = fixture or path.startswith("crates/cargo-test-support/containers/")
    if fixture:
        return "rust_fixture"
    if base == "build.rs":
        return "build_script"
    if "tests" in parts or "test" in parts or base in ("tests.rs", "test.rs") or base.endswith("_test.rs"):
        return "test_harness"
    if "examples" in parts or "example" in parts:
        return "example"
    return "implementation"


def support_role(path):
    base = PurePosixPath(path).name
    suffix = PurePosixPath(path).suffix.lower()
    if base.startswith(("LICENSE", "COPYRIGHT", "NOTICE")) or path.startswith("LICENSES/"):
        return "license"
    if base in ("Cargo.toml", "Cargo.lock", "go.mod", "go.sum", "rust-toolchain", "rust-toolchain.toml"):
        return "build_manifest"
    if suffix in (".py", ".sh", ".bash", ".ps1", ".bat", ".cmake") or base in ("Makefile", "CMakeLists.txt", "configure"):
        return "build_script"
    if suffix in (".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".s", ".ll"):
        return "native_support"
    if suffix in (".toml", ".json", ".yaml", ".yml", ".ini", ".cfg"):
        return "configuration"
    if suffix in (".md", ".rst", ".txt", ".html", ".css"):
        return "documentation_or_fixture"
    return "support_or_fixture"


def tree_rows(root, name, pins):
    cache = pinned_cache(root, name, pins)
    revision = pins[name]["revision"]
    # No -l: asking for blob sizes would hydrate every promised blob. Tree IDs
    # and filenames cover the full repository even with a sparse checkout.
    entries = git(cache, "ls-tree", "-r", "-z", revision)
    rows = {"sources": [], "support": [], "submodules": []}
    for entry in entries.split(b"\0"):
        if not entry:
            continue
        metadata, raw_path = entry.split(b"\t", 1)
        mode, kind, object_sha = metadata.decode("ascii").split()
        path = raw_path.decode("utf-8")
        safe_relative(path)
        if kind == "commit":
            category, role, target = "submodules", "gitlink", "port/{}/{}".format(name, path)
        elif kind == "blob" and path.endswith(".rs"):
            category, role = "sources", source_role(name, path)
            basename = path[:-3]
            if role == "rust_fixture":
                target = "port/{}/{}".format(name, path)
            else:
                if role == "test_harness" and not basename.endswith("_test"):
                    basename += "_test"
                target = "port/{}/{}.go".format(name, basename)
        elif kind == "blob":
            category, role, target = "support", support_role(path), "port/{}/{}".format(name, path)
        else:
            raise InventoryError("unsupported tree entry {} {}".format(kind, path))
        rows[category].append({
            "upstream": name, "source_path": path, "source_sha": object_sha,
            "source_mode": mode, "target_path": target, "component": component(name, path),
            "role": role, "status": "unported", "ported": False, "validated": False,
            "port_evidence": [], "validation_evidence": [], "notes": "",
        })
    for values in rows.values():
        values.sort(key=lambda item: item["source_path"])
    return cache, rows


def inventory_path(name, category):
    return "{}/{}-{}.jsonl".format(DATA, name, category)


def read_rows(path, optional=False):
    if optional and not path.exists():
        return []
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
        result = []
        for line_number, line in enumerate(lines, 1):
            try:
                result.append(json.loads(line))
            except ValueError as error:
                raise InventoryError("{}:{}: {}".format(path, line_number, error)) from error
        return result
    except OSError as error:
        raise InventoryError("{}: {}".format(path, error)) from error


def rows_bytes(rows):
    return b"".join((json.dumps(row, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8") for row in rows)


def update_flags(row):
    status = row["status"]
    if status not in STATUSES:
        raise InventoryError("invalid status {!r} for {}".format(status, row["source_path"]))
    row["ported"] = status in ("ported", "validated")
    row["validated"] = status == "validated"


def validate_progress(root, row):
    status = row.get("status")
    if status not in STATUSES:
        raise InventoryError("invalid status for {}".format(row.get("source_path")))
    if row.get("ported") != (status in ("ported", "validated")) or row.get("validated") != (status == "validated"):
        raise InventoryError("status flags disagree for {}".format(row["source_path"]))
    for field in ("port_evidence", "validation_evidence"):
        evidence = row.get(field)
        if not isinstance(evidence, list) or any(not isinstance(value, str) for value in evidence):
            raise InventoryError("invalid {} for {}".format(field, row["source_path"]))
        for relative in evidence:
            if not local_file(root, relative).is_file():
                raise InventoryError("missing {}: {}".format(field, relative))
    if row["role"] == "gitlink" and status != "unported":
        raise InventoryError("gitlink remains unexpanded; child inventory or explicit external-dependency policy required: {}".format(row["source_path"]))
    if status in ("ported", "validated"):
        target = local_file(root, row["target_path"])
        copied_fixture = row["role"] == "rust_fixture"
        target_exists = target.is_dir() if row["role"] == "gitlink" else target.is_file() and (copied_fixture or target.stat().st_size > 0)
        if not target_exists:
            raise InventoryError("missing or empty port destination: {}".format(row["target_path"]))
        if not row["port_evidence"]:
            raise InventoryError("ported file needs port_evidence: {}".format(row["source_path"]))
        if copied_fixture:
            payload = target.read_bytes()
            blob = b"blob " + str(len(payload)).encode("ascii") + b"\0" + payload
            algorithm = hashlib.sha1 if len(row["source_sha"]) == 40 else hashlib.sha256
            if algorithm(blob).hexdigest() != row["source_sha"]:
                raise InventoryError("retained Rust fixture differs from pinned source: {}".format(row["target_path"]))
            if not any(path.endswith(".go") and local_file(root, path).stat().st_size > 0 for path in row["port_evidence"]):
                raise InventoryError("retained Rust fixture needs Go runner port_evidence: {}".format(row["source_path"]))
    if status == "validated" and not row["validation_evidence"]:
        raise InventoryError("validated file needs validation_evidence: {}".format(row["source_path"]))


def merge_progress(root, expected, old_rows, reset_changed):
    previous = {}
    for row in old_rows:
        path = row["source_path"]
        if path in previous:
            raise InventoryError("duplicate source row {}".format(path))
        previous[path] = row
    for row in expected:
        old = previous.pop(row["source_path"], None)
        if old is None:
            continue
        changed = any(old.get(field) != row[field] for field in ("source_sha", "source_mode", "target_path"))
        if changed:
            if old.get("status") != "unported" and not reset_changed:
                raise InventoryError("ported source changed: {}; inspect it before --reset-changed".format(row["source_path"]))
            if old.get("status") != "unported":
                row["notes"] = "Port status reset after upstream source or mapping changed. Previous blob: " + old.get("source_sha", "unknown")
            continue
        for field in PRESERVED:
            row[field] = old.get(field, row[field])
        update_flags(row)
        validate_progress(root, row)
    removed_ports = [row["source_path"] for row in previous.values() if row.get("status") != "unported"]
    if removed_ports and not reset_changed:
        raise InventoryError("ported sources removed: {}".format(", ".join(removed_ports[:5])))


def describe(root, pins, generated):
    upstreams = []
    total = {"sources": 0, "support": 0, "submodules": 0, "statuses": {status: 0 for status in STATUSES}}
    for name in UPSTREAMS:
        cache, categories = generated[name]
        files = {}
        statuses = {status: 0 for status in STATUSES}
        for category, rows in categories.items():
            encoded = rows_bytes(rows)
            category_statuses = {status: sum(row["status"] == status for row in rows) for status in STATUSES}
            roles = {}
            for row in rows:
                roles[row["role"]] = roles.get(row["role"], 0) + 1
            files[category] = {"path": inventory_path(name, category), "entries": len(rows), "sha256": sha256(encoded), "statuses": category_statuses, "roles": dict(sorted(roles.items()))}
            total[category] += len(rows)
            for row in rows:
                statuses[row["status"]] += 1
                total["statuses"][row["status"]] += 1
        upstreams.append({"name": name, "repository": pins[name]["repository"], "revision": pins[name]["revision"],
                          "tree_sha": git(cache, "rev-parse", pins[name]["revision"] + "^{tree}").decode().strip(),
                          "cache": ".toolchain/upstream-" + name, "inventories": files, "statuses": statuses})
    return {"schema_version": SCHEMA, "goal": "Complete file-to-file Go ports of pinned rustc and Cargo with production Rust compatibility",
            "pins": PIN_FILE, "inventories": upstreams, "totals": total,
            "mapping": "Implementation .rs files mirror to .go; test harnesses to _test.go; Rust input subjects retain .rs; support files retain their source path and suffix",
            "fixture_accounting": "rust_fixture rows preserve original Rust bytes and need Go runner evidence; they are not Go-translated implementation files",
            "classification": "Deterministic path-based roles; review each source role and its runner obligations before recording completion",
            "status_meaning": {"unported": "No completed upstream-file port is recorded", "ported": "Completed file port is asserted with review evidence; conformance validation remains pending", "validated": "Port and pinned-upstream validation evidence are recorded"},
            "bootstrap_accounting": "Existing bootstrap compiler/Cargo code does not count as ported upstream files",
            "submodule_accounting": "Gitlinks are listed separately and unexpanded; their internal files are not included in top-level repository inventories",
            "production_gate": "docs/PORTING.md"}


def generate(root, reset_changed=False):
    pins = read_pins(root)
    generated = {}
    targets = {}
    for name in UPSTREAMS:
        cache, categories = tree_rows(root, name, pins)
        for category, rows in categories.items():
            merge_progress(root, rows, read_rows(root / inventory_path(name, category), optional=True), reset_changed)
            for row in rows:
                target = row["target_path"]
                if target in targets:
                    raise InventoryError("target mapping collision: {} and {}".format(targets[target], row["source_path"]))
                targets[target] = row["source_path"]
        generated[name] = (cache, categories)
    # Calculate and validate everything before replacing any generated file.
    manifest = describe(root, pins, generated)
    for name, (_, categories) in generated.items():
        for category, rows in categories.items():
            atomic_write(root / inventory_path(name, category), rows_bytes(rows))
    atomic_write(root / STATUS_FILE, json_bytes(manifest))
    return manifest


def verify(root):
    pins = read_pins(root)
    manifest = read_json(root / STATUS_FILE)
    generated = {}
    targets = {}
    for name in UPSTREAMS:
        cache, categories = tree_rows(root, name, pins)
        for category, expected in categories.items():
            actual = read_rows(root / inventory_path(name, category))
            if len(actual) != len(expected):
                raise InventoryError("{} {} file count changed; regenerate inventory".format(name, category))
            for found, wanted in zip(actual, expected):
                for field in wanted:
                    if field in PRESERVED or field in ("ported", "validated"):
                        continue
                    if found.get(field) != wanted[field]:
                        raise InventoryError("{} {} mismatch in {}: {}".format(name, category, wanted["source_path"], field))
                validate_progress(root, found)
                if found["target_path"] in targets:
                    raise InventoryError("duplicate target {}".format(found["target_path"]))
                targets[found["target_path"]] = True
            categories[category] = actual
        generated[name] = (cache, categories)
    expected_manifest = describe(root, pins, generated)
    if manifest != expected_manifest:
        raise InventoryError("port-status.json totals, pins, or inventory hashes disagree; regenerate after reviewing changes")
    return manifest


def set_status(root, upstream, source, status, port_evidence, validation_evidence, note):
    verify(root)
    path = root / inventory_path(upstream, "sources" if source.endswith(".rs") else "support")
    # Gitlinks have their own inventory and are selected by exact source path.
    candidates = [path, root / inventory_path(upstream, "submodules")]
    selected = None
    rows = None
    for candidate in candidates:
        data = read_rows(candidate)
        for row in data:
            if row["source_path"] == source:
                if selected is not None:
                    raise InventoryError("ambiguous source row {}".format(source))
                selected, rows, selected_row = candidate, data, row
    if selected is None:
        raise InventoryError("unknown {} source path {}".format(upstream, source))
    selected_row["status"] = status
    if port_evidence:
        selected_row["port_evidence"] = sorted(set(port_evidence))
    if validation_evidence:
        selected_row["validation_evidence"] = sorted(set(validation_evidence))
    if note is not None:
        selected_row["notes"] = note
    update_flags(selected_row)
    validate_progress(root, selected_row)
    atomic_write(selected, rows_bytes(rows))
    return generate(root)


def summary(manifest):
    counts = manifest["totals"]
    return "{} Rust source files, {} support/build files, {} gitlinks; status rows {}".format(
        counts["sources"], counts["support"], counts["submodules"], json.dumps(counts["statuses"], sort_keys=True))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("prepare", help="fetch pinned sparse source caches; requires network access, never pushes")
    command = commands.add_parser("generate", help="regenerate deterministic inventories from pinned Git trees")
    command.add_argument("--reset-changed", action="store_true", help="explicitly clear progress on changed/removed upstream files")
    commands.add_parser("verify", help="verify exact tree coverage, mapping, status evidence, destinations and hashes")
    commands.add_parser("summary", help="print recorded file/status counts without computing percentages")
    command = commands.add_parser("set-status", help="record an explicit completed-file port or validation assertion")
    command.add_argument("upstream", choices=UPSTREAMS)
    command.add_argument("source_path")
    command.add_argument("status", choices=STATUSES)
    command.add_argument("--port-evidence", action="append", default=[])
    command.add_argument("--validation-evidence", action="append", default=[])
    command.add_argument("--note")
    args = parser.parse_args(argv)
    try:
        root = args.root.resolve()
        if args.command == "prepare":
            prepare(root)
            print("Pinned upstream source caches are ready on main; worktree files are read-only")
            return 0
        if args.command == "generate":
            manifest = generate(root, args.reset_changed)
        elif args.command == "verify":
            manifest = verify(root)
        elif args.command == "summary":
            manifest = read_json(root / STATUS_FILE)
        else:
            manifest = set_status(root, args.upstream, args.source_path, args.status,
                                  args.port_evidence, args.validation_evidence, args.note)
        print(summary(manifest))
        return 0
    except (InventoryError, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired, subprocess.CalledProcessError) as error:
        print("error: {}".format(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
