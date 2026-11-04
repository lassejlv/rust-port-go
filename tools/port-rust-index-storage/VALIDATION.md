# Validation evidence

The source pin is `2d20b42b883dcfc014f3c2c56081eeb6091c2179`.
The full implementation and tests are `compiler/rustc_index/src/slice.rs`,
`vec.rs`, and `vec/tests.rs`; none of their source bodies is rewritten for the
reference build. Production is Go with standard-library dependencies only.
See [PORT.md](PORT.md) for the API, trait-witness, ownership, runtime, and
representation contracts.

The independent [original-source oracle](oracle/README.md) retains 150,068
reference records and 189,668 operation steps across eight separately compiled
profiles: normal/nightly source features, checked/wrapping overflow, and
enabled/disabled debug assertions. Its manifest binds source Git object bytes,
exact locked registry archives and expanded source, wrapper and request-builder
inputs, compiler versions/flags, and both decoded and compressed corpus hashes.
The actual original `Idx`, `IndexSlice`, and `IndexVec` compile together.

The source crate version is 1.101.0 development and the available development
compiler is Rust 1.99.1. `RUSTC_BOOTSTRAP=1` enables original feature gates; the
nightly reference additionally supplies only the missing `never_type` gate.
The exact compiler/flag mismatch is recorded rather than implying a complete
1.101.0 bootstrap. No shim substitutes for vector, proc-macro, or codec bodies.

The [reference builder](reference-build/README.md) compiles the actual full
index macro crate and original serialization/hash dependencies. All five
unchanged original vector tests pass in both feature profiles. Their nested
Option niche assertions remain intact; both expanded macro outputs are
retained. The real codec round trip also passes. The Go versions of the five
tests explicitly adapt the selected macro-generated key representation and
its expanded Step operations. This does not claim a macro or Rust ABI port.

The differential corpus covers every selected source method and trait path,
normal and exceptional callbacks, construction and capacity growth, exact
index-constructor/conversion order, generic slicing and copy conversion,
disjoint references, binary search, in-place mutation, default/overridden
clone-from, partial clone cleanup, owned closures, custom owned range bounds,
options, inversion assertions, FromIterator/Extend specialization and owned
allocation reuse, independent trusted for-each/fold overrides, and actual
nightly codec and extend trait operations. Iterator records exercise both
directions, shortcut/mapper differences, skip counts up to `usize::MAX`, and
resumption after constructor/destructor panics. Full logical zero-sized
lengths, capacity, and operations are compared without constructing an
impossible Go slice.

Formatter records compare complete flags, signed/unsigned hexadecimal
delegation, precision/width/alignment/fill, NUL and Unicode fill, every string
and independently overridden character write, pretty indentation, multiline
boundaries, nested options, sink failures, callbacks catching writer failures,
outer formatter state and reuse after errors, and local destructor ordering.
Failed-assertion Debug and destructor panic tests execute isolated child
processes and compare abort class and all prior callback events. Go exit 134 is
the host abort representation, not a signal ABI. Focused Go tests additionally
verify real borrowed source/destination/query/hash/debug cells and alias
identity across zero-sized views.

Ordinary Go tests replay committed Rust output offline; they never replace
missing original output with the port's output. Live regeneration and `-check`
commands are in the oracle README. Go 1.23 is supported in both feature
profiles, including generator integrity checks that compare decoded records
across differing gzip library versions. Unrelated production compiler files,
the general standard vector/iterator library, the macro crate, and the full
serialization library receive no Go completion credit from these adapters.
