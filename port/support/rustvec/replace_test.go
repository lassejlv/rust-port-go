// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec_test

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"reflect"
	"testing"
)

func TestReplaceOwnedTransfersDynamicDestructorAndCurrentScope(t *testing.T) {
	type oldValue struct{ data []byte }
	type newValue struct{ data []byte }
	var events []string
	ctx := func(name string) rustfmt.Context {
		return rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: name}}
	}
	a, b := ctx("A"), ctx("B")
	target := rustvec.NewOwned[any](&oldValue{[]byte{1}}, rustvec.ValueOps[any]{Drop: func(v *any, c rustfmt.Context) {
		(*v).(*oldValue).data = append((*v).(*oldValue).data, 2)
		events = append(events, "old:"+c.Runtime.Value.(string))
		c.Panic("old destructor")
	}}).MoveWithContext(a)
	replacement := rustvec.NewOwned[any](&newValue{[]byte{3}}, rustvec.ValueOps[any]{Drop: func(v *any, c rustfmt.Context) {
		events = append(events, "new:"+c.Runtime.Value.(string))
		(*v).(*newValue).data = nil
	}}).MoveWithContext(a)
	cell := target.Borrow()
	oldDescriptor := *target
	var payload any
	func() {
		defer func() { payload = recover() }()
		rustvec.ReplaceOwnedWithContext(target, replacement, b)
	}()
	if payload != "old destructor" || !target.IsLive() || replacement.IsLive() || oldDescriptor.IsLive() || target.Borrow() != cell {
		t.Fatalf("transfer/identity mismatch payload%v target%t replacement%t old%t", payload, target.IsLive(), replacement.IsLive(), oldDescriptor.IsLive())
	}
	if !reflect.DeepEqual((*target.Borrow()).(*newValue).data, []byte{3}) {
		t.Fatal("replacement value changed")
	}
	target.DropWithContext(a)
	if !reflect.DeepEqual(events, []string{"old:B", "new:A"}) {
		t.Fatalf("destructor or ambient scope changed: %v", events)
	}
}
func TestReplaceOwnedRejectsConsumedWithCurrentScope(t *testing.T) {
	var observed []string
	ctx := func(name string) rustfmt.Context {
		c := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: name}}
		return rustfmt.WithPanicOperations(c, &rustfmt.PanicOperations{Hook: func(_ rustfmt.SourcePanicInfo, c rustfmt.Context) {
			observed = append(observed, c.Runtime.Value.(string))
		}})
	}
	a, b := ctx("A"), ctx("B")
	target := rustvec.Own(1).MoveWithContext(a)
	target.DropWithContext(a)
	replacement := rustvec.Own(2)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("consumed target accepted")
			}
		}()
		rustvec.ReplaceOwnedWithContext(target, replacement, b)
	}()
	if !reflect.DeepEqual(observed, []string{"B"}) || !replacement.IsLive() {
		t.Fatalf("invalid call used stale scope or consumed argument: %v", observed)
	}
	replacement.DropWithContext(b)
}
