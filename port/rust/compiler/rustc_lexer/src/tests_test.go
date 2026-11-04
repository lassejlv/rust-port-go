// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported in full from rust-lang/rust compiler/rustc_lexer/src/tests.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// Source: 371 lines, 12,250 bytes; SHA-256:
// 90516c7444d6102ab1fceae40cf260cf473ec6542bc372d8bf9d6033eaf507da.
// All 17 upstream tests, 14 shebang assertions, and 9 token snapshots are
// preserved below. Snapshot indentation is removed as by expect_test.
package rustclexer

import (
	"strings"
	"testing"
)

// Upstream: check_raw_str (lines 5-11). Compare RawStrError fields rather
// than their debug formatting, preserving the upstream Result comparison.
func checkRawStr(t *testing.T, input string, wantHashes uint8, wantErr *RawStrError) {
	t.Helper()
	cursor := NewCursor("r"+input, FrontmatterNo)
	cursor.Bump()
	hashes, err := cursor.rawDoubleQuotedString(0)
	if wantErr != nil {
		if err == nil || *err != *wantErr {
			t.Fatalf("raw string %q error = %#v, want %#v", input, err, wantErr)
		}
	} else if err != nil || hashes != wantHashes {
		t.Fatalf("raw string %q = (%d, %#v), want Ok(%d)", input, hashes, err, wantHashes)
	}
}

// Upstream: test_naked_raw_str (lines 13-16).
func TestUpstreamNakedRawStr(t *testing.T) {
	checkRawStr(t, `"abc"`, 0, nil)
}

// Upstream: test_raw_no_start (lines 18-21).
func TestUpstreamRawNoStart(t *testing.T) {
	checkRawStr(t, `"abc"#`, 0, nil)
}

// Upstream: test_too_many_terminators (lines 23-27).
func TestUpstreamTooManyTerminators(t *testing.T) {
	// This error is handled in the parser later.
	checkRawStr(t, `#"abc"##`, 1, nil)
}

// Upstream: test_unterminated (lines 29-48).
func TestUpstreamUnterminated(t *testing.T) {
	checkRawStr(t, `#"abc`, 0, &RawStrError{
		Tag: NoTerminator, Expected: 1, Found: 0,
		HasPossibleTerminatorOffset: false,
	})
	checkRawStr(t, `##"abc"#`, 0, &RawStrError{
		Tag: NoTerminator, Expected: 2, Found: 1,
		PossibleTerminatorOffset: 7, HasPossibleTerminatorOffset: true,
	})
	// We're looking for "# not just any #.
	checkRawStr(t, `##"abc#`, 0, &RawStrError{
		Tag: NoTerminator, Expected: 2, Found: 0,
		HasPossibleTerminatorOffset: false,
	})
}

// Upstream: test_invalid_start (lines 50-53).
func TestUpstreamInvalidStart(t *testing.T) {
	checkRawStr(t, `#~"abc"#`, 0, &RawStrError{Tag: InvalidStarter, BadChar: '~'})
}

// Upstream: test_unterminated_no_pound (lines 55-62).
func TestUpstreamUnterminatedNoPound(t *testing.T) {
	// https://github.com/rust-lang/rust/issues/70677
	checkRawStr(t, `"`, 0, &RawStrError{
		Tag: NoTerminator, Expected: 0, Found: 0,
		HasPossibleTerminatorOffset: false,
	})
}

// Upstream: test_too_many_hashes (lines 64-78).
func TestUpstreamTooManyHashes(t *testing.T) {
	const maxCount = 255 // u8::MAX
	hashes1 := strings.Repeat("#", maxCount)
	hashes2 := strings.Repeat("#", maxCount+1)
	middle := `"abc"`
	s1 := strings.Join([]string{hashes1, middle, hashes1}, "")
	s2 := strings.Join([]string{hashes2, middle, hashes2}, "")

	// Valid number of hashes (255 = 2^8 - 1 = u8::MAX).
	checkRawStr(t, s1, 255, nil)
	// One more hash sign (256 = 2^8) becomes too many.
	checkRawStr(t, s2, 0, &RawStrError{Tag: TooManyDelimiters, Found: maxCount + 1})
}

