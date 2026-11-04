This is an additional, bounded proof for the still-partial `unord.rs` port.
It links the complete unchanged pinned `rustc_data_structures` crate from the
shared ten-crate native reference, using the matching Rust 1.101 nightly SDK.
It receives no source credit for the compiler graph or supporting std code.

The 5,662 retained original-Rust records cover:

- 2,400 `UnordSet` insert/remove/contains/get-only/clear/clone/default-clone-from
  operations, including actual table iteration as exposed by derived Debug.
- 1,083 map/set unordered stable hashes with empty, singleton and larger
  collections, three insertion orders, a populated caller hasher, every
  callback panic position, callback order, and the caller's partial hash state.
- 2,179 derived map/set Debug calls with six formatter patterns, independent
  `write_str`/`write_char`, every attempted writer failure, and post-call
  fill/alignment. The original standard builder and inner collection code run.

The supporting type uses the real stable Sip hasher and commutative Fingerprint
operation. The callback dictionary borrows the actual caller context and
element, rather than inventing an additional owned closure environment.

Run `python3 tools/port-rust-unord/traits/generate.py --check` to verify the
complete original graph before rebuilding the oracle. `--skip-prepare` uses an
already prepared graph while still verifying all recorded original-source
bytes. `FERRUGO_SPAN_REFERENCE_PROFILE` selects the shared reference profile.
Existing gzip bytes are retained when their decoded records agree.

`UnordSet`, map trait forwarding, borrowed queries and stable hashing are
genuine functional foundations. General owned `UnordItems` algebra, sorting,
codec defaults and related conversions are pending; this proof does not claim
completion of `unord.rs`.
