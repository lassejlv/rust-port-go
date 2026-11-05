The new `AddExternalSrcConsuming` and `MapCtxtConsuming` primaries expose the
actual `rustvec.Owned[E]` value to a source `FnOnce::call_once(Self, ...)`
implementation. It may Take, Move or forget whole Self. Existing ref-only
entry points remain conveniences. Transparent envelopes reuse the established
source algorithms and guards; accepted `ConsumeValue` performs exactly the
source callback's remaining-Self destruction at invocation completion. Unused
arguments retain their original destruction point.

The native caller links the complete unchanged pinned crate with manual
FnOnce types owning actual Arc and Vec fields. Its 1,920 records span all four
debug/overflow profiles, three external-source states, four Span representations
and unset-session decoding, ten whole-Self/failure strategies, and ordinary,
current-B, absent-runtime, inherited-caught/escaping and panic-processing modes.
Moved Self is destroyed later under runtime3. All 584 abort records are actual
native SIGABRT; Go compares complete stdout prefixes and termination class.
Native stderr is retained separately. No post-abort state, allocator/stack ABI
or new source-file/standard-library completion credit is claimed.

The producer retains all filenames verbatim, including genuine absolute SDK
scoped-tls registry paths. Client source is compiled with an explicit execution
ROOT plus `/` to empty remap; the metadata records the rule, not the checkout.
The historical producer remains strict and refuses changed raw observations.
CI must generate and replay its own current profile:

```sh
python3 tools/port-rust-span-lib/generate_consuming_callbacks.py --output-dir "$RUNNER_TEMP/span-consuming"
FERRUGO_SPAN_CONSUMING_PROOF="$RUNNER_TEMP/span-consuming" go test ./port/rust/compiler/rustc_span/src -run '^(TestOriginalConsumingCallbacks|TestConsumingProducerSelector)$' -count=1
```

Upload consuming-callbacks-manifest.json, consuming-callbacks.jsonl.gz and
consuming-callbacks.jsonl. Explicitly empty/missing/partial choices fail without
fallback. Parent source/compressed/raw checks precede child requests; every
child validates bounded selected files, schema/pin and the immutable captured
manifest digest. The source bindings cover the complete production closure and
the self-contained replay rather than unrelated sibling test files. Root's
isolated coherent snapshot regeneration and Go acceptance remain required.
