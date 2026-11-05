// SPDX-License-Identifier: MIT OR Apache-2.0
// Symbol's public ContextOrd<E> implementation from symbol.rs.
package rustcspan

import (
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	rustvec "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// SymbolStableComparator is the noncapturing returned function item. Session
// is supplied when it is invoked, adapting lookup of the then-active TLS
// session; the consumed context does not bind a construction-time session.
type SymbolStableComparator func(session *SessionGlobals, left, right *Symbol, ambient ...rustfmt.Context) int

func compareSymbolsStably(session *SessionGlobals, left, right *Symbol, ambient ...rustfmt.Context) int {
	context := rustfmt.Context{}
	if len(ambient) > 1 {
		panic("rustcspan: multiple panic contexts")
	}
	if len(ambient) == 1 {
		context = ambient[0]
	}
	return left.StableCompareWithContext(session, *right, context)
}

// SymbolStableComparatorOwned consumes the caller's exclusive owned context.
// Rust drops the unused E on the function's normal return path, before the
// caller can invoke the comparator. Custom Drop borrows the actual moved cell.
// Translated panic sites use Drop's Context.PanicAt/PanicFormattedAt; raw Go callbacks retain
// their own local defer/recover timing, with boundary handling by Owned.Drop.
func SymbolStableComparatorOwned[E any](context *rustvec.Owned[E]) SymbolStableComparator {
	ambient := context.Context()
	ambient.Runtime = nil
	return SymbolStableComparatorOwnedWithContext(context, ambient)
}
func SymbolStableComparatorOwnedWithContext[E any](context *rustvec.Owned[E], ambient rustfmt.Context) SymbolStableComparator {
	moved := context.MoveWithContext(ambient)
	moved.DropWithContext(ambient)
	return compareSymbolsStably
}

// SymbolStableComparatorWithContext is the trivial-drop convenience witness.
// Use SymbolStableComparatorOwned for an E with an observable destructor.
func SymbolStableComparatorWithContext[E any](context E) SymbolStableComparator {
	return SymbolStableComparatorOwned(rustvec.Own(context))
}
