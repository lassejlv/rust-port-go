# Verbatim panic-location producer

`generate_panic.py` links the complete unchanged source at Rust pin
`2d20b42b883dcfc014f3c2c56081eeb6091c2179` using the verified whole-crate
1.101.0-nightly reference in all four assertion/overflow profiles. The 256
records retain payload kind, text, source location, unwind/backtrace flags,
and real `FreezeLock` reentry observations.

Some actual source filenames are absolute Cargo cache paths. Fresh CI must
produce evidence in a separate directory:

```sh
python3 tools/port-rust-span-lib/generate_panic.py --output-dir "$RUNNER_TEMP/lib-panic"
FERRUGO_LIB_PANIC_PROOF="$RUNNER_TEMP/lib-panic" \
  go test ./port/rust/compiler/rustc_span/src -run '^TestLibOriginalPanicMetadata$' -count=1
```

Upload `panic-manifest.json`, `lib-panic.tsv.gz`, and `lib-panic.tsv` together.
`--check --output-dir <same-directory>` verifies that producer, its unchanged
native observations, and complete production/replay input hashes. This is
suitable on the same producer; historical raw paths are not portable strict
native-regeneration expectations.

The default retained gzip is never overwritten when native raw observations
or actual filenames differ. Its existing compressed bytes are preserved when
the bounded decompressed data and CRC match exactly. A mismatch directs the
caller to `--output-dir`. No historical fixture bytes are changed by this
selector addition.

The Go replay fails closed on an explicitly empty, missing, malformed or
symlink producer selection, incorrect pin/count/profile, hash mismatch, or
changed production/replay bindings. `FERRUGO_LIB_PANIC_MANIFEST_SHA256`, when
provided, additionally fixes the selected manifest identity. All 256 cases
are caught in the same test process; there is no child replay selector.

Each native hook retains a legacy normalized label and a separate **unmodified
actual filename**. Replay uses the former only to locate an already-declared
semantic source-site key. It supplies the latter as an explicit borrowed
source-location profile, then compares the actual Go hook's filename verbatim
along with every remaining metadata field. Neither the observed Rust filename
nor observed Go filename is normalized. No host stack or automatic build-path
inference is involved. The retained historical data uses this same verbatim
comparison and remains replayable independently of its producer's current
filesystem.

Fast producer/selector integrity checks require no Rust build:

```sh
python3 -m unittest discover -s tools/port-rust-span-lib -p test_panic_producer.py
```
