// SPDX-License-Identifier: MIT OR Apache-2.0
// Borrowed and owned adapters for binary_search_slice at Rust
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package binarysearchutil

import (
	"os"

	"github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// SearchContext carries source panic state. Translated Rust panic sites use
// Context.Panic; ordinary Go panic/defer callbacks retain Go cleanup timing.
type SearchContext = rustfmt.Context

// ExtractedKey owns one temporary. Drop and the operator receive the same
// actual Value cell. Nil Drop represents a trivial destructor.
type ExtractedKey[K any] struct {
	Value K
	Drop  func(*K, SearchContext)
}

// OwnedExtractor is the consumed Fn(&E)->K witness. Methods borrow the actual
// capture cell; Close destroys it even if no keys were requested. Implementers
// must follow Rust's Fn, ownership and shared-borrow rules. A consumed non-Copy
// extractor must not be reused.
type OwnedExtractor[E, K any] interface {
	Extract(*E, SearchContext) ExtractedKey[K]
	Close(SearchContext)
}

// ExtractorFuncs adapts two callbacks that can capture the same environment
// pointer. Implement OwnedExtractor on the actual object's pointer type when
// the source Fn receiver itself has observable identity.
type ExtractorFuncs[E, K any] struct {
	Call func(*E, SearchContext) ExtractedKey[K]
	Drop func(SearchContext)
}

func (extractor ExtractorFuncs[E, K]) Extract(element *E, context SearchContext) ExtractedKey[K] {
	return extractor.Call(element, context)
}
func (extractor ExtractorFuncs[E, K]) Close(context SearchContext) {
	if extractor.Drop != nil {
		extractor.Drop(context)
	}
}

// BorrowedKeyOperators supplies independent lt/ne/le implementations, borrowing
// the temporary and caller's original query cell rather than copies of either.
type BorrowedKeyOperators[K any] struct {
	Less        func(*K, *K, SearchContext) bool
	NotEqual    func(*K, *K, SearchContext) bool
	LessOrEqual func(*K, *K, SearchContext) bool
}

// Inherited unwind can still be caught by the caller. Only a second panic
// escaping required cleanup aborts here; exit134 represents the abort class.
func searchCleanup(completed *bool, context SearchContext, close func(SearchContext)) {
	if *completed {
		if context.ProcessingPanic {
			closed := false
			defer func() {
				if !closed {
					os.Exit(134)
				}
			}()
			close(context)
			closed = true
			return
		}
		close(context)
		return
	}
	value := recover()
	// Raw Go callback defers have already run by this point. Stop before any
	// component-owned cleanup; primary translated sites use Context.Panic to
	// stop before callback-local cleanup as well.
	if context.ProcessingPanic {
		os.Exit(134)
	}
	context.Unwinding = true
	func() {
		closed := false
		defer func() {
			if !closed {
				os.Exit(134)
			}
		}()
		close(context)
		closed = true
	}()
	panic(value)
}

func compareExtracted[E, K any](element *E, extractor OwnedExtractor[E, K], query *K, compare func(*K, *K, SearchContext) bool, context SearchContext) (result bool) {
	key := extractor.Extract(element, context)
	completed := false
	defer searchCleanup(&completed, context, func(context SearchContext) {
		if key.Drop != nil {
			key.Drop(&key.Value, context)
		}
	})
	result = compare(&key.Value, query, context)
	completed = true
	return
}

// BinarySearchSliceOwned is the primary source function with normal context.
// It consumes extractor even for an empty input or a missing key.
func BinarySearchSliceOwned[E, K any](data []E, extractor OwnedExtractor[E, K], query *K, operators BorrowedKeyOperators[K]) []E {
	return BinarySearchSliceOwnedWithContext(data, extractor, query, operators, SearchContext{})
}

// BinarySearchSliceOwnedWithContext preserves temporary/closure destruction,
// real borrowed cells and the source's query/callback order. Input/order rules
// match BinarySearchSliceWithOperators; the result borrows data. The query must
// be a valid cell, and callbacks must respect the source borrowing contracts.
func BinarySearchSliceOwnedWithContext[E, K any](data []E, extractor OwnedExtractor[E, K], query *K, operators BorrowedKeyOperators[K], context SearchContext) (result []E) {
	raw := rustcindex.IndexSliceFromRaw(data, rustcindex.UsizeIdx(), rustcindex.ValueOps[E]{}).Raw
	resultRaw := BinarySearchStorageOwnedWithContext(raw, extractor, query, operators, context)
	if resultRaw.IsZeroSized() && resultRaw.Len() != 0 {
		// All zero-sized element offsets share the input address. Project into
		// that original allocation; AsGoSlice would create a different owner.
		length := int(resultRaw.Len())
		return data[:length:length]
	}
	return resultRaw.AsGoSlice()
}

// BinarySearchStorageOwned is the full usize-domain slice entry point. The
// shared raw-slice representation also supports inhabited zero-sized elements
// with length usize::MAX, without constructing an impossible Go []E.
func BinarySearchStorageOwned[E, K any](data rustcindex.IndexRaw[E], extractor OwnedExtractor[E, K], query *K, operators BorrowedKeyOperators[K]) rustcindex.IndexRaw[E] {
	return BinarySearchStorageOwnedWithContext(data, extractor, query, operators, SearchContext{})
}

func BinarySearchStorageOwnedWithContext[E, K any](data rustcindex.IndexRaw[E], extractor OwnedExtractor[E, K], query *K, operators BorrowedKeyOperators[K], context SearchContext) (result rustcindex.IndexRaw[E]) {
	completed := false
	defer searchCleanup(&completed, context, extractor.Close)
	size := data.Len()
	start := partitionPointStorage(size, func(position uint) bool {
		return compareExtracted(data.At(position), extractor, query, operators.Less, context)
	})
	if start == size || compareExtracted(data.At(start), extractor, query, operators.NotEqual, context) {
		completed = true
		return rustcindex.IndexRaw[E]{}
	}
	offset := start + 1
	end := partitionPointStorage(size-offset, func(position uint) bool {
		return compareExtracted(data.At(position+offset), extractor, query, operators.LessOrEqual, context)
	}) + offset
	result = data.Slice(start, end)
	completed = true
	return
}

func partitionPointStorage(size uint, predicate func(uint) bool) uint {
	if size == 0 {
		return 0
	}
	base := uint(0)
	for size > 1 {
		half := size / 2
		middle := base + half
		if predicate(middle) {
			base = middle
		}
		size -= half
	}
	if predicate(base) {
		return base + 1
	}
	return base
}
