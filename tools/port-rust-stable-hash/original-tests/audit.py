#!/usr/bin/env python3
"""Audit original stable-hash declarations, literals, assertions and vectors."""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("stable_hash_original_run", HERE / "run.py")
RUN = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUN)


def mask(source):
    result = list(source)
    strings = []
    index = 0
    while index < len(source):
        start = index
        if source.startswith("//", index):
            index = source.find("\n", index)
            if index < 0:
                index = len(source)
        elif source.startswith("/*", index):
            depth = 1
            index += 2
            while depth:
                if source.startswith("/*", index):
                    depth += 1
                    index += 2
                elif source.startswith("*/", index):
                    depth -= 1
                    index += 2
                else:
                    index += 1
        elif source[index] == '"':
            index += 1
            value = ""
            while source[index] != '"':
                char = source[index]
                index += 1
                if char == "\\":
                    char = source[index]
                    index += 1
                    char = {"n": "\n", "r": "\r", "t": "\t", "0": "\0", "\\": "\\", '"': '"'}[char]
                value += char
            index += 1
            strings.append(value)
        else:
            index += 1
            continue
        for position in range(start, index):
            if result[position] != "\n":
                result[position] = " "
    return "".join(result), strings


def block_end(masked, opening):
    depth = 0
    for index in range(opening, len(masked)):
        if masked[index] == "{":
            depth += 1
        elif masked[index] == "}":
            depth -= 1
            if depth == 0:
                return index + 1
    raise SystemExit("unbalanced original Rust block")


NUMBER = re.compile(r"(?<!\w)(?:0[xX][0-9a-fA-F_]+|[0-9][0-9_]*)(?:_?[ui](?:128|64|32|16|8|size))?(?!\w)")


def source_stats(source):
    masked, strings = mask(source)
    numbers = []
    for match in NUMBER.finditer(masked):
        # out.0 is a member selector; range endpoints such as u[..1] are values.
        if match.start() and masked[match.start() - 1] == "." and not masked[:match.start()].endswith(".."):
            continue
        token = re.sub(r"_?[ui](?:128|64|32|16|8|size)$", "", match[0]).replace("_", "")
        value = int(token, 16 if token.startswith(("0x", "0X")) else 10)
        if masked[:match.start()].rstrip().endswith("-"):
            value = -value
        numbers.append(str(value))
    return {"integers": numbers, "strings": strings, "assertions": re.findall(r"\b(assert_eq|assert_ne|assert)!", masked)}


def combine_stats(functions):
    return {name: [value for function in functions for value in (function.get(name) or [])] for name in ("integers", "strings", "assertions")}



