// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func TestSymbolNativeBorrows(t *testing.T) {
	extra := strings.Clone("ferrugo_native_static_extra_borrow_20261104")
	session := NewSessionGlobals(Edition2024, []string{extra}, None[SourceMapInputs]())
	defer session.Close()
	input := fmt.Sprintf("ferrugo_native_fresh_borrow_%d", 73)
	first := InternSymbol(session, input)
	stored := first.AsStr(session)
	secondInput := strings.Clone(input)
	second := InternSymbol(session, secondInput)
	byteSymbol := InternByteSymbol(session, []byte(input))
	crates := StdlibStableCratesBorrowed()
	var result strings.Builder
	fmt.Fprintf(&result, "fresh_distinct=%t\n", unsafe.StringData(input) != unsafe.StringData(stored))
	fmt.Fprintf(&result, "occupied_retained=%t\n", unsafe.StringData(stored) == unsafe.StringData(second.AsStr(session)))
	fmt.Fprintf(&result, "occupied_distinct=%t\n", unsafe.StringData(secondInput) != unsafe.StringData(second.AsStr(session)))
	fmt.Fprintf(&result, "cross_kind_shared=%t\n", unsafe.StringData(stored) == unsafe.StringData(byteSymbol.AsByteStr(session)))
	fmt.Fprintf(&result, "prefill_static=%t\n", unsafe.StringData(extra) == unsafe.StringData(InternSymbol(session, extra).AsStr(session)))
	fmt.Fprintf(&result, "stable_crates_static=%t\n", crates == StdlibStableCratesBorrowed())
	fmt.Fprintf(&result, "stable_crates_indices=%d,%d,%d,%d\n", crates[0].AsU32(), crates[1].AsU32(), crates[2].AsU32(), crates[3].AsU32())
	want, err := os.ReadFile("testdata/symbol_borrows.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != string(want) {
		t.Fatalf("native borrow relations differ:\ngot %swant %s", &result, want)
	}
}

func TestSymbolComparatorUsesCurrentRuntime(t *testing.T) {
	old := &rustfmt.RuntimeMetadata{Value: "expired construction scope"}
	current := &rustfmt.RuntimeMetadata{Value: "current operation scope"}
	var seen rustfmt.Context
	newContext := func() *rustvec.Owned[int] {
		return rustvec.NewOwned(7, rustvec.ValueOps[int]{Drop: func(_ *int, ctx rustfmt.Context) { seen = ctx }}).
			MoveWithContext(rustfmt.Context{Runtime: old})
	}
	SymbolStableComparatorOwned(newContext())
	if seen.Runtime != nil {
		t.Fatal("no-context convenience retained an expired runtime scope")
	}
	SymbolStableComparatorOwnedWithContext(newContext(), rustfmt.Context{Runtime: current})
	if seen.Runtime != current {
		t.Fatal("explicit operation lost its current runtime scope")
	}
}
