// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"testing"
)

// The full original Symbol oracle supplies the named native location profile.
// This regression checks the adapter's separate obligation: an absent profile
// is unlocated, and a caller-supplied spelling is passed through verbatim.
func TestIdentConstructorPanicLocationProfiles(t *testing.T) {
	cases := []struct {
		name, site string
		call       func(*SessionGlobals, rustfmt.Context)
	}{
		{"dummy", PanicIdentDummySpanSite, func(s *SessionGlobals, c rustfmt.Context) { NewIdentWithDummySpanWithContext(s, Sym_empty(), c) }},
		{"from", PanicIdentDummySpanSite, func(s *SessionGlobals, c rustfmt.Context) { IdentFromStrWithContext(s, "", c) }},
		{"from_span", PanicIdentFromSpanSite, func(s *SessionGlobals, c rustfmt.Context) { IdentFromStrAndSpanWithContext(s, "", DummySpan(), c) }},
		{"spanpos", PanicIdentSpanPosSite, func(s *SessionGlobals, c rustfmt.Context) {
			(Ident{Name: Sym_empty()}).WithSpanPosWithContext(s, DummySpan(), c)
		}},
		{"quote", PanicIdentQuoteSite, func(s *SessionGlobals, c rustfmt.Context) {
			(Ident{Name: InternSymbol(s, "'")}).WithoutFirstQuoteWithContext(s, c)
		}},
		{"macro20", PanicIdentMacros20Site, func(s *SessionGlobals, c rustfmt.Context) {
			(Ident{Name: Sym_empty()}).NormalizeToMacros20WithContext(s, c)
		}},
		{"rules", PanicIdentMacroRulesSite, func(s *SessionGlobals, c rustfmt.Context) {
			(Ident{Name: Sym_empty()}).NormalizeToMacroRulesWithContext(s, c)
		}},
		{"normalizer", PanicIdentMacroRulesSite, func(s *SessionGlobals, c rustfmt.Context) {
			NewMacroRulesNormalizedIdentWithContext(s, Ident{Name: Sym_empty()}, c)
		}},
	}
	for _, tc := range cases {
		for _, emitted := range []bool{false, true} {
			for _, profile := range []bool{false, true} {
				t.Run(tc.name+"/"+map[bool]string{false: "fallback", true: "emitted"}[emitted]+"/"+map[bool]string{false: "unlocated", true: "profile"}[profile], func(t *testing.T) {
					s := NewSessionGlobals(Edition2024, nil, None[SourceMapInputs](), SpanEncodingConfig{DebugAssertions: true})
					defer s.Close()
					var hooks []rustfmt.SourcePanicInfo
					var diagnostic SourceLocation
					old := SwapEmitBugDiagnostic(func(_ *SessionGlobals, _ Option[Span], _ SourceFormatArguments, loc SourceLocation, _ rustfmt.Context) bool {
						diagnostic = loc
						return emitted
					})
					defer SwapEmitBugDiagnostic(old)
					ops := &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) { hooks = append(hooks, info) }}
					actual := rustfmt.SourceLocation{File: "selected-build/./original/compiler/rustc_span/src/symbol.rs", Line: 73, Column: 19}
					if profile {
						ops.Locations = map[string]rustfmt.SourceLocation{tc.site: actual}
					}
					c := rustfmt.WithPanicOperations(rustfmt.Context{}, ops)
					var payload any
					func() { defer func() { payload = recover() }(); tc.call(s, c) }()
					if payload == nil || len(hooks) != 1 {
						t.Fatalf("panic=%v hooks=%d", payload, len(hooks))
					}
					if profile {
						if hooks[0].Location == nil || *hooks[0].Location != actual || diagnostic.Host || diagnostic.File != actual.File || diagnostic.Line != actual.Line || diagnostic.Column != actual.Column {
							t.Fatalf("selected location changed: hook=%+v diagnostic=%+v", hooks[0].Location, diagnostic)
						}
					} else if hooks[0].Location != nil || !diagnostic.Host {
						t.Fatalf("absent source profile invented exact location: hook=%+v diagnostic=%+v", hooks[0].Location, diagnostic)
					}
					if emitted {
						if _, ok := payload.(ExplicitBug); !ok || hooks[0].Kind != rustfmt.PanicOpaque {
							t.Fatalf("wrong explicit bug payload:%T/%s", payload, hooks[0].Kind)
						}
					} else if payload != "called `Ident::new` with empty symbol" || hooks[0].Kind != rustfmt.PanicOwnedString {
						t.Fatalf("wrong fallback payload:%v/%s", payload, hooks[0].Kind)
					}
				})
			}
		}
	}
}
