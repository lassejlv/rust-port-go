// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"strings"
	"testing"
)

func TestIndexMapUnconstrainedConstructorAndUnavailableTraits(t *testing.T) {
	type state struct{ data [][]byte }
	var dropped *state
	var hookScope string
	a := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "A"}}
	b := rustfmt.WithPanicOperations(rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "B"}}, &rustfmt.PanicOperations{Hook: func(_ rustfmt.SourcePanicInfo, c rustfmt.Context) { hookScope = c.Runtime.Value.(string) }})
	owner := rustvec.NewOwned(state{[][]byte{{1, 2}}}, rustvec.ValueOps[state]{Drop: func(s *state, c rustfmt.Context) {
		if c.Runtime.Value != "B" {
			t.Fatal("stale Drop scope")
		}
		dropped = s
	}}).MoveWithContext(a)
	cell := owner.Borrow()
	builder := im.NewOwnedBuilder(owner, im.BuilderOps[state]{}, b)
	config := im.Config[[]byte, []byte]{BucketLayout: im.Layout{Size: 56, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: im.KeyOps[[]byte]{KeyOps: rusthashmap.KeyOps[[]byte]{Clone: func(s *[]byte, _ im.Context) []byte { return append([]byte{}, (*s)...) }}}, Values: im.ValueOps[[]byte]{ValueOps: rusthashmap.ValueOps[[]byte]{Clone: func(s *[]byte, _ im.Context) []byte { return append([]byte{}, (*s)...) }}}}
	m := im.WithCapacityAndHasher(7, config, builder, b)
	if owner.IsLive() || builder.Borrow(b) != cell || m.Len(b) != 0 || m.Capacity(b) != 7 || m.Iter(b).Count(b) != 0 {
		t.Fatal("constructor required unused traits or copied actual state")
	}
	for _, call := range []func(){func() { m.Clone(b) }, func() { builder.Build(b) }} {
		func() {
			defer func() {
				p := recover()
				if p == nil || !strings.Contains(p.(string), "unavailable source") || hookScope != "B" {
					t.Fatalf("unavailable trait/ambient scope: %v %s", p, hookScope)
				}
			}()
			call()
		}()
	}
	m.Close(b)
	if dropped != cell {
		t.Fatal("Drop did not receive actual S")
	}
}
