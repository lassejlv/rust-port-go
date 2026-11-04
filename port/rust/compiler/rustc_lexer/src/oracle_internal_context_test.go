// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	"fmt"
	"strconv"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

func runInternalContextCase(id int, emit func(string)) {
	mode := id / 3
	c := validation.OwnedCase{Ambient: id%3 == 1, Processing: id%3 == 2}
	validation.Ambient(c, emit, func(context rustfmt.Context) {
		defer func() {
			p := recover()
			emit(fmt.Sprintf("scope_drop:%t", context.Panicking() || p != nil))
			if p != nil {
				panic(p)
			}
		}()
		emit("before")
		switch mode {
		case 0:
			NewCursor("λ", FrontmatterNo).EatUntilWithContext(0xbb, context)
		case 1:
			NewCursorWithContext("λ", FrontmatterNo, context).BumpBytes(1)
		case 2:
			cursor := NewCursor("#\"x\"#", FrontmatterNo)
			cursor.Bump()
			cursor.GuardedDoubleQuotedStringWithContext(context)
		case 3:
			ValidateRawStrWithContext("", 0, context)
		case 4:
			ValidateRawStrWithContext("r", 2, context)
		case 5:
			ValidateRawStrWithContext("x", 1, context)
		case 6:
			NewCursorWithContext("", FrontmatterNo, context).addU32(^uint32(0), 1)
		case 7:
			NewCursorWithContext("", FrontmatterNo, context).subU32(0, 1)
		case 8, 9:
			// A private position bias injects the exact bounded u32 source
			// subtraction into real Next. Native uses that same arithmetic in
			// from_fn. Neither side claims a >4GiB text allocation/lexing run.
			it := NewTokenizer("---", FrontmatterYes)
			bias := uint64(^uint32(0)) - 1
			it.cursor.lenRemaining += int(bias)
			state := &validation.OwnedState{Emit: emit}
			if mode == 8 {
				ForEachTokenOwned(it, TokenConsumer[validation.OwnedState]{Env: state, Context: context, Call: func(*validation.OwnedState, rustfmt.Context, Token) { panic("unreachable Call") }, Drop: func(env *validation.OwnedState, ctx rustfmt.Context) { env.Drop(ctx) }})
			} else {
				folder := TokenFolder[validation.OwnedState, contractAccumulator]{Env: state, Context: context, Call: func(*validation.OwnedState, rustfmt.Context, contractAccumulator, Token) contractAccumulator {
					panic("unreachable Call")
				}, Drop: func(env *validation.OwnedState, ctx rustfmt.Context) { env.Drop(ctx) }}
				result := FoldTokensOwned(it, contractAccumulator{0, -1, state}, folder, dropContractAccumulator)
				dropContractAccumulator(&result, context)
			}
		case 10, 11, 12, 13:
			it := NewTokenizer("---", FrontmatterYes)
			bias := uint64(^uint32(0)) - 1
			it.cursor.lenRemaining += int(bias)
			switch mode {
			case 10:
				it.NextWithContext(context)
			case 11:
				it.NthWithContext(0, context)
			case 12:
				it.CountWithContext(context)
			case 13:
				it.LastWithContext(context)
			}
		default:
			panic("unknown internal source failure site")
		}
		emit("returned")
	})
}
func TestOriginalInternalFailureContexts(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("native u32 position-bias probe requires64-bit host")
	}
	validation.ReplayOwnedNamed(t, contractDirectory(), "internal-context", "TestOriginalInternalFailureContexts", runInternalContextCase)
}
func TestCursorContextScopesRestore(t *testing.T) {
	wrapping := NewCursorWithConfigAndContext("", FrontmatterNo, CursorConfig{}, rustfmt.Context{ProcessingPanic: true})
	if wrapping.addU32(^uint32(0), 1) != 0 || wrapping.subU32(0, 1) != ^uint32(0) {
		t.Fatal("processing context changed wrapping configuration")
	}
	wrapping.debugAssert(false)
	for _, prior := range []rustfmt.Context{{}, {Unwinding: true}} {
		it := NewTokenizerWithContext("a", FrontmatterNo, prior)
		calls, drops := 0, 0
		ForEachTokenOwned(it, TokenConsumer[int]{Context: rustfmt.Context{Unwinding: true}, Call: func(_ *int, ctx rustfmt.Context, _ Token) {
			calls++
			if !ctx.Panicking() {
				t.Fatal("Call lost inherited panic state")
			}
		}, Drop: func(_ *int, ctx rustfmt.Context) {
			drops++
			if !ctx.Panicking() {
				t.Fatal("Drop lost inherited panic state")
			}
		}})
		if calls != 1 || drops != 1 || it.cursor.context != prior {
			t.Fatal("ForEach context was not scoped")
		}
		it = NewTokenizerWithContext("a", FrontmatterNo, prior)
		FoldTokensOwned(it, 0, TokenFolder[int, int]{Context: rustfmt.Context{Unwinding: true}, Call: func(_ *int, ctx rustfmt.Context, a int, _ Token) int {
			if !ctx.Panicking() {
				t.Fatal("fold Call lost inherited state")
			}
			return a + 1
		}}, nil)
		if it.cursor.context != prior {
			t.Fatal("Fold context was not restored")
		}
		cursor := NewCursorWithContext("abc", FrontmatterNo, prior)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("expected predicate panic")
				}
			}()
			EatWhileOwned(cursor, Predicate[int]{Context: rustfmt.Context{Unwinding: true}, Call: func(_ *int, ctx rustfmt.Context, _ rune) bool {
				if !ctx.Panicking() {
					t.Fatal("predicate lost inherited state")
				}
				ctx.Panic("callback")
				return false
			}})
		}()
		if cursor.context != prior {
			t.Fatal("predicate context was not restored after unwind")
		}
	}
}
