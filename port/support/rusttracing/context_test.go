// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"testing"
)

func TestPanicAndTracingRuntimeMetadataCompose(t *testing.T) {
	scope := NewScope(nil)
	defer scope.Close(rustfmt.Context{})
	ops := &rustfmt.PanicOperations{}
	profile := &struct{ File string }{File: "selected/verbatim/source.rs"}
	base := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Profiles: map[string]any{"selected": profile}}}
	first := AttachScope(rustfmt.WithPanicOperations(base, ops), scope)
	second := rustfmt.WithPanicOperations(AttachScope(base, scope), ops)
	for _, ctx := range []rustfmt.Context{first, second} {
		if ScopeFromContext(ctx) != scope || ctx.Runtime.Panic != ops || ctx.Runtime.Profiles["selected"] != profile {
			t.Fatal("runtime attachment erased another borrowed operation")
		}
		if AttachScope(ctx, scope) != ctx || rustfmt.WithPanicOperations(ctx, ops) != ctx {
			t.Fatal("unchanged attachment replaced the active runtime pointer")
		}
	}
}
