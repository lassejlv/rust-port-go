// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"reflect"
	"testing"
)

func TestRawIterNextBucketActualCellsTerminalAndCurrentContext(t *testing.T) {
	callbacks := []uint{}
	table := NewRawTableWithCapacity(7, Layout{Size: 8, Align: 8}, DefaultProfile(), ElementOps[uint]{Hash: func(value *uint, _ Context) uint64 { callbacks = append(callbacks, *value); return 0 }}, Context{})
	positions := map[uint]uint{}
	cells := map[uint]*uint{}
	for i := uint(0); i < 7; i++ {
		position, cell := table.InsertUnique(0, i, Context{})
		if *cell != i {
			t.Fatal("insert returned different cell")
		}
		positions[i] = position
		cells[i] = cell // Retained only as an opaque identity after this borrow ends.
	}
	callbacks = nil
	found := map[uint]uint{}
	func() {
		iterator := table.Iter(Context{})
		for {
			position, value, ok := iterator.NextBucket(Context{})
			if !ok {
				if position != 0 || value != nil {
					t.Fatal("terminal value differs")
				}
				break
			}
			if value != cells[*value] {
				t.Fatal("iterator did not borrow actual bucket cell")
			}
			found[*value] = position
		}
		if position, value, ok := iterator.NextBucket(Context{}); ok || position != 0 || value != nil {
			t.Fatal("terminal iterator resumed")
		}
	}()
	if !reflect.DeepEqual(found, positions) || len(callbacks) != 0 {
		t.Fatalf("bucket iteration changed identities/callbacks: %v %v", found, callbacks)
	}
	// The numeric snapshot survives ending the borrow; mutate only after that.
	for value, position := range found {
		cell := table.At(position, Context{})
		*cell = value + 100
	}
	iterator := table.Iter(Context{})
	table.Close(Context{}) // iterator is now intentionally invalid for the rejection witness.
	hook := 0
	ctx := rustfmt.WithPanicOperations(Context{Runtime: &rustfmt.RuntimeMetadata{Value: "current"}}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, processing Context) {
		hook++
		if processing.Runtime.Value != "current" || !processing.ProcessingPanic || info.Payload != "rusthashmap: use of an uninitialized or consumed table" {
			t.Fatal("rejection lost current source context")
		}
	}})
	var payload any
	func() { defer func() { payload = recover() }(); iterator.NextBucket(ctx) }()
	if hook != 1 || payload != "rusthashmap: use of an uninitialized or consumed table" {
		t.Fatalf("consumed iterator accepted or reported wrong context: %d %v", hook, payload)
	}
}
