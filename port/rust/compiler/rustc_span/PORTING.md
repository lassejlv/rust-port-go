# Span source ports and validation

Checkpoint 35 accepts nine complete source/test files (1,942 pinned Rust lines).
Four larger implementations remain scoped partial (9,236 lines) because their
owned String/Path public contracts are open. The 13-file goal remains active.

| Original source | Lines | Checkpoint 35 status |
| --- | ---: | --- |
| `compiler/rustc_span/src/analyze_source_file.rs` |298| Accepted complete file |
| `compiler/rustc_span/src/analyze_source_file/tests.rs` |109| Accepted complete file |
| `compiler/rustc_span/src/caching_source_map_view.rs` |170| Accepted complete file |
| `compiler/rustc_span/src/edit_distance.rs` |289| Accepted complete file |
| `compiler/rustc_span/src/edit_distance/tests.rs` |82| Accepted complete file |
| `compiler/rustc_span/src/fatal_error.rs` |41| Accepted complete file |
| `compiler/rustc_span/src/hygiene.rs` |1766| Scoped partial; no file credit |
| `compiler/rustc_span/src/lib.rs` |2930| Scoped partial; no file credit |
| `compiler/rustc_span/src/tests.rs` |119| Accepted complete file |
| `compiler/rustc_span/src/source_map.rs` |1327| Scoped partial; no file credit |
| `compiler/rustc_span/src/source_map/tests.rs` |799| Accepted complete file |
| `compiler/rustc_span/src/symbol.rs` |3213| Scoped partial; no file credit |
| `compiler/rustc_span/src/symbol/tests.rs` |35| Accepted complete file |

The accepted implementations are source analysis, caching source-map lookup,
edit distance and fatal-error operations. Five canonical child test files
preserve all 43 original span test declarations, including the 12 analyzer macro
cases. Their implementations call the real parent Go package.

The exact source revision is `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.
Source identities, canonical Go paths, per-file evidence and actual command
results are retained in the [integration report](../../../../tools/port-rust-span-reference/checkpoint35-validation.json).
The [58-region lib mapping](../../../../tools/port-rust-span-lib/source-inventory.json)
accounts for all 2,930 lines; it is a source map, not a full API attestation.

The scoped candidate passes 63 native/integrity gates (66 Python tests),
29-package minimum-Go normal/race/tagged replay, default/tagged vet and
focused no-CGO checks. The report distinguishes the frozen full runs from
the later SourceMap producer and keyed-test-literal follow-ups. All 208
historical native observations remain byte-identical. Fresh producer bundles
preserve actual filenames and are independently verified before replay.

The [remaining ownership contracts](../../../../docs/SPAN-API-GAPS.md) have
concrete lawful-caller examples. Their implementation is the next acceptance
goal. DefID and container prerequisites retain their separate partial status.
This work grants no complete standard-library/dependency credit and does
not provide a production Rust frontend/backend. Windows native comparisons
under Wine remain labeled emulated evidence; LSX has no hardware validation.
The complete compiler and Cargo port continues.
