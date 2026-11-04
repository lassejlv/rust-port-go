package syntax

import (
	"reflect"
	"strings"
	"testing"
)

func TestRawLexerAdapterCompoundPunctuation(t *testing.T) {
	operators := []string{"<<=", ">>=", "..=", "..", "::", "->", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>"}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			tokens, err := lex(op)
			if err != nil {
				t.Fatal(err)
			}
			if len(tokens) != 2 || tokens[0].kind != op || tokens[0].text != op || tokens[1].span.Offset != len(op) {
				t.Fatalf("tokens = %#v", tokens)
			}
		})
	}
	for _, tc := range []struct {
		source string
		kinds  []string
	}{
		{"</*x*/<=", []string{"<", "<="}},
		{"< /*x*/ < =", []string{"<", "<", "="}},
		{"./*x*/.=", []string{".", ".", "="}},
		{"&//x\n&", []string{"&", "&"}},
		{":/*x*/:", []string{":", ":"}},
		{"- /*x*/ >", []string{"-", ">"}},
		{"/ /*x*/ =", []string{"/", "="}},
		{"+\u200e=", []string{"+", "="}},
		{"1 /*x*/ u8", []string{"int", "ident"}},
		{`foo/*x*/"a"`, []string{"ident", "string"}},
		{"r/*x*/#foo", []string{"ident", "#", "ident"}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			tokens, err := lex(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			var kinds []string
			for _, token := range tokens[:len(tokens)-1] {
				kinds = append(kinds, token.kind)
			}
			if !reflect.DeepEqual(kinds, tc.kinds) {
				t.Fatalf("kinds = %q, want %q", kinds, tc.kinds)
			}
		})
	}
}

func TestRawLexerAdapterUnsupportedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		source, message string
	}{
		{`r#name`, "raw identifiers"},
		{`k#fn`, "forced keyword"},
		{`🦀name`, "invalid identifier containing emoji"},
		{`foo"a"`, "unknown literal prefix"},
		{`foo'a'`, "unknown literal prefix"},
		{`foo#`, "unknown literal prefix"},
		{`'a`, "lifetimes"},
		{`'r#name`, "lifetimes"},
		{`'name#`, "lifetimes"},
		{`'a'`, "character literals"},
		{`b'a'`, "byte literals"},
		{`b"a"`, "byte strings"},
		{`c"a"`, "C strings"},
		{`r"a"`, "raw strings"},
		{`br#"a"#`, "raw strings"},
		{`cr#"a"#`, "raw strings"},
		{`r#~`, "raw strings"},
		{`#"a"#`, "guarded strings"},
		{`##`, "guarded strings"},
		{`1.5`, "floating-point"},
		{`1e+`, "floating-point"},
		{`"a"suffix`, "string literal suffixes"},
		{`"a"_`, "string literal suffixes"},
		{"---cargo\nx\n---\nfn main(){}", "frontmatter"},
		{" \t---\nx\n---", "frontmatter"},
		{"#!/bin/rust\nfn main(){}", "shebangs"},
		{`@`, "unsupported character"},
		{`café`, "Unicode identifiers"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			_, err := lex(tc.source)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error = %v, want %q", err, tc.message)
			}
		})
	}
}

func TestRawLexerAdapterIntegerSpellingAndStrings(t *testing.T) {
	for _, spelling := range []string{"0", "1_000", "0b_10_u8", "0o77u16", "0xA_B_Cu32", "18446744073709551615u64", "9223372036854775808i64"} {
		tokens, err := lex(spelling)
		if err != nil || len(tokens) != 2 || tokens[0].kind != "int" || tokens[0].text != spelling {
			t.Errorf("integer %q: tokens=%#v, err=%v", spelling, tokens, err)
		}
	}
	for _, spelling := range []string{"0x", "0b_", "0o99", "1i128", "1i64i32", "0xffu8x"} {
		if _, err := lex(spelling); err == nil || !strings.Contains(err.Error(), "integer literal") {
			t.Errorf("invalid integer %q: err=%v", spelling, err)
		}
	}
	tokens, err := lex("\"é\r\n東京\\\r\n  z\\n\"+1")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].text != "é\n東京z\n" || tokens[1].kind != "+" || tokens[1].span != (Span{Offset: 20, Line: 3, Column: 7}) {
		t.Fatalf("decoded tokens = %#v", tokens)
	}
	for _, tc := range []struct {
		source, position, message string
	}{
		{`"\u{123" }`, "1:2:", "unterminated Unicode escape"},
		{"\"x\"suffix", "1:4:", "string literal suffixes"},
		{"\"x\ry\"", "1:3:", "bare CR"},
		{"\"x\\\ry\"", "1:3:", "bare CR"},
		{"\"x\r\ny", "1:1:", "unterminated string"},
		{"/*é*/café", "1:9:", "Unicode identifiers"},
		{"//é\n//\xff", "2:3:", "invalid UTF-8"},
	} {
		_, err := lex(tc.source)
		if err == nil || !strings.HasPrefix(err.Error(), tc.position) || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("lex(%q) error=%v, want %s %s", tc.source, err, tc.position, tc.message)
		}
	}
}

func TestRawLexerAdapterWhitespaceAndDocComments(t *testing.T) {
	// Doc comments preserve the frontend's existing skip behavior. This does not
	// implement their attribute/documentation semantics.
	source := "/// outer\n//! inner\n/** outer */ /*! inner */\n\v\f\u0085\u2028\u2029\u200e\u200f fn\tmain(){}"
	program, err := Parse(source)
	if err != nil || len(program.Functions) != 1 || program.Functions[0].Name != "main" {
		t.Fatalf("program=%#v, err=%v", program, err)
	}
	tokens, err := lex("/*é\n東京*/\u200efn")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].span != (Span{Offset: 16, Line: 2, Column: 6}) {
		t.Fatalf("source position = %#v", tokens[0].span)
	}
	for _, source := range []string{"/*", "/* /* */"} {
		if _, err := lex(source); err == nil || !strings.HasPrefix(err.Error(), "1:1:") || !strings.Contains(err.Error(), "unterminated block comment") {
			t.Errorf("block comment %q: %v", source, err)
		}
	}
	if _, err := lex("\u00a0fn"); err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Errorf("non-Pattern_White_Space error = %v", err)
	}
}
