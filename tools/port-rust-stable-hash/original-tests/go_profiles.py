#!/usr/bin/env python3
"""Run source-mirrored Go tests with every feature/arithmetic profile."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--goarch")
    parser.add_argument("--compile-only", action="store_true")
    args = parser.parse_args()
    root = args.root.resolve()
    go = os.environ.get("FERRUGO_GO") or str(root / ".toolchain/go/bin/go")
    go = os.path.abspath(shutil.which(go) or go)
    environment = os.environ.copy()
    environment.setdefault("GOCACHE", "/tmp/ferrugo-go-cache")
    environment.pop("FERRUGO_RUN_STABLE_HASH_ORIGINAL_TESTS", None)
    if args.goarch:
        environment.update(GOARCH=args.goarch, CGO_ENABLED="0")
    native_arch = subprocess.check_output([go, "env", "GOARCH"], env=environment, text=True).strip()
    expected = 15 if native_arch in ("amd64", "386", "arm") else 14
    cache = root / ".toolchain/oracles/stable-hash-original-tests"
    cache.mkdir(parents=True, exist_ok=True)
    profiles = []
    for nightly in (False, True):
        for checked, debug in (((None, None),) if args.compile_only else ((False, False), (False, True), (True, False), (True, True))):
            current = environment.copy()
            name = f"{native_arch}-{'nightly' if nightly else 'default'}"
            command = [go, "test", "-count=1"]
            if nightly:
                command += ["-tags", "rustc_stable_hash_nightly"]
            if args.compile_only:
                binary = cache / (name + ".test")
                command += ["-c", "-o", str(binary)]
            else:
                current["FERRUGO_STABLE_HASH_TEST_CHECKED"] = str(checked).lower()
                current["FERRUGO_STABLE_HASH_TEST_DEBUG"] = str(debug).lower()
                name += f"-overflow-{str(checked).lower()}-debug-{str(debug).lower()}"
                command.append("-json")
            command.append("./port/dependencies/rustc-stable-hash/v0_1_2/tests")
            result = subprocess.run(command, cwd=root, env=current, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            if result.returncode:
                raise SystemExit(result.stdout.decode(errors="replace") + result.stderr.decode(errors="replace"))
            unit_names, doc_names = [], []
            if not args.compile_only:
                for line in result.stdout.splitlines():
                    event = json.loads(line)
                    if event.get("Action") == "pass":
                        test = event.get("Test", "")
                        if test.startswith(("TestSip128_", "TestStableHasher_")):
                            unit_names.append(test)
                        elif test.startswith("TestDoc_"):
                            doc_names.append(test)
                if len(unit_names) != expected or len(doc_names) != 3:
                    raise SystemExit(f"Go {name} ran {len(unit_names)} original units and {len(doc_names)} docs")
            profiles.append({"name": name, "goarch": native_arch, "nightly": nightly, "overflow_checks": checked, "debug_assertions": debug, "compile_only": args.compile_only, "unit_tests_passed": len(unit_names), "unit_test_names": unit_names, "doc_examples_passed": len(doc_names), "doc_example_names": doc_names, "stdout_sha256": hashlib.sha256(result.stdout).hexdigest(), "stderr_sha256": hashlib.sha256(result.stderr).hexdigest()})
            print(name + (": compiled; runtime unvalidated" if args.compile_only else f": {expected} original units and 3 doc mappings passed"), flush=True)
    evidence = {"go": subprocess.check_output([go, "version"], text=True).strip(), "host_machine": platform.machine(), "scope": "Compile-only targets do not execute the binary or arithmetic profiles; each feature selects the complete target-specific source mappings.", "profiles": profiles}
    (cache / f"go-evidence-{native_arch}.json").write_text(json.dumps(evidence, indent=2) + "\n")


if __name__ == "__main__":
    main()
