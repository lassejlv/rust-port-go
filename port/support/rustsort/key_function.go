// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

// KeyFunction is the owned FnMut(&T)->K parameter of sort_by_cached_key. It
// owns one actual mutable environment cell, including all capture/field Drop.
// Callbacks temporarily borrow that cell and argument; neither borrow may
// escape or overlap an incompatible call/move/drop. The returned K owner must
// use the same complete Drop operation supplied in CachedKeyOps.
type KeyFunction[T, K any] struct{ inner keyFunction[T, K] }
type keyFunction[T, K any] interface {
	call(*T, Context) *rustvec.Owned[K]
	close(Context)
	move(Context) keyFunction[T, K]
}
type keyFunctionCell[S, T, K any] struct {
	state  *rustvec.Owned[S]
	invoke func(*S, *T, Context) *rustvec.Owned[K]
	live   bool
}

func NewKeyFunction[S, T, K any](state *rustvec.Owned[S], invoke func(*S, *T, Context) *rustvec.Owned[K]) KeyFunction[T, K] {
	if state == nil || !state.IsLive() || invoke == nil {
		panic("rustsort: invalid owned key function")
	}
	return KeyFunction[T, K]{inner: &keyFunctionCell[S, T, K]{state: state.Move(), invoke: invoke, live: true}}
}
func (f *keyFunctionCell[S, T, K]) call(value *T, context Context) (result *rustvec.Owned[K]) {
	if !f.live {
		context.Panic("rustsort: use of consumed key function")
	}
	defer func() {
		if p := recover(); p != nil {
			context.ResumeUnwind(p)
		}
	}()
	result = f.invoke(f.state.Borrow(), value, context)
	if result == nil || !result.IsLive() {
		context.Panic("rustsort: key function returned a consumed result")
	}
	return result
}
func (f *keyFunctionCell[S, T, K]) close(context Context) {
	if !f.live {
		return
	}
	f.live = false
	f.state.DropWithContext(context)
}
func (f *keyFunctionCell[S, T, K]) move(context Context) keyFunction[T, K] {
	if !f.live {
		context.Panic("rustsort: use of consumed key function")
	}
	f.live = false
	return &keyFunctionCell[S, T, K]{state: f.state.MoveWithContext(context), invoke: f.invoke, live: true}
}
func (f KeyFunction[T, K]) MoveWithContext(context Context) KeyFunction[T, K] {
	if f.inner == nil {
		context.Panic("rustsort: nil key function")
	}
	return KeyFunction[T, K]{inner: f.inner.move(context)}
}
func (f KeyFunction[T, K]) Call(value *T, context Context) *rustvec.Owned[K] {
	if f.inner == nil {
		context.Panic("rustsort: nil key function")
	}
	return f.inner.call(value, context)
}
func (f KeyFunction[T, K]) CloseWithContext(context Context) {
	if f.inner != nil {
		f.inner.close(context)
	}
}
