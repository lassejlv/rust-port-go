// SPDX-License-Identifier: MIT OR Apache-2.0
// Mirrors compiler/rustc_span/src/symbol/tests.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179, in declaration/assertion order.
package symbol_test

import (
	"testing"

	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

func assertSymbol[T comparable](t *testing.T, actual, expected T) {
	t.Helper()
	if actual != expected {
		t.Fatalf("got %v, want %v", actual, expected)
	}
}

// Rust: interner_tests. The explicit witness delegates to the private actual
// interner with the source's empty prefills, not a full predefined session.
func TestInternerTests(t *testing.T) {
	i := span.NewSymbolInternerSourceTestWitness(nil, nil)
	assertSymbol(t, i.InternStr("dog"), span.NewSymbol(0))
	assertSymbol(t, i.InternByteStr([]byte("dog")), span.NewByteSymbol(0))
	assertSymbol(t, i.InternByteStr([]byte("cat")), span.NewByteSymbol(1))
	assertSymbol(t, i.InternStr("cat"), span.NewSymbol(1))
	assertSymbol(t, i.InternStr("dog"), span.NewSymbol(0))
}

// Rust: interner_get.
func TestInternerGet(t *testing.T) {
	i := span.NewSymbolInternerSourceTestWitness([]string{"chicken"}, []string{"cow"})
	dogIndex := i.InternStr("dog")
	catIndex := i.InternStr("cat")
	assertSymbol(t, i.GetStr(span.NewSymbol(0)), "chicken")
	assertSymbol(t, i.GetStr(span.NewSymbol(1)), "cow")
	assertSymbol(t, i.GetStr(catIndex), "cat")
	assertSymbol(t, i.GetStr(dogIndex), "dog")
}

// Rust: without_first_quote_test.
func TestWithoutFirstQuote(t *testing.T) {
	session := span.NewSpanEncodingSession(span.DefaultSpanEncodingConfig())
	i := span.IdentFromStr(session, "'break")
	assertSymbol(t, i.WithoutFirstQuote(session).Name, span.KwBreak())
}
