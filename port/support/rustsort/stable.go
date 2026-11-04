// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped dependency translation of Rust 1.101 core slice sorting. This is not
// completion credit for any standard-library source file.
package rustsort

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

type Context = rustfmt.Context

// Type is the actual source type's layout and specialization metadata. It is
// never inferred from Go's layout. Freeze means absence of direct Rust interior
// mutability, including UnsafeCell; it is distinct from Copy.
type Type struct {
	Size, Align  uint
	Freeze, Copy bool
}

// Less borrows both actual source elements and operation metadata. Its caller
// owns any comparator environment and its Drop. No hidden owned captures may
// be smuggled into this borrowed dictionary. Source shared/exclusive borrow
// laws apply throughout the callback, including temporary scratch borrows.
type Less[T any] func(*T, *T, Context) bool

type engine[T any] struct {
	typ     Type
	context Context
	compare Less[T]
}

func (e *engine[T]) less(a, b *T) (result bool) {
	// Route a raw escaping Go panic before sort-local hole cleanup. Translated
	// callbacks use Context.Panic at the actual site; their raw Go local defers
	// cannot be intercepted by this boundary.
	defer func() {
		if p := recover(); p != nil {
			e.context.Panic(p)
		}
	}()
	return e.compare(a, b, e.context)
}

// Stable implements the normal (non-optimize_for_size, 32/64-bit) Rust 1.101
// slice::sort_by strategy. Scratch capacity, specialization thresholds, exact
// comparison order and panic-restoration states follow the pinned SDK source.
// Allocations are represented by Go storage; no Rust ABI/allocator is exposed.
func Stable[T any](v []T, typ Type, less Less[T], contexts ...Context) {
	var context Context
	if len(contexts) > 1 {
		panic("rustsort: expected at most one context")
	}
	if len(contexts) == 1 {
		context = contexts[0]
	}
	if typ.Align == 0 || typ.Align&(typ.Align-1) != 0 || typ.Size%typ.Align != 0 {
		context.Panic("rustsort: invalid source type layout")
	}
	if typ.Size == 0 || len(v) < 2 {
		return
	}
	if less == nil {
		context.Panic("rustsort: missing comparison implementation")
	}
	e := engine[T]{typ: typ, context: context, compare: less}
	if len(v) <= 20 {
		e.insertion(v, 1)
		return
	}
	// stable/mod.rs::driftsort_main: the full 4096-byte stack buffer is
	// visible as scratch, rather than truncating it to the requested length.
	full := uint(8_000_000) / typ.Size
	alloc := max(len(v)-len(v)/2, min(len(v), int(full)))
	alloc = max(alloc, 48)
	scratchLen := alloc
	if stack := uint(4096) / typ.Size; stack >= uint(alloc) {
		scratchLen = int(stack)
	}
	scratch := make([]T, scratchLen)
	e.drift(v, scratch, len(v) <= e.stableThreshold()*2)
}

func (e *engine[T]) stableThreshold() int {
	if e.typ.Freeze {
		return 32
	}
	return 16
}
func (e *engine[T]) stableSmall(v, scratch []T) {
	if len(v) < 2 {
		return
	}
	if e.typ.Freeze {
		e.smallGeneral(v, scratch)
	} else {
		e.insertion(v, 1)
	}
}

func reverse[T any](v []T) {
	for left, right := 0, len(v)-1; left < right; left, right = left+1, right-1 {
		v[left], v[right] = v[right], v[left]
	}
}
