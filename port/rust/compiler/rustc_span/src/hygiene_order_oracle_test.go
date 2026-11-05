// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	"strconv"
	"testing"

	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

func hygieneOrderName(n int) string {
	if n < 0 {
		return "Less"
	}
	if n > 0 {
		return "Greater"
	}
	return "Equal"
}
func hygieneOrderClamp(call func() uint32) (text string) {
	defer func() {
		if p := recover(); p != nil {
			text = fmt.Sprint(p)
		}
	}()
	return strconv.FormatUint(uint64(call()), 10)
}
func TestOriginalHygieneOrder(t *testing.T) {
	readHygieneOracle(t, "order", func(row hygieneOracleRow) string {
		n := row.N
		if n < 125 {
			values := [...]uint32{0, 1, 17, 0xfffffeff, 0xffffff00}
			a, b, c := sp.ExpnIndexFromU32(values[n/25]), sp.ExpnIndexFromU32(values[n/5%5]), sp.ExpnIndexFromU32(values[n%5])
			partial, _ := a.PartialCompare(b).Get()
			return fmt.Sprintf("%s Some(%s) %t %t %t %t %d %d %s\n", hygieneOrderName(a.Compare(b)), hygieneOrderName(partial), a.Less(b), a.LessEqual(b), a.Greater(b), a.GreaterEqual(b), a.Min(b).AsU32(), a.Max(b).AsU32(), hygieneOrderClamp(func() uint32 { return a.Clamp(b, c).AsU32() }))
		}
		if n < 152 {
			n -= 125
			values := [...]sp.MacroKind{sp.BangMacro, sp.AttrMacro, sp.DeriveMacro}
			a, b, c := values[n/9], values[n/3%3], values[n%3]
			partial, _ := a.PartialCompare(b).Get()
			return fmt.Sprintf("%s Some(%s) %t %t %t %t %d %d %s\n", hygieneOrderName(a.Compare(b)), hygieneOrderName(partial), a.Less(b), a.LessEqual(b), a.Greater(b), a.GreaterEqual(b), a.Min(b), a.Max(b), hygieneOrderClamp(func() uint32 { return uint32(a.Clamp(b, c)) }))
		}
		if n < 179 {
			n -= 152
			values := [...]sp.Transparency{sp.Transparent, sp.SemiOpaque, sp.Opaque}
			a, b := values[n/9], values[n/3%3]
			partial, _ := a.PartialCompare(b).Get()
			return fmt.Sprintf("Some(%s) %t %t %t %t\n", hygieneOrderName(partial), a.Less(b), a.LessEqual(b), a.Greater(b), a.GreaterEqual(b))
		}
		step := sp.ExpnIndexStep()
		if n < 204 {
			n -= 179
			values := [...]uint32{0, 1, 17, 0xfffffeff, 0xffffff00}
			a, b := sp.ExpnIndexFromU32(values[n/5]), sp.ExpnIndexFromU32(values[n%5])
			lower, upper, ok := step.StepsBetween(a, b)
			if !ok {
				return fmt.Sprintf("(%d, None)\n", lower)
			}
			return fmt.Sprintf("(%d, Some(%d))\n", lower, upper)
		}
		inputs := [...][2]uint32{{17, 0}, {17, 1}, {17, 17}, {0xffffff00, 0}, {0xfffffeff, 1}}
		a, count := sp.ExpnIndexFromU32(inputs[n-204][0]), uint(inputs[n-204][1])
		forward, backward := step.Forward(a, count), step.Backward(a, count)
		fc, fok := step.ForwardChecked(a, count)
		bc, bok := step.BackwardChecked(a, count)
		fo, fover := step.ForwardOverflowing(a, count)
		bo, bover := step.BackwardOverflowing(a, count)
		if !fok || !bok {
			t.Fatal("native-valid stepping input failed")
		}
		return fmt.Sprintf("%s %d %d %d %d Some(%d) Some(%d) (%d, %t) (%d, %t)\n", step.TypeName, forward.AsU32(), backward.AsU32(), step.ForwardUnchecked(a, count).AsU32(), step.BackwardUnchecked(a, count).AsU32(), fc.AsU32(), bc.AsU32(), fo.AsU32(), fover, bo.AsU32(), bover)
	})
}