// Upstream: test_valid_shebang (lines 80-125).
func TestUpstreamValidShebang(t *testing.T) {
	// https://github.com/rust-lang/rust/issues/70528
	for _, tc := range []struct {
		input   string
		length  int
		present bool
	}{
		{"#!/bin/bash", len("#!/bin/bash"), true},
		{"#![attribute]", 0, false},
		{"#!    /bin/bash", len("#!    /bin/bash"), true},
		{"#!    [attribute]", 0, false},
		{"#! /* blah */  /bin/bash", len("#! /* blah */  /bin/bash"), true},
		{"#! /* blah */  [attribute]", 0, false},
		{"#! // blah\n/bin/bash", 10, true}, // Strip up to the newline.
		{"#! // blah\n[attribute]", 0, false},
		{"#! /* blah\nblah\nblah */  /bin/bash", 10, true},
		{"#! /* blah\nblah\nblah */  [attribute]", 0, false},
		{"#!\n/bin/sh", 2, true},
		{"#!\n[attribute]", 0, false},
		// The kernel requires shebangs to be on the first line.
		{"\n#!/bin/bash", 0, false},
		{"\n#![attribute]", 0, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			length, present := StripShebang(tc.input)
			if length != tc.length || present != tc.present {
				t.Errorf("StripShebang(%q) = (%d, %t), want (%d, %t)", tc.input, length, present, tc.length, tc.present)
			}
		})
	}
}

// Upstream: check_lexing (lines 127-131).
func checkLexing(t *testing.T, input string, frontmatter FrontmatterAllowed, expected string) {
	t.Helper()
	var actual strings.Builder
	for _, token := range Tokenize(input, frontmatter) {
		actual.WriteString(token.RustDebug())
		actual.WriteByte('\n')
	}
	if actual.String() != expected {
		t.Fatalf("tokens for %q:\n%s\nwant:\n%s", input, actual.String(), expected)
	}
}

// Upstream: smoke_test (lines 133-160).
func TestUpstreamSmokeTest(t *testing.T) {
	checkLexing(t,
		"/* my source file */ fn main() { println!(\"zebra\"); }\n",
		FrontmatterNo,
		`Token { kind: BlockComment { doc_style: None, terminated: true }, len: 20 }
Token { kind: Whitespace, len: 1 }
Token { kind: Ident, len: 2 }
Token { kind: Whitespace, len: 1 }
Token { kind: Ident, len: 4 }
Token { kind: OpenParen, len: 1 }
Token { kind: CloseParen, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: OpenBrace, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: Ident, len: 7 }
Token { kind: Bang, len: 1 }
Token { kind: OpenParen, len: 1 }
Token { kind: Literal { kind: Str { terminated: true }, suffix_start: 7 }, len: 7 }
Token { kind: CloseParen, len: 1 }
Token { kind: Semi, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: CloseBrace, len: 1 }
Token { kind: Whitespace, len: 1 }
`,
	)
}

// Upstream: comment_flavors (lines 162-199).
func TestUpstreamCommentFlavors(t *testing.T) {
	checkLexing(t,
		"\n// line\n//// line as well\n/// outer doc line\n//! inner doc line\n/* block */\n/**/\n/*** also block */\n/** outer doc block */\n/*! inner doc block */\n",
		FrontmatterNo,
		`Token { kind: Whitespace, len: 1 }
Token { kind: LineComment { doc_style: None }, len: 7 }
Token { kind: Whitespace, len: 1 }
Token { kind: LineComment { doc_style: None }, len: 17 }
Token { kind: Whitespace, len: 1 }
Token { kind: LineComment { doc_style: Some(Outer) }, len: 18 }
Token { kind: Whitespace, len: 1 }
Token { kind: LineComment { doc_style: Some(Inner) }, len: 18 }
Token { kind: Whitespace, len: 1 }
Token { kind: BlockComment { doc_style: None, terminated: true }, len: 11 }
Token { kind: Whitespace, len: 1 }
Token { kind: BlockComment { doc_style: None, terminated: true }, len: 4 }
Token { kind: Whitespace, len: 1 }
Token { kind: BlockComment { doc_style: None, terminated: true }, len: 18 }
Token { kind: Whitespace, len: 1 }
Token { kind: BlockComment { doc_style: Some(Outer), terminated: true }, len: 22 }
Token { kind: Whitespace, len: 1 }
Token { kind: BlockComment { doc_style: Some(Inner), terminated: true }, len: 22 }
Token { kind: Whitespace, len: 1 }
`,
	)
}

// Upstream: nested_block_comments (lines 201-211).
func TestUpstreamNestedBlockComments(t *testing.T) {
	checkLexing(t,
		"/* /* */ */'a'",
		FrontmatterNo,
		`Token { kind: BlockComment { doc_style: None, terminated: true }, len: 11 }
Token { kind: Literal { kind: Char { terminated: true }, suffix_start: 3 }, len: 3 }
`,
	)
}

// Upstream: characters (lines 213-226).
func TestUpstreamCharacters(t *testing.T) {
	checkLexing(t,
		"'a' ' ' '\\n'",
		FrontmatterNo,
		`Token { kind: Literal { kind: Char { terminated: true }, suffix_start: 3 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Char { terminated: true }, suffix_start: 3 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Char { terminated: true }, suffix_start: 4 }, len: 4 }
`,
	)
}

