// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	"strings"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type hygieneIndexFailureValue[T any] interface {
	AsU32() uint32
	AddWithContext(uint, rf.Context, ...sp.HygieneConfig) T
}

func hygieneIndexFailureValueRun[T hygieneIndexFailureValue[T]](action string, s *hygieneOwnedCore, c rf.Context, from32 func(uint32, rf.Context) T, fromSize func(uint, rf.Context) T, maximum T, assign func(*T, uint, rf.Context, ...sp.HygieneConfig), dictionary index.Idx[T]) {
	switch action {
	case "u32":
		_ = from32(^uint32(0), c)
	case "usize":
		_ = fromSize(^uint(0), c)
	case "add_bound":
		_ = maximum.AddWithContext(1, c, s.profile)
	case "add_overflow":
		v := from32(1, c).AddWithContext(^uint(0), c, s.profile)
		fmt.Printf("returned:index:%d\n", v.AsU32())
	case "plus_bound":
		_ = dictionary.Plus(maximum, 1)
	case "plus_overflow":
		v := dictionary.Plus(from32(1, c), ^uint(0))
		fmt.Printf("returned:index:%d\n", v.AsU32())
	case "assign_bound", "assign_overflow", "increment_bound", "increment_overflow":
		value, amount := maximum, uint(1)
		if strings.HasSuffix(action, "_overflow") {
			value = from32(1, c)
			amount = ^uint(0)
		}
		defer hygieneOwnedClose(c, func(c rf.Context) {
			fmt.Printf("index_state:%d:%t:%d:%d\n", value.AsU32(), c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		})
		if strings.HasPrefix(action, "increment_") {
			dictionary.IncrementBy(&value, amount)
		} else {
			assign(&value, amount, c, s.profile)
		}
		fmt.Printf("returned:assign:%d\n", value.AsU32())
	default:
		panic("unknown index action")
	}
}
func hygieneIndexFailureRun(operation string, s *hygieneOwnedCore, c rf.Context) {
	rest := strings.TrimPrefix(operation, "index_")
	switch {
	case strings.HasPrefix(rest, "syntax_"):
		hygieneIndexFailureValueRun(strings.TrimPrefix(rest, "syntax_"), s, c, sp.SyntaxContextFromU32WithContext, sp.SyntaxContextFromUsizeWithContext, sp.SyntaxContextMax(), (*sp.SyntaxContext).AddAssignWithContext, sp.SyntaxContextIdxWithContext(c, s.profile))
	case strings.HasPrefix(rest, "expn_"):
		hygieneIndexFailureValueRun(strings.TrimPrefix(rest, "expn_"), s, c, sp.ExpnIndexFromU32WithContext, sp.ExpnIndexFromUsizeWithContext, sp.ExpnIndexMax(), (*sp.ExpnIndex).AddAssignWithContext, sp.ExpnIndexIdxWithContext(c, s.profile))
	case strings.HasPrefix(rest, "local_"):
		hygieneIndexFailureValueRun(strings.TrimPrefix(rest, "local_"), s, c, sp.LocalExpnIdFromU32WithContext, sp.LocalExpnIdFromUsizeWithContext, sp.LocalExpnIdMax(), (*sp.LocalExpnId).AddAssignWithContext, sp.LocalExpnIdIdxWithContext(c, s.profile))
	case strings.HasPrefix(rest, "decoded_"):
		hygieneIndexFailureValueRun(strings.TrimPrefix(rest, "decoded_"), s, c, sp.DecodedSyntaxContextFromU32WithContext, sp.DecodedSyntaxContextFromUsizeWithContext, sp.DecodedSyntaxContextMax(), (*sp.DecodedSyntaxContext).AddAssignWithContext, sp.DecodedSyntaxContextIdxWithContext(c, s.profile))
	default:
		one, zero := sp.ExpnIndexFromU32(1), sp.ExpnIndexZero()
		switch rest {
		case "step_forward_bound":
			_ = sp.ExpnIndexMax().ForwardCheckedWithContext(1, c)
		case "step_forward_none":
			v := one.ForwardCheckedWithContext(^uint(0), c)
			if raw, ok := v.Get(); ok {
				fmt.Printf("returned:step:Some(%d)\n", raw.AsU32())
			} else {
				fmt.Println("returned:step:None")
			}
		case "step_backward_none":
			v := zero.BackwardCheckedWithContext(1, c)
			if raw, ok := v.Get(); ok {
				fmt.Printf("returned:step:Some(%d)\n", raw.AsU32())
			} else {
				fmt.Println("returned:step:None")
			}
		case "step_forward_overflow":
			v, overflow := one.ForwardOverflowingWithContext(^uint(0), c)
			fmt.Printf("returned:step_overflow:%d:%t\n", v.AsU32(), overflow)
		case "step_backward_overflow":
			_, _ = zero.BackwardOverflowingWithContext(1, c)
		case "step_forward":
			_ = one.ForwardWithContext(^uint(0), c)
		case "step_backward":
			_ = zero.BackwardWithContext(1, c)
		default:
			panic("unknown index step action")
		}
	}
}
