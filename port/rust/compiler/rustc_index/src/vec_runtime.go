// SPDX-License-Identifier: MIT OR Apache-2.0
// Runtime adapters required by the complete pinned slice.rs/vec.rs port.
package rustcindex

import (
	"os"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// IndexDropContext reports source unwinding to destructor implementations without
// altering Go's global panic behavior or sharing state across goroutines.
type IndexDropContext struct{ Unwinding bool }

// A panic escaping required cleanup during unwinding is unrecoverable in Rust.
// Exit 134 is this host representation of the abort class, not a SIGABRT ABI.
func indexStorageUnwindCleanup(cleanup func()) {
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	cleanup()
}
func indexCapturePanic(body func()) (value any) {
	defer func() { value = recover() }()
	body()
	return nil
}
func (ops ValueOps[T]) hasDrop() bool {
	return ops.DropWithRuntimeContext != nil || ops.DropWithContext != nil || ops.Drop != nil
}
func (ops ValueOps[T]) dropAt(value *T, context IndexDropContext) {
	if ops.DropWithRuntimeContext != nil {
		ops.DropWithRuntimeContext(value, rustfmt.Context{Unwinding: context.Unwinding})
	} else if ops.DropWithContext != nil {
		ops.DropWithContext(value, context)
	} else if ops.Drop != nil {
		ops.Drop(value)
	}
}
func indexDropDuringUnwind[T any](value *T, ops ValueOps[T]) {
	if ops.hasDrop() {
		indexStorageUnwindCleanup(func() { ops.dropAt(value, IndexDropContext{true}) })
	}
}

func indexDeferOwnedValue[T any](value *T, ops ValueOps[T], owned *bool) {
	panicValue := recover()
	if *owned && ops.hasDrop() {
		if panicValue != nil {
			indexDropDuringUnwind(value, ops)
		} else {
			ops.dropAt(value, IndexDropContext{})
		}
	}
	if panicValue != nil {
		panic(panicValue)
	}
}
func indexDeferValueOnPanic[T any](value *T, ops ValueOps[T], owned *bool) {
	if panicValue := recover(); panicValue != nil {
		if *owned {
			indexDropDuringUnwind(value, ops)
		}
		panic(panicValue)
	}
}

// Owned callback descriptors represent Rust closure-environment destruction.
// Plain func entrypoints wrap these with trivial Close; use descriptors when
// the source closure owns resources, including callbacks that are never called.
type IndexFactory[T any] struct {
	Call             func() T
	Close            func()
	CloseWithContext func(IndexDropContext)
}
type IndexFnMut[I, T any] struct {
	Call             func(I) T
	Close            func()
	CloseWithContext func(IndexDropContext)
}
type IndexOnceFactory[T any] struct {
	Call             func() T
	Close            func()
	CloseWithContext func(IndexDropContext)
}

func indexCloseCallback(close func(), closeContext func(IndexDropContext), context IndexDropContext) {
	call := func() {
		if closeContext != nil {
			closeContext(context)
		} else if close != nil {
			close()
		}
	}
	if context.Unwinding {
		indexStorageUnwindCleanup(call)
	} else {
		call()
	}
}
func indexDeferCallback(close func(), closeContext func(IndexDropContext), owned *bool) {
	panicValue := recover()
	if *owned {
		indexCloseCallback(close, closeContext, IndexDropContext{panicValue != nil})
	}
	if panicValue != nil {
		panic(panicValue)
	}
}
func indexCallOnce[T any](factory IndexOnceFactory[T]) T {
	owned := true
	defer indexDeferCallback(factory.Close, factory.CloseWithContext, &owned)
	value := factory.Call()
	owned = false
	// In the pinned Rust profile a return place is leaked if consuming the
	// FnOnce environment panics after producing its result. Do not invent a
	// destructor call for value here; the Option caller has not installed it.
	indexCloseCallback(factory.Close, factory.CloseWithContext, IndexDropContext{})
	return value
}

func indexCloseValueIterator[T any](it IndexValueIterator[T], context IndexDropContext) {
	if it.Owned != nil {
		it.Owned.CloseWithContext(context)
		return
	}
	indexCloseCallback(it.Close, it.CloseWithContext, context)
}
func indexDeferValueIterator[T any](it IndexValueIterator[T]) {
	panicValue := recover()
	indexCloseValueIterator(it, IndexDropContext{panicValue != nil})
	if panicValue != nil {
		panic(panicValue)
	}
}
