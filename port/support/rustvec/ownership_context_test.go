package rustvec

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"testing"
)

func TestOwnedRuntimeScopeFollowsExplicitOperation(t *testing.T) {
	first := &rustfmt.RuntimeMetadata{Value: "first operation"}
	second := &rustfmt.RuntimeMetadata{Value: "second operation"}
	var dropped DropContext
	newValue := func() *Owned[int] {
		return NewOwned(7, ValueOps[int]{Drop: func(_ *int, context DropContext) { dropped = context }})
	}
	ctx := DropContext{Unwinding: true, Runtime: first}
	value := newValue().MoveWithContext(ctx)
	if value.Context() != ctx {
		t.Fatal("active guard lost supplied runtime metadata")
	}
	value.Drop()
	if dropped.Runtime != nil || !dropped.Unwinding {
		t.Fatal("convenience Drop retained a prior scope or changed panic guard")
	}
	value = newValue().MoveWithContext(ctx).Move()
	if value.Context().Runtime != nil || !value.Context().Unwinding {
		t.Fatal("convenience Move retained a prior scope")
	}
	value.DropWithContext(DropContext{Runtime: second})
	if dropped.Runtime != second || dropped.Unwinding {
		t.Fatal("DropWithContext did not use current caller context")
	}
	value = newValue().MoveWithContext(ctx)
	result := ConsumeValue(value, DropContext{Runtime: second}, func(arg *Owned[int]) int {
		if arg.Context().Runtime != second {
			t.Fatal("operation callback saw construction-time scope")
		}
		*arg.Borrow() = 42
		return *arg.Borrow()
	})
	if result != 42 || dropped.Runtime != second || value.IsLive() {
		t.Fatal("consuming operation failed ownership/current-scope contract")
	}
}
