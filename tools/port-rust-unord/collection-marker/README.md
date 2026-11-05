# Exact private UnordCollection marker

The original Rust module implements its private UnordCollection trait only for
UnordMap, UnordSet and UnordBag. Its blanket ExtendUnord implementation requires
that marker in addition to Extend. Five compiler programs retain positive and
alias bounds, and negative ordered Vec, distinct wrapper, and Deref wrapper
bounds. The positive program also runs ordinary empty extension on all three
source collections and records actual input next/Drop dispatch.

Go promotes private methods from embedded fields. The old private-method-only
constraint could therefore admit a distinct wrapper, including one with an
ordered collection field. The additive actual-owner method on each source
family and the tiny ExtendUnordItems guard enforce exact dynamic Self before
source input conversion or selected Extend side effects. True type aliases
pass. Distinct pointer and value wrappers fail. The guard compares reflect.Type
descriptors, so slice/map/function fields in noncomparable wrapper values never
reach Go interface-data equality.

The external Go test exercises actual alias receiver extension and compares
input dispatch with the original native stdout. Its invalid-wrapper calls must
fail before callback or input effects; the input remains owned and can then be
closed once. Runtime witness validation represents the rejected Go adaptation
of an invalid Rust trait bound; this is not a claim that Rust emits a runtime
panic for an invalid bound.

The compiler graph and loader inputs are verified before each native compile.
Only future proof bindings are refreshed; valid existing fixtures are retained
byte for byte. This scope grants no whole Unord source credit.

```sh
python3 tools/port-rust-unord/collection-marker/generate.py --native-only --check
python3 tools/port-rust-unord/collection-marker/generate.py --check
python3 tools/port-rust-unord/collection-marker/test_tools.py
```

The Go boundary also rejects nil embedding-wrapper pointers before invoking a
promoted receiver method, so invalid wrappers cannot host-nil-dereference or
consume the actual input before the intended source-bound rejection. Null actual
collection pointers are not valid source borrows and are rejected there too.
