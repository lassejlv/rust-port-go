// SPDX-License-Identifier: MIT OR Apache-2.0
// Original comments/tests.rs: all five declarations and ten assertions in order.
package comments_test

import (
	rustcast "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_ast/src"
	astutil "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_ast/src/util"
	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"testing"
)

func commentSession(t *testing.T) *span.SessionGlobals {
	t.Helper()
	session := span.NewSessionGlobals(span.DefaultEdition, nil, span.None[span.SourceMapInputs]())
	t.Cleanup(session.Close)
	return session
}
func assertBeautified(t *testing.T, session *span.SessionGlobals, text string, kind rustcast.CommentKind, expected string) {
	t.Helper()
	stripped := astutil.BeautifyDocString(session, span.InternSymbol(session, text), kind)
	if actual := stripped.AsStr(session); actual != expected {
		t.Fatalf("got %q, want %q", actual, expected)
	}
}

func TestBlockDocComment1(t *testing.T) {
	session := commentSession(t)
	comment := "\n * Test \n **  Test\n *   Test\n"
	assertBeautified(t, session, comment, rustcast.CommentKindBlock(), " Test \n*  Test\n   Test")
}
func TestBlockDocComment2(t *testing.T) {
	session := commentSession(t)
	comment := "\n * Test\n *  Test\n"
	assertBeautified(t, session, comment, rustcast.CommentKindBlock(), " Test\n  Test")
}
func TestBlockDocComment3(t *testing.T) {
	session := commentSession(t)
	comment := "\n let a: *i32;\n *a = 5;\n"
	assertBeautified(t, session, comment, rustcast.CommentKindBlock(), "let a: *i32;\n*a = 5;")
}
func TestLineDocComment(t *testing.T) {
	session := commentSession(t)
	assertBeautified(t, session, " test", rustcast.CommentKindLine(), " test")
	assertBeautified(t, session, "! test", rustcast.CommentKindLine(), "! test")
	assertBeautified(t, session, "test", rustcast.CommentKindLine(), "test")
	assertBeautified(t, session, "!test", rustcast.CommentKindLine(), "!test")
}
func TestDocBlocks(t *testing.T) {
	session := commentSession(t)
	assertBeautified(t, session, " # Returns\n     *\n     ", rustcast.CommentKindBlock(), " # Returns\n\n")
	assertBeautified(t, session, "\n     * # Returns\n     *\n     ", rustcast.CommentKindBlock(), " # Returns\n\n")
	assertBeautified(t, session, "\n *     a\n ", rustcast.CommentKindBlock(), "     a\n")
}