def in_order(expected, actual):
    remaining=iter(actual)
    return all(any(candidate==value for candidate in remaining) for value in expected)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=RUN.ROOT)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    root = args.root.resolve()
    originals = RUN.source_files(root)
    target = root / "port/dependencies/rustc-stable-hash/v0_1_2/tests"
    paths = sorted(target.glob("*_test.go"))
    go = os.environ.get("FERRUGO_GO") or str(root / ".toolchain/go/bin/go")
    go = os.path.abspath(shutil.which(go) or go)
    environment = os.environ.copy()
    environment.setdefault("GOCACHE", "/tmp/ferrugo-go-cache")
    go_files = json.loads(subprocess.check_output([go, "run", str(HERE / "audit.go"), "--", *map(str, paths)], env=environment))
    entries = []
    counts = Counter()
    for path, prefix in (("src/sip128/tests.rs", "Sip128"), ("src/stable_hasher/tests.rs", "StableHasher")):
        source = originals[path].decode()
        masked, _ = mask(source)
        ordered_go = []
        pattern = r'#\[test\]\s*(?:#\[cfg\(target_arch = "([^"]+)"\)\]\s*)?fn (\w+)\(\)\s*\{'
        for match in re.finditer(pattern, source):
            cfg, name = match[1], match[2]
            start = match.end() - 1
            end = block_end(masked, start)
            body = source[start + 1:end - 1]
            if cfg:
                arch = {"x86_64": "amd64", "x86": "386", "arm": "arm"}[cfg]
                go_name = "sip128_arch_" + arch + "_test.go"
                if not (target / go_name).read_text().startswith("//go:build " + arch + "\n"):
                    raise SystemExit("architecture gate differs: " + cfg)
            else:
                go_name = "sip128_test.go" if prefix == "Sip128" else "stable_hasher_test.go"
            functions = {item["name"]: item for item in go_files[str(target / go_name)]["functions"]}
            target_name = "Test" + prefix + "_" + name
            names = [target_name]
            if name == "test_hash_no_bytes_dropped_64":
                names.append("zero_byte64")
            elif name == "test_hash_no_bytes_dropped_32":
                names.append("zero_byte32")
            actual = combine_stats([functions[item] for item in names])
            expected = source_stats(body)
            if actual["assertions"] != expected["assertions"]:
                raise SystemExit("assertion declaration order differs: " + path + "::" + name)
            for kind in ("integers", "strings"):
                missing = Counter(expected[kind]) - Counter(actual[kind])
                if missing:
                    raise SystemExit(f"{kind} occurrences missing in {path}::{name}: {missing}")
                if not in_order(expected[kind],actual[kind]):
                    raise SystemExit(f"{kind} source order differs in {path}::{name}")
            counts["tests"] += 1
            counts["assertions"] += len(expected["assertions"])
            counts["strings"] += len(expected["strings"])
            counts["integers"] += len(expected["integers"])
            entries.append({"source": path, "source_line": source.count("\n", 0, match.start()) + 1, "name": name, "cfg_target_arch": cfg, "target": go_name, "go_functions": names, "assertion_order": expected["assertions"], "string_occurrences": len(expected["strings"]), "signed_integer_occurrences": len(expected["integers"])})
            if not cfg:
                ordered_go.append(target_name)
        main_name = "sip128_test.go" if prefix == "Sip128" else "stable_hasher_test.go"
        actual_order = [item["name"] for item in go_files[str(target / main_name)]["functions"] if item["name"].startswith("Test" + prefix + "_")]
        if actual_order != ordered_go:
            raise SystemExit("test declaration order differs: " + path)
    complete_counts = {}
    for source_name,go_names in (
        ("src/sip128/tests.rs",["sip128_test.go","sip128_vector_test.go","sip128_arch_arm_test.go","sip128_arch_amd64_test.go","sip128_arch_386_test.go","util_test.go"]),
        ("src/stable_hasher/tests.rs",["stable_hasher_test.go","util_test.go"]),
    ):
        expected = source_stats(originals[source_name].decode())
        if source_name.endswith("sip128/tests.rs"):
            expected["strings"] = [value for value in expected["strings"] if value not in ("arm","x86_64","x86")]
        actual = combine_stats([go_files[str(target/name)]["all"] for name in go_names])
        for kind in ("integers","strings"):
            missing = Counter(expected[kind])-Counter(actual[kind])
            if missing: raise SystemExit("complete source literal occurrences differ: "+source_name+" "+kind+" "+str(missing))
        complete_counts[source_name] = {kind:len(expected[kind]) for kind in ("integers","strings")}
    sip = originals["src/sip128/tests.rs"].decode()
    masked, _ = mask(sip)
    opening = masked.index("{", masked.index("macro_rules! test_fill_buffer"))
    expected = source_stats(sip[opening + 1:block_end(masked, opening) - 1])
    helper = next(item for item in go_files[str(target / "sip128_test.go")]["functions"] if item["name"] == "test_fill_buffer")
    if helper["assertions"] != expected["assertions"]:
        raise SystemExit("source fill-buffer macro assertion differs")
    for kind in ("integers", "strings"):
        if Counter(expected[kind]) - Counter(helper[kind] or []):
            raise SystemExit("source fill-buffer macro literal differs: " + kind)
        if not in_order(expected[kind],helper[kind] or []):
            raise SystemExit("source fill-buffer macro literal order differs: "+kind)
        counts[kind] += len(expected[kind])
    counts["assertions"] += len(expected["assertions"])
    source_types = re.findall(r"test_fill_buffer!\((\w+),\s*(\w+)\)", sip)
    expected_writes = ["Write" + method.removeprefix("write_").capitalize() for _, method in source_types]
    go_fill = next(item for item in go_files[str(target / "sip128_test.go")]["functions"] if item["name"] == "TestSip128_test_fill_buffer")
    if [name for name in go_fill["calls"] if name.startswith("Write")] != expected_writes:
        raise SystemExit("source fill-buffer type/write invocation order differs")
    vector_source = sip[sip.index("const TEST_VECTOR"):sip.index("\n];", sip.index("const TEST_VECTOR"))]
    expected_vector = [[str(int(token, 16)) for token in re.findall(r"0x[0-9a-f]+", row)] for row in vector_source.splitlines()[1:]]
    if len(expected_vector) != 64 or any(len(row) != 16 for row in expected_vector):
        raise SystemExit("original vector dimensions differ")
    if go_files[str(target / "sip128_vector_test.go")]["vector"] != expected_vector:
        raise SystemExit("original 64x16 vector values/order differ")
    doc_source = originals["src/stable_hasher.rs"].decode()
    docs = []
    fence = re.escape(chr(96) * 3)
    for index, match in enumerate(re.finditer(r"(?m)^/// " + fence + r"\n(.*?)^/// " + fence + r"\n", doc_source, re.S), 1):
        if (HERE / f"fixtures/doctests/stable_hasher_{index}.rs.txt").read_bytes() != match[0].encode():
            raise SystemExit("original doctest fixture differs")
        docs.append({"source": "src/stable_hasher.rs", "source_line": doc_source.count("\n", 0, match.start()) + 1, "sha256": hashlib.sha256(match[0].encode()).hexdigest(), "target": "doc_examples_test.go", "go_test": ["TestDoc_ExtendedHasher", "TestDoc_StableHasher", "TestDoc_FromStableHash"][index - 1], "scope": "interface compile witness" if index in (1, 3) else "typed finish runtime example"})
    doc_functions = [item["name"] for item in go_files[str(target / "doc_examples_test.go")]["functions"] if item["name"].startswith("TestDoc_")]
    if doc_functions != [doc["go_test"] for doc in docs] or len(docs) != 3:
        raise SystemExit("source doctest declaration order differs")
    for path in ("src/sip128/tests.rs", "src/stable_hasher/tests.rs"):
        if (HERE / "fixtures" / path).read_bytes() != originals[path]:
            raise SystemExit("retained original fixture differs")
    if counts["tests"] != 17 or counts["assertions"] != 38:
        raise SystemExit("source accounting differs: " + str(counts))
    metadata = {
        "crate": "rustc-stable-hash", "version": "0.1.2", "archive_sha256": RUN.CHECKSUM,
        "source_test_files": 2, "source_test_lines": 475, "test_declarations": counts["tests"],
        "native_x86_64_test_functions": 15, "assertion_sites_all_cfg": counts["assertions"],
        "native_x86_64_static_assertion_sites": 34, "runtime_string_occurrences": counts["strings"],
        "signed_integer_occurrences_excluding_vector": counts["integers"],
        "vector_rows": 64, "vector_bytes_per_row": 16, "vector_values": 1024,
        "fill_buffer_type_order": [kind for kind, _ in source_types],
        "complete_file_literal_counts": complete_counts,
        "files": [{"source": path, "sha256": hashlib.sha256(originals[path]).hexdigest()} for path in ("src/sip128/tests.rs", "src/stable_hasher/tests.rs")],
        "declarations": entries, "doc_examples": docs,
        "targets": [{"path": str(path.relative_to(root)), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()} for path in paths],
        "language_adaptations": [
            "Rust Hash inputs map to a closed test-input dispatcher; string sentinels/slice prefixes/bulk native i32 writes are verified by the retained compiler input-dispatch witness.",
            "Tuple field selectors are members rather than numeric values; projection maps to the named Go wrapper/array.",
            "u128 literals remain exact Go constants before low/high 64 decomposition; signed Int128 values are sign-extended.",
            "Nested zero_byte helpers move immediately after their tests while retaining assertion source order.",
            "Three duplicate architecture-gated declarations map to Go build-tag files; unexecuted targets have no runtime-validation claim.",
            "The fill macro retains all 12 integer kinds, every fill offset and the original type/write invocation order.",
            "Private pure TestHash conversion wraps FinishHash; public generic descriptors are compiled by the doc mappings.",
            "The doc upper-shift-by64 expression maps to the explicit UInt128 upper limb.",
            "Examples 1 and 3 remain interface compile witnesses without invented runtime assertions; Go diagnostics use testing instead of Rust's assertion runtime."
        ],
        "unit_literal_order": "Source literal value occurrences retain Go AST order, allowing additional representation syntax values; vector data has exact element order.",
        "scope": "Test/doc-source preservation only; no implementation-file or whole-project completion credit."
    }
    encoded = (json.dumps(metadata, indent=2) + "\n").encode()
    if args.check:
        if (HERE / "audit.json").read_bytes() != encoded:
            raise SystemExit("original stable-hash source audit differs")
    else:
        (HERE / "audit.json").write_bytes(encoded)
    print("17 declarations, 38 assertion sites, 1024 exact vector bytes, all unit literals/order and 3 doc examples accounted")


if __name__ == "__main__":
    main()
