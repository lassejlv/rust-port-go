// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// ClonedBorrowedUnordItems is cloned for the source &T representation used by
// Map::keys and Set/Bag::items. These source references are ordinary Go *T;
// rustvec.Ref[T] additionally carries the temporary trait-call context in the
// other generic entrypoint. No source element, iterator or reference is copied
// here: the actual selected I::cloned implementation owns and moves I.
func ClonedBorrowedUnordItems[S, T, R any](items *UnordItems[*T], selected UnordUnaryIteratorConstructor[S, *T, R, T], contexts ...rustfmt.Context) *UnordItems[T] {
	return unordConstructUnary(items, selected, unordContext(contexts))
}

// CopiedBorrowedUnordItems is the matching actual I::copied call. Selected
// witnesses the source T:Copy bound and must preserve independent I overrides;
// in particular it never substitutes T::clone for the source copy operation.
func CopiedBorrowedUnordItems[S, T, R any](items *UnordItems[*T], selected UnordUnaryIteratorConstructor[S, *T, R, T], contexts ...rustfmt.Context) *UnordItems[T] {
	return unordConstructUnary(items, selected, unordContext(contexts))
}
