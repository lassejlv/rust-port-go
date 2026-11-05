// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststr

import (
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestOwnedStringCellGrowthBorrowAndMove(t *testing.T) {
	s := NewOwnedStringWithCapacity(4, rustfmt.Context{})
	cell := s.Borrow()
	cell.PushString("abc")
	before := unsafe.StringData(cell.AsStr())
	cell.Reserve(1)
	if cell.Len() != 3 || cell.Capacity() != 4 || unsafe.StringData(cell.AsStr()) != before {
		t.Fatal("sufficient reserve allocated")
	}
	cell.PushCharWithContext('🦀', rustfmt.Context{})
	if cell.AsStr() != "abc🦀" || cell.Len() != 7 || cell.Capacity() != 8 {
		t.Fatal(cell.AsStr(), cell.Len(), cell.Capacity())
	}
	cell.ReserveExactWithContext(10, rustfmt.Context{})
	if cell.Capacity() != 17 {
		t.Fatal(cell.Capacity())
	}
	before = unsafe.StringData(cell.AsStr())
	moved := s.MoveWithContext(rustfmt.Context{})
	if moved.Borrow() != cell || unsafe.StringData(moved.Borrow().AsStr()) != before {
		t.Fatal("owner move cloned actual cell/buffer")
	}
	transferred := OwnedStringFromMovedCell(moved.TakeCellWithContext(rustfmt.Context{}), rustfmt.Context{})
	if transferred.Borrow() == cell || unsafe.StringData(transferred.Borrow().AsStr()) != before || transferred.Borrow().Capacity() != 17 {
		t.Fatal("String value move lost buffer/capacity or retained old header")
	}
	transferred.Close()
}
func TestOwnedStringCapacityFailureAtCurrentSourceSite(t *testing.T) {
	for _, ambient := range []bool{false, true} {
		owned := OwnedStringFromGo("abc", rustfmt.Context{})
		cell := owned.Borrow()
		before := unsafe.StringData(cell.AsStr())
		location := &rustfmt.SourceLocation{File: "selected SDK raw_vec/mod.rs", Line: 28, Column: 5}
		calls := 0
		context := rustfmt.Context{Unwinding: ambient, Runtime: &rustfmt.RuntimeMetadata{Value: 2}}
		context = rustfmt.WithPanicOperations(context, &rustfmt.PanicOperations{LocationCells: map[string]*rustfmt.SourceLocation{OwnedStringCapacityPanicSite: location}, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
			calls++
			if info.Payload != "capacity overflow" || info.Kind != rustfmt.PanicStaticStr || info.Location != location || current.Runtime.Value != 2 || !current.ProcessingPanic || !current.Unwinding {
				t.Fatal(info, current)
			}
		}})
		func() {
			defer func() {
				if recover() == nil {
					t.Error("reserve overflow accepted")
				}
			}()
			cell.ReserveWithContext(^uint(0), context)
		}()
		if calls != 1 || cell.AsStr() != "abc" || cell.Capacity() != 3 || unsafe.StringData(cell.AsStr()) != before {
			t.Fatal("capacity failure changed original String")
		}
		owned.CloseWithContext(context)
	}
}
