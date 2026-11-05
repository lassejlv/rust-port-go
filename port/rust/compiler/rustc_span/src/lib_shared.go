// SPDX-License-Identifier: MIT OR Apache-2.0
// Supporting Arc used-operation adapter for the full span runtime. This is not
// completion credit for a Rust standard-library source file or an ABI promise.
package rustcspan

import (
	"sync/atomic"
	"unicode/utf8"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type sharedCell[T any] struct {
	value  T
	strong atomic.Uintptr
	drop   func(*T, rustfmt.Context)
}
type sharedOwner[T any] struct {
	cell   *sharedCell[T]
	closed atomic.Bool
}

// Shared owns one reference to an actual value cell. Assignment moves the
// logical owner: descriptor copies share one token and must not be used as
// independent owners. Clone creates a new owner and increments the real count.
// Borrow is temporary immutable access; source borrow lifetimes remain caller
// obligations. All owned fields belong in T. The drop witness supplies borrowed
// method code and must perform the complete source destructor/field cleanup.
type Shared[T any] struct{ owner *sharedOwner[T] }

func NewShared[T any](value T, drop func(*T, rustfmt.Context)) Shared[T] {
	cell := &sharedCell[T]{value: value, drop: drop}
	cell.strong.Store(1)
	return Shared[T]{owner: &sharedOwner[T]{cell: cell}}
}

// newSharedWithInitializer constructs an address-sensitive value in its final
// cell. The initializer borrows that cell, must not let it escape unfinished,
// and owns cleanup of any partially initialized fields if it panics.
func newSharedWithInitializer[T any](initialize func(*T), drop func(*T, rustfmt.Context)) Shared[T] {
	cell := &sharedCell[T]{drop: drop}
	initialize(&cell.value)
	cell.strong.Store(1)
	return Shared[T]{owner: &sharedOwner[T]{cell: cell}}
}
func (s Shared[T]) check(context rustfmt.Context) *sharedCell[T] {
	if s.owner == nil || s.owner.closed.Load() {
		context.Panic("rustcspan: use of absent or consumed shared owner")
	}
	return s.owner.cell
}
func (s Shared[T]) Borrow() *T                                   { return s.BorrowWithContext(rustfmt.Context{}) }
func (s Shared[T]) BorrowWithContext(context rustfmt.Context) *T { return &s.check(context).value }
func (s Shared[T]) StrongCount() uint                            { return uint(s.check(rustfmt.Context{}).strong.Load()) }
func (s Shared[T]) PointerEqual(other Shared[T]) bool {
	return s.check(rustfmt.Context{}) == other.check(rustfmt.Context{})
}
func (s Shared[T]) Clone() Shared[T] { return s.CloneWithContext(rustfmt.Context{}) }
func (s Shared[T]) CloneWithContext(context rustfmt.Context) Shared[T] {
	cell := s.check(context)
	old := cell.strong.Add(1) - 1
	// Arc's relaxed overflow guard aborts before refcount wrapping can permit
	// premature destruction. This guard is independent of Rust debug builds.
	if old > ^uintptr(0)>>1 {
		abort := context
		abort.ProcessingPanic = true
		abort.Panic("shared reference count overflow")
	}
	return Shared[T]{owner: &sharedOwner[T]{cell: cell}}
}
func (s Shared[T]) GetMut() (*T, bool) {
	cell := s.check(rustfmt.Context{})
	if cell.strong.Load() != 1 {
		return nil, false
	}
	return &cell.value, true
}
func (s Shared[T]) Close() { s.CloseWithContext(rustfmt.Context{}) }
func (s Shared[T]) CloseWithContext(context rustfmt.Context) {
	if s.owner == nil || s.owner.closed.Swap(true) {
		return
	}
	cell := s.owner.cell
	if cell.strong.Add(^uintptr(0)) == 0 && cell.drop != nil {
		cell.drop(&cell.value, context)
	}
}

// SharedString retains the immutable string operations consumed by the span
// source. It does not expose writable storage or claim String capacity/layout.
type SharedString struct{ inner Shared[string] }

func NewSharedString(value string) SharedString {
	requireSpanUTF8(value, rustfmt.Context{})
	return SharedString{NewShared(value, nil)}
}

func requireSpanUTF8(value string, context rustfmt.Context) {
	if !utf8.ValidString(value) {
		context.Panic("rustcspan: string is not valid UTF-8")
	}
}
func (s SharedString) String() string      { return *s.inner.Borrow() }
func (s SharedString) Clone() SharedString { return SharedString{s.inner.Clone()} }
func (s SharedString) CloneWithContext(context rustfmt.Context) SharedString {
	return SharedString{s.inner.CloneWithContext(context)}
}
func (s SharedString) StrongCount() uint                        { return s.inner.StrongCount() }
func (s SharedString) PointerEqual(other SharedString) bool     { return s.inner.PointerEqual(other.inner) }
func (s SharedString) Close()                                   { s.inner.Close() }
func (s SharedString) CloseWithContext(context rustfmt.Context) { s.inner.CloseWithContext(context) }
