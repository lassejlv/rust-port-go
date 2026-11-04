// SPDX-License-Identifier: MIT OR Apache-2.0
// Package rustvec supplies scoped collection and ownership witnesses used by
// translated source. It is not a completed Rust stdVec module or an ABI model.
//
// Callback dictionaries represent static source operations, not additional
// Rust-owned closure environments. Destructible captures and iterator/input
// fields must live in S, with destruction described by ValueOps/Drop. Captured
// Go callbacks may carry borrowed references or immutable metadata; the adapter
// does not invent destruction for hidden Go closure captures.
//
// Ref.Value and Owned.Borrow return temporary source borrows of the actual S
// cell. Next borrows exclusively; SizeHint borrows shared state and may mutate
// only source interior-mutable fields. Callers must preserve Rust's alias and
// lifetime rules: a live reference cannot be retained across Move, Take, Drop,
// or another incompatible borrow. A retained pointer used solely as an opaque
// identity witness does not grant permission to read a moved/dropped value.
//
// Owned transfer invalidates earlier descriptor copies. State is not copied
// for borrowed callbacks. Drop is the complete source environment destruction
// witness, including fields and the implementation's local panic-cleanup
// guards, rather than merely the body of a user Drop::drop implementation.
// The adapter handles destruction of its consumed parameters and partial
// collected vector; it distinguishes an inherited panic context from a new
// escaping panic whose second destructor panic aborts. Processing-panic calls
// must use Context.Panic for source panic semantics, including immediate abort.
//
// IteratorOps.VecFromIterator is a static witness for the concrete iterator
// type's monomorphized standard-library collector specialization. It is not an
// overridable source method. Nil is lawful for actual generic/trusted source
// strategies; adapters with private standard specializations must supply their
// strategy. Callback bodies must retain the ownership/borrowing laws above.
// TrustedLength witnesses the concrete source unsafe TrustedLen implementation,
// not merely ExactSizeIterator or a size_hint that happens to be exact. Generic
// iterators with exact hints still follow their generic source strategy.
package rustvec
