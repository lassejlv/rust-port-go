// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	"fmt"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

type contractAccumulator struct {
	ID, Fail int
	State    *validation.OwnedState
}

func dropContractAccumulator(acc *contractAccumulator, context rustfmt.Context) {
	acc.State.Emit(fmt.Sprintf("acc:%d:%t", acc.ID, context.Panicking()))
	if acc.ID == acc.Fail {
		context.Panic("accumulator Drop")
	}
}
func runIteratorOwnedCase(id int, emit func(string)) {
	runIteratorCase(validation.DecodeOwned(id, false), emit)
}
func runIteratorCase(c validation.OwnedCase, emit func(string)) {
	state := &validation.OwnedState{Case: c, Emit: emit}
	input := validation.PredicateInputs[c.Input]
	validation.Ambient(c, emit, func(context rustfmt.Context) {
		it := NewTokenizer(input, FrontmatterNo)
		if c.Operation == 0 {
			consumer := TokenConsumer[validation.OwnedState]{Env: state, Context: context, Call: func(env *validation.OwnedState, ctx rustfmt.Context, token Token) { env.Call(ctx, token.RustDebug()) }, Drop: func(env *validation.OwnedState, ctx rustfmt.Context) { env.Drop(ctx) }}
			if c.Borrow {
				validation.BorrowScope(context, consumer.Destroy, func() { ForEachTokenOwned(it, consumer.Borrow()); emit("returned") })
			} else {
				ForEachTokenOwned(it, consumer)
				emit("returned")
			}
		} else {
			folder := TokenFolder[validation.OwnedState, contractAccumulator]{Env: state, Context: context, Call: func(env *validation.OwnedState, ctx rustfmt.Context, acc contractAccumulator, token Token) (result contractAccumulator) {
				defer func() {
					if p := recover(); p != nil {
						ctx.Unwinding = true
						validation.RequiredCleanup(func() { dropContractAccumulator(&acc, ctx) })
						panic(p)
					}
					dropContractAccumulator(&acc, ctx)
				}()
				env.Call(ctx, fmt.Sprintf("%d:%s", acc.ID, token.RustDebug()))
				return contractAccumulator{acc.ID + 1, acc.Fail, acc.State}
			}, Drop: func(env *validation.OwnedState, ctx rustfmt.Context) { env.Drop(ctx) }}
			fail := -1
			if c.Operation == 2 {
				fail = 1
			}
			acc := contractAccumulator{0, fail, state}
			if c.Borrow {
				validation.BorrowScope(context, folder.Destroy, func() {
					result := FoldTokensOwned(it, acc, folder.Borrow(), dropContractAccumulator)
					emit("returned")
					dropContractAccumulator(&result, context)
				})
			} else {
				result := FoldTokensOwned(it, acc, folder, dropContractAccumulator)
				emit("returned")
				dropContractAccumulator(&result, context)
			}
		}
	})
}
func TestOriginalLazyIteratorOwnedConsumers(t *testing.T) {
	validation.ReplayOwnedNamed(t, contractDirectory(), "iterator-owned", "TestOriginalLazyIteratorOwnedConsumers", runIteratorOwnedCase)
}
func TestOriginalIteratorProcessingPanic(t *testing.T) {
	validation.ReplayOwnedNamed(t, contractDirectory(), "iterator-processing", "TestOriginalIteratorProcessingPanic", func(id int, emit func(string)) { runIteratorCase(validation.ProcessingCase(id, "iterator"), emit) })
}

// The original std from_fn probe injects a Next panic without allocating a
// >4GiB input. Nil is only the Go test's fault injector, not a lawful Rust
// cursor state or a public-input equivalence claim.
func TestOriginalFoldNextPanicCleanup(t *testing.T) {
	validation.ReplayOwnedNamed(t, contractDirectory(), "iterator-next", "TestOriginalFoldNextPanicCleanup", func(id int, emit func(string)) {
		state := &validation.OwnedState{Case: validation.OwnedCase{DropPanic: id == 1}, Emit: emit}
		fail := -1
		if id == 2 {
			fail = 0
		}
		acc := contractAccumulator{0, fail, state}
		folder := TokenFolder[validation.OwnedState, contractAccumulator]{Env: state, Call: func(*validation.OwnedState, rustfmt.Context, contractAccumulator, Token) contractAccumulator {
			panic("unreachable Call")
		}, Drop: func(env *validation.OwnedState, ctx rustfmt.Context) { env.Drop(ctx) }}
		caught := validation.Caught(func() { emit("next"); FoldTokensOwned(&Tokenizer{}, acc, folder, dropContractAccumulator) })
		emit(fmt.Sprintf("caught:%t", caught))
	})
}
