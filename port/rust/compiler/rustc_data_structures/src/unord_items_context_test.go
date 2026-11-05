// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"reflect"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// The source has operation-local TLS. The translated descriptors receive that
// current scope explicitly, and must not retain the scope which created or
// cloned their persistent cell. The native Bag/Index A/B/no-observer proof
// covers the underlying scope law; this also exercises the erased item/Fn bridge.
func TestUnordItemsContextIsOperationLocal(t *testing.T) {
	var events []string
	current := func(c rustfmt.Context) string {
		if c.Runtime == nil {
			return "none"
		}
		return c.Runtime.Value.(string)
	}
	a := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "A"}}
	b := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "B"}}
	drop := func(_ *uint, c rustfmt.Context) { events = append(events, "drop:"+current(c)) }
	source := NewUnordItems(rustvec.NewOwned(uint(3), rustvec.ValueOps[uint]{Drop: drop}).MoveWithContext(a), UnordIteratorOps[uint, uint]{
		Next:     func(rustvec.Ref[uint]) *rustvec.Owned[uint] { return nil },
		SizeHint: func(rustvec.Ref[uint]) rustvec.Hint { return rustvec.Hint{} },
		Count: func(state *rustvec.Owned[uint], c rustfmt.Context) uint {
			events = append(events, "count:"+current(c))
			return *state.Borrow()
		},
		Clone: func(ref rustvec.Ref[uint]) *rustvec.Owned[uint] {
			events = append(events, "clone:"+current(ref.Context))
			return rustvec.NewOwned(*ref.Value, rustvec.ValueOps[uint]{Drop: drop}).MoveWithContext(a)
		},
		CloneItem: true,
	})
	clone := source.Clone(b)
	if got := clone.Count(); got != 3 {
		t.Fatal(got)
	}
	if got := source.Count(b); got != 3 {
		t.Fatal(got)
	}
	if want := []string{"clone:B", "count:none", "drop:none", "count:B", "drop:B"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("scope flow %v want %v", events, want)
	}
}

func TestUnordFunctionContextIsOperationLocal(t *testing.T) {
	var events []string
	current := func(c rustfmt.Context) string {
		if c.Runtime == nil {
			return "none"
		}
		return c.Runtime.Value.(string)
	}
	a := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "A"}}
	b := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "B"}}
	drop := func(_ *uint, c rustfmt.Context) { events = append(events, "drop:"+current(c)) }
	body := func(ref rustvec.Ref[uint], _ *rustvec.Owned[uint]) *rustvec.Owned[bool] {
		events = append(events, "call:"+current(ref.Context))
		*ref.Value++
		return rustvec.Own(true)
	}
	source := NewUnordFunction(rustvec.NewOwned(uint(0), rustvec.ValueOps[uint]{Drop: drop}).MoveWithContext(a), UnordFunctionOps[uint, uint, bool]{
		Call: body, CallMut: body,
		CallOnce: func(state *rustvec.Owned[uint], arg *rustvec.Owned[uint], c rustfmt.Context) *rustvec.Owned[bool] {
			return body(rustvec.Ref[uint]{Value: state.Borrow(), Context: c}, arg)
		},
		Clone: func(ref rustvec.Ref[uint]) *rustvec.Owned[uint] {
			events = append(events, "clone:"+current(ref.Context))
			return rustvec.NewOwned(*ref.Value, rustvec.ValueOps[uint]{Drop: drop}).MoveWithContext(a)
		},
	})
	source.CallMut(rustvec.Own(uint(1)), b).Take()
	clone := source.Clone(b)
	clone.Call(rustvec.Own(uint(2)), rustfmt.Context{}).Take()
	source.Close(rustfmt.Context{})
	clone.Close(b)
	if want := []string{"call:B", "clone:B", "call:none", "drop:none", "drop:B"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("scope flow %v want %v", events, want)
	}
}
