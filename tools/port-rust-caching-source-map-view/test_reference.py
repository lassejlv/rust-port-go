"""Isolated verified-reference regressions; no Rust or Go command is run."""
import importlib.util
import json
from pathlib import Path
from types import SimpleNamespace
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
PROFILES = ["d0o0", "d0o1", "d1o0", "d1o1"]


def module_at(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


def generators():
    # generate_context imports its sibling by the ordinary name "generate".
    # Bind that name to this fresh module only while loading the context client,
    # avoiding a collision with unrelated tool-directory tests in one process.
    previous = sys.path[:]
    try:
        regular = module_at("isolated_caching_generate", HERE / "generate.py")
        with mock.patch.dict(sys.modules, {"generate": regular}):
            contextual = module_at("isolated_caching_context", HERE / "generate_context.py")
        return regular, contextual
    finally:
        sys.path[:] = previous


class RejectedReference(RuntimeError):
    pass


class VerifiedReferenceTests(unittest.TestCase):
    def exercise(self, mode, prepared, reject=None, mismatch=None):
        regular, contextual = generators()
        tool = contextual if mode == "context" else regular
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            here = root / "tools/port-rust-caching-source-map-view"
            source = root / "port/rust/compiler/rustc_span/src"
            reference = root / ".toolchain/oracles/rustc-span-reference"
            here.mkdir(parents=True)
            (source / "testdata").mkdir(parents=True)
            reference.mkdir(parents=True)
            inputs = [root / "production.go", root / "dependency/alternate_platform.go"]
            required = inputs + [here / name for name in ["generate.py", "generate_context.py", "oracle.rs", "context_oracle.rs"]]
            required += [root / "tools/port-rust-span-lib/native_source_helpers.rs",
                         root / "tools/port-rust-symbol/native_profile.py"]
            required += [source / name for name in ["caching_source_map_view.go", "caching_source_map_view_oracle_test.go", "caching_source_map_view_context_test.go"]]
            for path in required:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("isolated bound input\n")
            # Cache files contain deliberately untrusted compiler/linker fields.
            # A prepared cache must never make these executable commands.
            for profile in PROFILES:
                (reference / ("build-profile-" + profile + ".json")).write_text(json.dumps({
                    "rustc": "untrusted-cached-" + profile,
                    "link_arguments": ["untrusted-linkage-" + profile],
                    "debug_assertions": profile[1] == "1",
                    "overflow_checks": profile[3] == "1",
                    "source_revision": regular.native_profile.REVISION,
                    "provenance_path": str(reference / ("provenance-" + profile + ".json")),
                }))
                (reference / ("provenance-" + profile + ".json")).write_text("verified original provenance " + profile)
            if mode == "context":
                manifest = here / "context-manifest.json"
                fixture = source / "testdata/caching_source_map_view_context.jsonl.gz"
            else:
                manifest = here / ("metadata-manifest.json" if mode == "metadata" else "manifest.json")
                suffix = "_metadata" if mode == "metadata" else ""
                fixture = source / ("testdata/caching_source_map_view" + suffix + ".tsv.gz")
            original_evidence = {manifest: b"existing manifest must survive rejected verification\n",
                                 fixture: b"existing fixture must survive rejected verification\n"}
            # Successful generation starts without fixtures. Failure paths must
            # leave existing evidence untouched, including late-profile rejection.
            if reject is not None or mismatch is not None:
                for path, data in original_evidence.items():
                    path.write_bytes(data)
            selections, compiles, executions = [], [], []

            def select(path):
                self.assertEqual(Path(path).parent, reference)
                profile = Path(path).stem.removeprefix("build-profile-")
                self.assertIn(profile, PROFILES)
                selections.append(profile)
                self.assertEqual(json.loads(Path(path).read_text())["rustc"], "untrusted-cached-" + profile)
                if profile == reject:
                    raise RejectedReference("changed actual original source: " + profile)
                debug, overflow = profile[1] == "1", profile[3] == "1"
                if profile == mismatch:
                    overflow = not overflow
                return {"rustc": "fresh-verified-" + profile,
                        "link_arguments": ["fresh-linkage-" + profile],
                        "debug_assertions": debug, "overflow_checks": overflow,
                        "source_revision": regular.native_profile.REVISION,
                        "provenance_path": str(reference / ("provenance-" + profile + ".json"))}, {"VERIFIED": profile}

            def run(command, env, data=None):
                profile = env["VERIFIED"]
                if data is None:
                    self.assertEqual(command, ["fresh-verified-" + profile,
                                              str(here / ("context_oracle.rs" if mode == "context" else "oracle.rs")),
                                              "fresh-linkage-" + profile, "-o",
                                              str(root / ".toolchain/oracles/caching-source-map-view" /
                                                  (("context-" if mode == "context" else "oracle-") + profile))])
                    compiles.append(profile)
                    return b""
                self.assertEqual(command, [str(root / ".toolchain/oracles/caching-source-map-view" / ("oracle-" + profile))])
                self.assertEqual(data, b"1|b,0;s,0,1\n")
                executions.append(profile)
                return b"original-observation\n"

            def child(command, **kwargs):
                profile = kwargs["env"]["VERIFIED"]
                self.assertEqual(command[0], str(root / ".toolchain/oracles/caching-source-map-view" / ("context-" + profile)))
                self.assertIn(command[1], ["normal", "caught", "escaping", "processing"])
                executions.append(profile)
                return SimpleNamespace(returncode=0, stdout=b"R:original-observation\n", stderr=b"")

            argv = [str(tool.__file__)]
            if prepared:
                argv.append("--prepared")
            if mode == "metadata":
                argv.append("--metadata")
            if reject is not None or mismatch is not None:
                argv.append("--check")
            with mock.patch.object(regular, "ROOT", root), mock.patch.object(regular, "HERE", here), \
                 mock.patch.object(contextual, "ROOT", root), mock.patch.object(contextual, "HERE", here), \
                 mock.patch.object(regular.native_profile, "compiler_profile", return_value=("untrusted-initial-command", {"UNTRUSTED": "1"}, {})), \
                 mock.patch.object(regular.native_profile, "source_bytes", return_value=b"unchanged original source"), \
                 mock.patch.object(regular.reference, "load_profile", side_effect=select), \
                 mock.patch.object(regular.span_wrapper_evidence, "inputs", return_value=inputs) as bindings, \
                 mock.patch.object(regular, "cases", return_value=["1|b,0;s,0,1"]), \
                 mock.patch.object(regular, "metadata_cases", return_value=["1|b,0;s,0,1"]), \
                 mock.patch.object(regular, "run", side_effect=run), \
                 mock.patch.object(contextual.subprocess, "run", side_effect=child), \
                 mock.patch.object(contextual.resource, "setrlimit"), \
                 mock.patch.object(sys, "argv", argv), mock.patch("builtins.print"):
                if reject is not None:
                    with self.assertRaisesRegex(RejectedReference, "changed actual original source: " + reject):
                        tool.main()
                elif mismatch is not None:
                    with self.assertRaisesRegex(SystemExit, "reference profile mismatch|native reference profile differs"):
                        tool.main()
                else:
                    tool.main()
                    bindings.assert_called_once_with()
            stopped = reject if reject is not None else mismatch
            if stopped is not None:
                at = PROFILES.index(stopped)
                self.assertEqual(selections, PROFILES[:at + 1])
                self.assertEqual(compiles, PROFILES[:at])
                self.assertTrue(all(profile in PROFILES[:at] for profile in executions))
                bindings.assert_not_called()
                for path, data in original_evidence.items():
                    self.assertEqual(path.read_bytes(), data)
            else:
                self.assertEqual(selections, PROFILES)
                self.assertEqual(compiles, PROFILES)
                self.assertEqual(sorted(set(executions)), PROFILES)
                metadata = json.loads(manifest.read_text())
                self.assertEqual([(p["debug_assertions"], p["overflow_checks"]) for p in metadata["profiles"]],
                                 [(False, False), (False, True), (True, False), (True, True)])
                bound = {item["path"] for item in metadata["files"]}
                self.assertTrue({str(path.relative_to(root)) for path in inputs}.issubset(bound))

    def test_both_prepared_flags_always_use_fresh_all_profile_commands_and_bindings(self):
        for mode in ["normal", "metadata", "context"]:
            for prepared in [False, True]:
                with self.subTest(mode=mode, prepared=prepared):
                    self.exercise(mode, prepared)

    def test_verification_rejection_stops_before_affected_profile_compile_and_preserves_evidence(self):
        for mode in ["normal", "metadata", "context"]:
            for prepared in [False, True]:
                for profile in PROFILES:
                    with self.subTest(mode=mode, prepared=prepared, profile=profile):
                        self.exercise(mode, prepared, reject=profile)

    def test_verified_profile_flag_mismatch_cannot_be_compiled(self):
        for mode in ["normal", "metadata", "context"]:
            for prepared in [False, True]:
                for profile in PROFILES:
                    with self.subTest(mode=mode, prepared=prepared, profile=profile):
                        self.exercise(mode, prepared, mismatch=profile)


if __name__ == "__main__":
    unittest.main()
