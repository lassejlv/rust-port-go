// SPDX-License-Identifier: MIT OR Apache-2.0
package literalescaper

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

func ownedUnitCase[T any](input string, state *validation.OwnedState, context CallbackContext, format func(T) string, call func(string, UnitCallback[validation.OwnedState, T])) {
	callback := UnitCallback[validation.OwnedState, T]{Env: state, Context: context, Call: func(env *validation.OwnedState, ctx CallbackContext, span Range, result Result[T]) {
		text := "Err(" + result.Error.String() + ")"
		if result.IsOK() {
			text = "Ok(" + format(result.Value) + ")"
		}
		env.Call(ctx, fmt.Sprintf("%d..%d:%s", span.Start, span.End, text))
	}, Drop: func(env *validation.OwnedState, ctx CallbackContext) { env.Drop(ctx) }}
	if state.Case.Borrow {
		validation.BorrowScope(context, callback.Destroy, func() { call(input, callback.Borrow()); state.Emit("returned") })
	} else {
		call(input, callback)
		state.Emit("returned")
	}
}
func runLiteralOwnedCase(id int, emit func(string)) {
	runLiteralCase(validation.DecodeOwned(id, true), emit)
}
func runLiteralCase(c validation.OwnedCase, emit func(string)) {
	state := &validation.OwnedState{Case: c, Emit: emit}
	input := validation.LiteralInputs[c.Input]
	if c.Processing {
		input = "\r"
	}
	validation.Ambient(c, emit, func(context rustfmt.Context) {
		char := func(value rune) string {
			return rustfmt.String(func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugChar(value, f) })
		}
		byte := func(value byte) string { return fmt.Sprint(value) }
		switch c.Operation {
		case 0:
			ownedUnitCase(input, state, context, char, CheckRawStrOwned[validation.OwnedState])
		case 1:
			ownedUnitCase(input, state, context, byte, CheckRawByteStrOwned[validation.OwnedState])
		case 2:
			ownedUnitCase(input, state, context, func(v NonZeroChar) string { return v.RustDebug() }, CheckRawCStrOwned[validation.OwnedState])
		case 3:
			ownedUnitCase(input, state, context, char, UnescapeStrOwned[validation.OwnedState])
		case 4:
			ownedUnitCase(input, state, context, byte, UnescapeByteStrOwned[validation.OwnedState])
		case 5:
			ownedUnitCase(input, state, context, func(v MixedUnit) string { return v.RustDebug() }, UnescapeCStrOwned[validation.OwnedState])
		case 6, 7, 8, 9, 10, 11, 12, 13:
			mode := Mode(c.Operation - 6)
			callback := ErrorCallback[validation.OwnedState]{Env: state, Context: context, Call: func(env *validation.OwnedState, ctx CallbackContext, span Range, error EscapeError) {
				env.Call(ctx, fmt.Sprintf("%d..%d:%s", span.Start, span.End, error.String()))
			}, Drop: func(env *validation.OwnedState, ctx CallbackContext) { env.Drop(ctx) }}
			if c.Borrow {
				validation.BorrowScope(context, callback.Destroy, func() { CheckForErrorsOwned(input, mode, callback.Borrow()); emit("returned") })
			} else {
				CheckForErrorsOwned(input, mode, callback)
				emit("returned")
			}
		default:
			panic("invalid original callback operation")
		}
	})
}
func TestOriginalLiteralOwnedCallbacks(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	directory := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../../tools/port-literal-contracts"))
	validation.ReplayOwned(t, directory, "TestOriginalLiteralOwnedCallbacks", runLiteralOwnedCase)
}
func TestOriginalLiteralProcessingPanic(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	directory := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../../tools/port-literal-contracts"))
	validation.ReplayOwnedNamed(t, directory, "processing", "TestOriginalLiteralProcessingPanic", func(id int, emit func(string)) { runLiteralCase(validation.ProcessingCase(id, "literal"), emit) })
}
