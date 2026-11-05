# Tracing failure-boundary proofs

These clients link the actual unchanged pinned tracing dependency through the
verified whole-span reference graph and matching nightly 2026-11-04 compiler.
They provide scoped dependency support evidence, not whole tracing-file credit.

`generate.py` retains three real panic-hook cases: raw subscriber failure in
event delivery, new-span delivery and on-register-dispatch. The source aborts
before owned field/subscriber cleanup. The Go component boundary preserves that
behavior; callback-local raw Go defers remain a convenience outside Rust frame
lowering. Translated callbacks use their supplied Context at actual panic sites.

`generate_borrow.py` retains nine cases for set_default while get_default holds
its borrow, DefaultGuard destruction under that borrow and zero span IDs.
Normal, inherited-caught outer unwind and real processing-panic contexts retain
all hooks, payload kinds, verbatim locations and frame events. Production never
infers a Cargo-cache filename; the replay supplies explicit immutable location
cells from the retained producer profile. DefaultGuard's actual selected point
is dispatcher.rs897:65, independently observed rather than guessed from a nearby
source line.

`generate_registry.py` retains eight scripts for writer/reader registry panic,
singleton versus multiple registrars, subsequent new-dispatch/rebuild calls,
subscriber destruction and inherited/processing contexts. A writer only poisons
when it began outside panic and an escaping panic unwinds it. Readers do not
poison. Failed poisoned-lock unwrap raises before subscriber callbacks while its
acquired guard is still held; replay hooks check the corresponding real Go lock
with TryRLock/TryLock. Original dependency source retains the acquired guard
inside the Err payload until unwind. This hook-lock observation is compositional
with that unchanged source and standard RwLock semantics; the external Rust
client cannot access tracing's private global registry lock directly.

All twenty cases retain stdout hook/callback/Drop events. Eight native children
must terminate with actual SIGABRT; Go children use the documented exit134 abort
representation. Terminal runtime stderr is host-specific and is not compared.
Generators enforce finite timeouts and reject any unexpected child status.

```sh
python3 tools/port-rust-tracing/processing/generate.py --check
python3 tools/port-rust-tracing/processing/generate_borrow.py --check
python3 tools/port-rust-tracing/processing/generate_registry.py --check
go test ./port/support/rusttracing
```

Borrow and registry evidence contains actual dependency-source Cargo-cache
filenames. The checked-in files are historical producer profiles. Default
generation refuses different raw observations and directs a new producer to
`--output-dir`; observer filenames are never normalized. Both generators can
write into the same explicit directory:

```sh
producer="${RUNNER_TEMP:-/tmp}/ferrugo-tracing-producer"
python3 tools/port-rust-tracing/processing/generate_borrow.py --output-dir "$producer"
python3 tools/port-rust-tracing/processing/generate_registry.py --output-dir "$producer"
FERRUGO_TRACING_PRODUCER_DIR="$producer" go test ./port/support/rusttracing -count=1
python3 -m unittest discover -s tools/port-rust-tracing/processing -p 'test_*.py'
```

Upload all four files in that directory: BORROW_VALIDATION.json,
borrow_panic.tsv, REGISTRY_VALIDATION.json and registry_poison.tsv. Each
manifest binds current source/tool inputs, exact raw bytes and producer
locations. Parent and child replay validate bounded regular files, source and
raw hashes, schema/counts and abort coverage. Explicit empty, missing,
symlinked or corrupted selections never fall back. Parents pass the canonical
directory and captured manifest SHA to children. Relocation tests replay all
seventeen cases from a copied bundle without changing its producer metadata.
The 384 dispatch cases and three processing cases remain stable `--check`
profiles. Complete verified source/archive/compiler/reference loading remains
mandatory for every native producer run. This repair changes tooling and Go
tests only; it adds no whole tracing or Span source-file credit.