// Upstream: lifetime (lines 228-237).
func TestUpstreamLifetime(t *testing.T) {
	checkLexing(t,
		"'abc",
		FrontmatterNo,
		`Token { kind: Lifetime { starts_with_number: false }, len: 4 }
`,
	)
}

// Upstream: raw_string (lines 239-248).
func TestUpstreamRawString(t *testing.T) {
	checkLexing(t,
		"r###\"\"#a\\b\u0000c\"\"###",
		FrontmatterNo,
		`Token { kind: Literal { kind: RawStr { n_hashes: Some(3) }, suffix_start: 17 }, len: 17 }
`,
	)
}

// Upstream: literal_suffixes (lines 250-296).
func TestUpstreamLiteralSuffixes(t *testing.T) {
	checkLexing(t,
		"\n'a'\nb'a'\n\"a\"\nb\"a\"\n1234\n0b101\n0xABC\n1.0\n1.0e10\n2us\nr###\"raw\"###suffix\nbr###\"raw\"###suffix\n",
		FrontmatterNo,
		`Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Char { terminated: true }, suffix_start: 3 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Byte { terminated: true }, suffix_start: 4 }, len: 4 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Str { terminated: true }, suffix_start: 3 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: ByteStr { terminated: true }, suffix_start: 4 }, len: 4 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Int { base: Decimal, empty_int: false }, suffix_start: 4 }, len: 4 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Int { base: Binary, empty_int: false }, suffix_start: 5 }, len: 5 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Int { base: Hexadecimal, empty_int: false }, suffix_start: 5 }, len: 5 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Float { base: Decimal, empty_exponent: false }, suffix_start: 3 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Float { base: Decimal, empty_exponent: false }, suffix_start: 6 }, len: 6 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Int { base: Decimal, empty_int: false }, suffix_start: 1 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: RawStr { n_hashes: Some(3) }, suffix_start: 12 }, len: 18 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: RawByteStr { n_hashes: Some(3) }, suffix_start: 13 }, len: 19 }
Token { kind: Whitespace, len: 1 }
`,
	)
}

// Upstream: frontmatter_allowed (lines 298-325).
func TestUpstreamFrontmatterAllowed(t *testing.T) {
	checkLexing(t,
		"\n---cargo\n[dependencies]\nclap = \"4\"\n---\n\nfn main() {}\n",
		FrontmatterYes,
		`Token { kind: Whitespace, len: 1 }
Token { kind: Frontmatter { has_invalid_preceding_whitespace: false, invalid_infostring: false }, len: 38 }
Token { kind: Whitespace, len: 2 }
Token { kind: Ident, len: 2 }
Token { kind: Whitespace, len: 1 }
Token { kind: Ident, len: 4 }
Token { kind: OpenParen, len: 1 }
Token { kind: CloseParen, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: OpenBrace, len: 1 }
Token { kind: CloseBrace, len: 1 }
Token { kind: Whitespace, len: 1 }
`,
	)
}

// Upstream: frontmatter_disallowed (lines 327-371).
func TestUpstreamFrontmatterDisallowed(t *testing.T) {
	checkLexing(t,
		"\n---cargo\n[dependencies]\nclap = \"4\"\n---\n\nfn main() {}\n",
		FrontmatterNo,
		`Token { kind: Whitespace, len: 1 }
Token { kind: Minus, len: 1 }
Token { kind: Minus, len: 1 }
Token { kind: Minus, len: 1 }
Token { kind: Ident, len: 5 }
Token { kind: Whitespace, len: 1 }
Token { kind: OpenBracket, len: 1 }
Token { kind: Ident, len: 12 }
Token { kind: CloseBracket, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: Ident, len: 4 }
Token { kind: Whitespace, len: 1 }
Token { kind: Eq, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: Literal { kind: Str { terminated: true }, suffix_start: 3 }, len: 3 }
Token { kind: Whitespace, len: 1 }
Token { kind: Minus, len: 1 }
Token { kind: Minus, len: 1 }
Token { kind: Minus, len: 1 }
Token { kind: Whitespace, len: 2 }
Token { kind: Ident, len: 2 }
Token { kind: Whitespace, len: 1 }
Token { kind: Ident, len: 4 }
Token { kind: OpenParen, len: 1 }
Token { kind: CloseParen, len: 1 }
Token { kind: Whitespace, len: 1 }
Token { kind: OpenBrace, len: 1 }
Token { kind: CloseBrace, len: 1 }
Token { kind: Whitespace, len: 1 }
`,
	)
}
