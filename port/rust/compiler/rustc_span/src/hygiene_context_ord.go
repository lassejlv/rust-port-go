// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type ExpnHashStableComparator func(*ExpnHash, *ExpnHash) int

func compareExpnHashes(left, right *ExpnHash) int { return left.fingerprint.Compare(right.fingerprint) }

// The source ignores and drops E before returning its noncapturing comparator.
func ExpnHashStableComparatorOwned[E any](owned *rustvec.Owned[E], ambient ...rustfmt.Context) ExpnHashStableComparator {
	ctx := hygieneOwnedContext(owned.Context(), ambient)
	moved := owned.MoveWithContext(ctx)
	moved.DropWithContext(ctx)
	return compareExpnHashes
}
func ExpnHashStableComparatorWithContext[E any](context E) ExpnHashStableComparator {
	return ExpnHashStableComparatorOwned(rustvec.Own(context))
}
