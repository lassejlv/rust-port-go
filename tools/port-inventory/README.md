# File-level port inventory

See [the full port contract](../../docs/PORTING.md) and [current manifest](../../port-status.json).

`inventory.py` prepares exact pinned sparse Git reference caches, generates a complete top-level tree inventory, verifies provenance and records explicit evidence-backed status. It never pushes or commits. Generation/verification read all tree paths/object IDs without hydrating every source blob.

```sh
python3 tools/port-inventory/inventory.py prepare
python3 tools/port-inventory/inventory.py generate
python3 tools/port-inventory/inventory.py verify
python3 -m unittest discover -s tools/port-inventory -p 'test_*.py' -v
```

`data/*-sources.jsonl` includes every tracked `.rs` file. Implementation and harness rows map to Go; Rust language-input subjects retain original Rust and need Go runner evidence. `data/*-support.jsonl` accounts all other tracked files. `data/*-submodules.jsonl` records unexpanded gitlinks separately. `port-status.json` contains pins, tree hashes, inventory digests, role counts and status counts; it contains no percentage-complete claim.

Only `prepare` needs network access. The unit tests create isolated local Git repositories and require no network/source caches. `verify` needs the pinned local reference caches and checks destination/evidence files for completed rows. It cannot establish semantic equivalence by itself.
