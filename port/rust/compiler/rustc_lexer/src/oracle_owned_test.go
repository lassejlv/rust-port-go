// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	"encoding/hex"
	"fmt"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

func runPredicateOwnedCase(id int, emit func(string)) {
	runPredicateCase(validation.DecodeOwned(id, false), emit)
}
func runPredicateCase(c validation.OwnedCase, emit func(string)) {
	state := &validation.OwnedState{Case: c, Emit: emit}
	input := validation.PredicateInputs[c.Input]
	validation.Ambient(c, emit, func(context rustfmt.Context) {
		cursor := NewCursor(input, FrontmatterNo)
		var caught any
		func() {
			defer func() { caught = recover() }()
			predicate := Predicate[validation.OwnedState]{Env: state, Context: context, Call: func(env *validation.OwnedState, ctx rustfmt.Context, c rune) bool {
				env.Call(ctx, fmt.Sprint(c))
				return c != '!'
			}, Drop: func(env *validation.OwnedState, ctx rustfmt.Context) { env.Drop(ctx) }}
			if c.Borrow {
				validation.BorrowScope(context, predicate.Destroy, func() { EatWhileOwned(cursor, predicate.Borrow()); emit("returned") })
			} else {
				EatWhileOwned(cursor, predicate)
				emit("returned")
			}
		}()
		emit(fmt.Sprintf("cursor:%s:%d:%d:%d:%d:%d:%t", hex.EncodeToString([]byte(cursor.AsStr())), cursor.Prev(), cursor.First(), cursor.Second(), cursor.Third(), cursor.PosWithinToken(), cursor.IsEOF()))
		if caught != nil {
			panic(caught)
		}
	})
}
func TestOriginalLexerOwnedPredicate(t *testing.T) {
	validation.ReplayOwned(t, contractDirectory(), "TestOriginalLexerOwnedPredicate", runPredicateOwnedCase)
}
func TestOriginalPredicateProcessingPanic(t *testing.T) {
	validation.ReplayOwnedNamed(t, contractDirectory(), "processing", "TestOriginalPredicateProcessingPanic", func(id int, emit func(string)) { runPredicateCase(validation.ProcessingCase(id, "lexer"), emit) })
}
