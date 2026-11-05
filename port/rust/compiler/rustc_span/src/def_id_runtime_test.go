// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"reflect"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// Runtime is host ambient metadata, rather than a cloned source Fn capture.
// This supplements native Fn/panic proofs with the explicit adapter protocol.
func TestDefIdFnRuntimeContext(t *testing.T) {
	a := &rustfmt.RuntimeMetadata{Value: "scope-a"}
	b := &rustfmt.RuntimeMetadata{Value: "scope-b"}
	ctxA := rustfmt.Context{Runtime: a}
	ctxB := rustfmt.Context{Runtime: b}
	var got []string
	observe := func(kind string, ctx rustfmt.Context) {
		scope := "none"
		if ctx.Runtime != nil {
			scope = ctx.Runtime.Value.(string)
		}
		got = append(got, kind+":"+scope)
	}
	ops := DefPathHasherOps[int]{
		Call:    func(r rustvec.Ref[int], id DefId) DefPathHash { observe("call", r.Context); return ZeroDefPathHash() },
		CallMut: func(r rustvec.Ref[int], id DefId) DefPathHash { observe("mut", r.Context); return ZeroDefPathHash() },
		CallOnce: func(s *rustvec.Owned[int], id DefId) DefPathHash {
			observe("once", s.Context())
			return ZeroDefPathHash()
		},
		Drop: func(_ *int, ctx rustfmt.Context) { observe("drop", ctx) },
	}
	id := CrateDefId().ToDefId()
	h := NewDefPathHasherWithContext(1, ops, ctxA)
	h.CallWithContext(id, ctxA)
	h.Call(id)
	h = h.MoveWithContext(ctxB)
	h.CallMutWithContext(id, ctxB)
	h.CallMut(id)
	h = h.Move()
	h.CallOnceWithContext(id, ctxB)
	if h.IsLive() {
		t.Fatal("FnOnce did not consume actual owned state")
	}

	// An incoming Owned snapshot must not become implicit current TLS.
	owned := rustvec.NewOwned(2, rustvec.ValueOps[int]{Drop: ops.Drop}).MoveWithContext(ctxA)
	h = NewOwnedDefPathHasherWithOps(owned, ops)
	h.Call(id)
	h.Close()
	h = NewDefPathHasherWithContext(3, ops, ctxA)
	h.CloseWithContext(ctxB)
	want := []string{"call:scope-a", "call:none", "mut:scope-b", "mut:none", "once:scope-b", "drop:scope-b", "call:none", "drop:none", "drop:scope-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ambient metadata leaked across caller operations:\ngot%v\nwant%v", got, want)
	}
}

type defIdRuntimeProvider struct {
	ops DefPathHasherOps[int]
}

func (p defIdRuntimeProvider) DefPathHasherWithContext(ctx rustfmt.Context) DefPathHasher {
	return NewDefPathHasherWithContext(1, p.ops, ctx)
}
func TestDefIdComparatorRuntimeContext(t *testing.T) {
	var observed []*rustfmt.RuntimeMetadata
	ctxA := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "a"}}
	ctxB := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "b"}}
	ops := DefPathHasherOps[int]{
		Call: func(r rustvec.Ref[int], _ DefId) DefPathHash {
			observed = append(observed, r.Context.Runtime)
			return ZeroDefPathHash()
		},
		Drop: func(_ *int, c rustfmt.Context) { observed = append(observed, c.Runtime) },
	}
	cmp := DefIdStableCmpWithContext(defIdRuntimeProvider{ops}, ctxA)
	id := CrateDefId().ToDefId()
	cmp.Compare(id, id)
	cmp.CompareWithContext(id, id, ctxB)
	cmp = cmp.Move()
	cmp.CallOnceWithContext(id, id, ctxB)
	want := []*rustfmt.RuntimeMetadata{nil, nil, ctxB.Runtime, ctxB.Runtime, ctxB.Runtime, ctxB.Runtime, ctxB.Runtime}
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("comparator used captured ambient metadata: got%v want%v", observed, want)
	}
}

func TestDefIdFnForwardingDoesNotRaiseTwice(t *testing.T) {
	var observed []rustfmt.SourcePanicInfo
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, processing rustfmt.Context) {
		if !processing.ProcessingPanic {
			t.Fatal("hook did not enter source panic processing")
		}
		observed = append(observed, info)
	}})
	info := rustfmt.SourcePanicInfo{Payload: "actual callback panic", Kind: rustfmt.PanicStaticStr, CanUnwind: true}
	h := NewDefPathHasherWithContext(1, DefPathHasherOps[int]{Call: func(r rustvec.Ref[int], _ DefId) DefPathHash { r.Context.PanicAt(info); return ZeroDefPathHash() }}, ctx)
	value, failed := defIDCatch(func() any { return h.CallWithContext(CrateDefId().ToDefId(), ctx) })
	h.CloseWithContext(ctx)
	if !failed || value != "actual callback panic" || len(observed) != 1 || !reflect.DeepEqual(observed[0], info) {
		t.Fatalf("forwarding changed source raise: failed%t payload%v hooks%+v", failed, value, observed)
	}
}
