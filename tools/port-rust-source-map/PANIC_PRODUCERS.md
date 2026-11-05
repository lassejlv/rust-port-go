The `--session-map` and `--assert-id` panic-site proofs retain native filenames verbatim. Their historical gzip fixtures and schema-1 manifests are immutable observations of the original producer. A different Cargo cache or checkout may have different filenames; it must produce separate evidence.

```sh
python3 tools/port-rust-source-map/generate_panic_sites.py --session-map --output-dir "$fresh"
python3 tools/port-rust-source-map/generate_panic_sites.py --assert-id --output-dir "$fresh"
python3 tools/port-rust-source-map/generate_panic_sites.py --session-map --output-dir "$fresh" --check
python3 tools/port-rust-source-map/generate_panic_sites.py --assert-id --output-dir "$fresh" --check
FERRUGO_SOURCE_MAP_PANIC_PROOF="$fresh" go test ./port/rust/compiler/rustc_span/src -run '^(TestSourceMapNativePanicSessionMap|TestSourceMapStableIdAssertionComposition|TestSourceMapPanicProducer.*)$' -count=1
```

Each scope writes `source_map_panic_{session,assert}.tsv`, `.tsv.gz`, and `_manifest.json` in the selected directory. The four records cover independent debug/overflow profiles. The session scope has four caught panics; the assertion scope has two, with the other two assertions disabled. Every build uses the complete source/archive/tool-verified reference loader. Output and replay inputs are bounded, and the schema-2 manifest binds the producer, replay, relevant implementation inputs, source identity, and verified build provenance.

`FERRUGO_SOURCE_MAP_PANIC_PROOF` is an explicit directory selection, never a fallback hint. Empty, missing, symlinked, incomplete, stale, corrupt, or wrong-scope evidence fails. Replay checks compressed/raw hashes, exact requests/profile order, and the unchanged native filename in every hook against its selected location profile. Optional `FERRUGO_SOURCE_MAP_PANIC_SESSION_MANIFEST_SHA256` and `FERRUGO_SOURCE_MAP_PANIC_ASSERT_MANIFEST_SHA256` pin the selected manifest bytes. Without the directory variable, the retained historical fixture and its own historical filename profile are used. No production location is inferred or changed.

The default generator cannot overwrite either historical producer on filename drift. Fresh `--output-dir` is supported only for these two scopes; other panic-site scopes retain their existing stable-path validation. These are bounded composition proofs, not extra full-file source credit. The assertion proof uses actual SDK `debug_assert_eq!` with genuine `StableSourceFileId` values; the private source assertion is unreachable with lawful deterministic constructors.

Python integrity checks need no compiler: `python3 -m unittest discover -s tools/port-rust-source-map -p test_panic_site_producer.py -v`.
