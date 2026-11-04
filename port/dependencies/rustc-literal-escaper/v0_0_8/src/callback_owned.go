// SPDX-License-Identifier: Apache-2.0 OR MIT
package literalescaper

import (
	"os"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// CallbackContext exposes ambient source panic state. A locally active panic
// is tracked separately, so an outer unwind that a caller can catch does not
// itself force an abort. Source panic sites use Context.Panic.
type CallbackContext = rustfmt.Context

// UnitCallback is a source FnMut and its complete environment destruction
// witness. Call and Drop receive the actual borrowed environment cell. Drop
// includes source environment field cleanup; Borrow retains that same cell
// while transferring only a trivial-drop mutable reference.
// Passing an owned witness invalidates that ownership, as moving F does in Rust.
type UnitCallback[Env, T any] struct {
	Env     *Env
	Call    func(*Env, CallbackContext, Range, Result[T])
	Drop    func(*Env, CallbackContext)
	Context CallbackContext
}

func (c UnitCallback[Env, T]) Borrow() UnitCallback[Env, T] { c.Drop = nil; return c }
func (c UnitCallback[Env, T]) Destroy(context CallbackContext) {
	if c.Drop != nil {
		c.Drop(c.Env, context)
	}
}

type ErrorCallback[Env any] struct {
	Env     *Env
	Call    func(*Env, CallbackContext, Range, EscapeError)
	Drop    func(*Env, CallbackContext)
	Context CallbackContext
}

func (c ErrorCallback[Env]) Borrow() ErrorCallback[Env] { c.Drop = nil; return c }
func (c ErrorCallback[Env]) Destroy(context CallbackContext) {
	if c.Drop != nil {
		c.Drop(c.Env, context)
	}
}

// ownedCallbackDrop runs required environment destruction before propagating a
// callback panic. A panic escaping this locally active cleanup is the source
// abort class, represented by exit134, after the Drop witness's own cleanup.
func ownedCallbackDrop(context CallbackContext, destroy func(CallbackContext)) {
	if p := recover(); p != nil {
		context.Unwinding = true
		func() {
			defer func() {
				if recover() != nil {
					os.Exit(134)
				}
			}()
			destroy(context)
		}()
		panic(p)
	}
	destroy(context)
}

func CheckRawStrOwned[Env any](src string, callback UnitCallback[Env, rune]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	CheckRawStr(src, func(span Range, result Result[rune]) { callback.Call(callback.Env, callback.Context, span, result) })
}
func CheckRawByteStrOwned[Env any](src string, callback UnitCallback[Env, byte]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	CheckRawByteStr(src, func(span Range, result Result[byte]) { callback.Call(callback.Env, callback.Context, span, result) })
}
func CheckRawCStrOwned[Env any](src string, callback UnitCallback[Env, NonZeroChar]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	CheckRawCStr(src, func(span Range, result Result[NonZeroChar]) {
		callback.Call(callback.Env, callback.Context, span, result)
	})
}
func UnescapeStrOwned[Env any](src string, callback UnitCallback[Env, rune]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	UnescapeStr(src, func(span Range, result Result[rune]) { callback.Call(callback.Env, callback.Context, span, result) })
}
func UnescapeByteStrOwned[Env any](src string, callback UnitCallback[Env, byte]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	UnescapeByteStr(src, func(span Range, result Result[byte]) { callback.Call(callback.Env, callback.Context, span, result) })
}
func UnescapeCStrOwned[Env any](src string, callback UnitCallback[Env, MixedUnit]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	UnescapeCStr(src, func(span Range, result Result[MixedUnit]) {
		callback.Call(callback.Env, callback.Context, span, result)
	})
}
func CheckForErrorsOwned[Env any](src string, mode Mode, callback ErrorCallback[Env]) {
	defer ownedCallbackDrop(callback.Context, callback.Destroy)
	CheckForErrors(src, mode, func(span Range, error EscapeError) { callback.Call(callback.Env, callback.Context, span, error) })
}
