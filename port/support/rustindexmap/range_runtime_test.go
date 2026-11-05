// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"reflect"
	"testing"
)

func TestIndexMapRangeActualBorrowDropAndCurrentPanicProfile(t *testing.T) {
	type state struct {
		bound  uint
		data   [][]byte
		starts uint
	}
	a := im.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "A"}}
	events := []string{}
	location := &rustfmt.SourceLocation{File: "caller-profile.rs", Line: 31, Column: 9}
	b := rustfmt.WithPanicOperations(im.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "B"}}, &rustfmt.PanicOperations{LocationCells: map[string]*rustfmt.SourceLocation{im.IndexMapRangeStartPanicSite: location}, Hook: func(info rustfmt.SourcePanicInfo, c im.Context) {
		if info.Location != location || info.Kind != rustfmt.PanicOwnedString || c.Runtime.Value != "B" || !c.ProcessingPanic {
			t.Fatal("range diagnostic lost current caller/profile cell")
		}
		events = append(events, "hook")
	}})
	owner := rustvec.NewOwned(state{bound: 1, data: [][]byte{{2, 3}}}, rustvec.ValueOps[state]{Drop: func(s *state, c im.Context) {
		if c.Runtime.Value != "B" || !c.Unwinding || s.starts != 1 || !reflect.DeepEqual(s.data, [][]byte{{2, 3}}) {
			t.Fatal("range Drop lost actual state/current unwind context")
		}
		events = append(events, "drop")
	}}).MoveWithContext(a)
	cell := owner.Borrow()
	r := im.NewOwnedRange(owner, im.RangeOps[state]{StartBound: func(s rustvec.Ref[state]) im.Bound {
		if s.Value != cell || s.Context.Runtime.Value != "B" {
			t.Fatal("start callback copied state or retained constructor context")
		}
		s.Value.starts++
		events = append(events, "start")
		return im.Bound{Kind: im.Included, Value: &s.Value.bound}
	}, EndBound: func(rustvec.Ref[state]) im.Bound { t.Fatal("invalid start must not call end_bound"); return im.Bound{} }}, a)
	builder := im.NewOwnedBuilder(rustvec.Own(struct{}{}), im.BuilderOps[struct{}]{}, b)
	m := im.WithHasher(im.Config[[]byte, []byte]{BucketLayout: im.Layout{Size: 56, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true}, builder, b)
	var payload any
	func() { defer func() { payload = recover() }(); m.DrainOwned(r, b) }()
	if payload != "range start index 1 out of range for slice of length 0" || !reflect.DeepEqual(events, []string{"start", "hook", "drop"}) || owner.IsLive() || m.Len(b) != 0 {
		t.Fatalf("range callback/raise/cleanup: %v %v", payload, events)
	}
	m.Close(b)
}
