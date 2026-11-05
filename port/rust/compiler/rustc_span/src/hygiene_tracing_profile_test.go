// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"testing"
)

func TestHygieneTracingProfileContextComposition(t *testing.T) {
	profile := hygieneTraceOracleSelectedProfile(t)
	marker := new(int)
	ops := &rustfmt.PanicOperations{}
	base := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: marker, Profiles: map[string]any{"retained": marker}}}
	attached := WithHygieneTracingProfile(base, profile)
	if attached.Runtime.Value != marker || attached.Runtime.Profiles["retained"] != marker || base.Runtime.Profiles[hygieneTracingProfileKey] != nil {
		t.Fatal("profile attachment replaced current payload or mutated borrowed metadata")
	}
	scope := rusttracing.NewScope(nil)
	defer scope.Close(rustfmt.Context{})
	first := rusttracing.AttachScope(rustfmt.WithPanicOperations(attached, ops), scope)
	second := rustfmt.WithPanicOperations(rusttracing.AttachScope(attached, scope), ops)
	for _, ctx := range []rustfmt.Context{first, second} {
		if rusttracing.ScopeFromContext(ctx) != scope || ctx.Runtime.Value != scope || ctx.Runtime.Panic != ops || ctx.Runtime.Profiles[hygieneTracingProfileKey] != profile || ctx.Runtime.Profiles["retained"] != marker {
			t.Fatal("current scope/panic attachment erased a borrowed selected profile")
		}
		if hygieneSelectedTraceCallsite(ctx, hygieneTraceMarks) != profile.sites[635] {
			t.Fatal("selected callsite identity was not retained")
		}
	}
	if profile.sites[635].Metadata.Name == hygieneTraceMarks.Metadata.Name || profile.sites[635].Metadata.File == hygieneTraceMarks.Metadata.File {
		t.Fatal("verbatim native metadata was silently projected to the logical convenience")
	}
	logical := WithHygieneTracingProfile(attached, LogicalHygieneTracingProfile())
	if hygieneSelectedTraceCallsite(logical, hygieneTraceMarks) != hygieneTraceMarks || LogicalHygieneTracingProfile().Name() != "logical-compiler-path" {
		t.Fatal("explicit logical convenience lost its own static identity")
	}
}
