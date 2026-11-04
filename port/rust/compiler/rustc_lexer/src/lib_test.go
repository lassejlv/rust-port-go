// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_lexer/src/tests.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179; additional parity cases below.
package rustclexer

import (
	"reflect"
	"strings"
	"testing"
)

func TestIdentifiersAndPrefixes(t *testing.T) {
	for _, tc := range []struct {
		input string
		tag   TokenTag
		len   uint32
	}{
		{"_", Ident, 1},
		{"東京", Ident, 6},
		{"r#東京", RawIdent, 8},
		{"k#fn", ForcedKeywordIdent, 4},
		{"k#_", ForcedKeywordIdent, 3},
		{"k#1", UnknownPrefix, 1},
		{"hello#", UnknownPrefix, 5},
		{"hello\"x\"", UnknownPrefix, 5},
		{"hello'x'", UnknownPrefix, 5},
		{"hello🦀world", InvalidIdent, 14},
		{"🦀world", InvalidIdent, 9},
		{"🦀#", InvalidIdent, 4},
		{"r#foo🦀", RawIdent, 5},
		{"k#foo🦀", ForcedKeywordIdent, 5},
		{"№", Unknown, 3},
		{"\x00", Unknown, 1},
	} {
		token := NewCursor(tc.input, FrontmatterNo).AdvanceToken()
		if token.Kind.Tag != tc.tag || token.Len != tc.len {
			t.Errorf("first token %q = %s, want %s length %d", tc.input, token.RustDebug(), tc.tag, tc.len)
		}
	}
	for _, tc := range []struct {
		input string
		want  bool
	}{{"", false}, {"_", true}, {"foo_1", true}, {"東京", true}, {"a\u0301", true}, {"\u0301a", false}, {"🦀", false}, {"r#foo", false}, {"1a", false}} {
		if got := IsIdent(tc.input); got != tc.want {
			t.Errorf("IsIdent(%q) = %t, want %t", tc.input, got, tc.want)
		}
	}
}

func TestNumberMetadata(t *testing.T) {
	for _, tc := range []struct {
		input  string
		kind   LiteralKind
		suffix uint32
		length uint32
	}{
		{"0", LiteralKind{Tag: Int, Base: Decimal}, 1, 1},
		{"1f32", LiteralKind{Tag: Int, Base: Decimal}, 1, 4},
		{"0b__u8", LiteralKind{Tag: Int, Base: Binary, EmptyInt: true}, 4, 6},
		{"0o__", LiteralKind{Tag: Int, Base: Octal, EmptyInt: true}, 4, 4},
		{"0x__G", LiteralKind{Tag: Int, Base: Hexadecimal, EmptyInt: true}, 4, 5},
		{"0b120i99", LiteralKind{Tag: Int, Base: Binary}, 5, 8},
		{"0o987", LiteralKind{Tag: Int, Base: Octal}, 5, 5},
		{"0xA_B_Cu8", LiteralKind{Tag: Int, Base: Hexadecimal}, 7, 9},
		{"0_1_2", LiteralKind{Tag: Int, Base: Decimal}, 5, 5},
		{"12.foo()", LiteralKind{Tag: Int, Base: Decimal}, 2, 2},
		{"0..2", LiteralKind{Tag: Int, Base: Decimal}, 1, 1},
		{"1._", LiteralKind{Tag: Int, Base: Decimal}, 1, 1},
		{"1.東京", LiteralKind{Tag: Int, Base: Decimal}, 1, 1},
		{"1.", LiteralKind{Tag: Float, Base: Decimal}, 2, 2},
		{"1.🦀", LiteralKind{Tag: Float, Base: Decimal}, 2, 2},
		{"1.0e+__f64", LiteralKind{Tag: Float, Base: Decimal, EmptyExponent: true}, 7, 10},
		{"1e-", LiteralKind{Tag: Float, Base: Decimal, EmptyExponent: true}, 3, 3},
		{"0e_2f32", LiteralKind{Tag: Float, Base: Decimal}, 4, 7},
		{"0b1.5e2", LiteralKind{Tag: Float, Base: Binary}, 7, 7},
		{"0x1.8", LiteralKind{Tag: Float, Base: Hexadecimal}, 5, 5},
	} {
		token := NewCursor(tc.input, FrontmatterNo).AdvanceToken()
		want := Token{Kind: literal(tc.kind, tc.suffix), Len: tc.length}
		if token != want {
			t.Errorf("first token %q = %s, want %s", tc.input, token.RustDebug(), want.RustDebug())
		}
	}
}

func TestLifetimesAndCharacterRecovery(t *testing.T) {
	for _, tc := range []struct {
		input  string
		kind   TokenKind
		length uint32
	}{
		{"'r#foo", simple(RawLifetime), 6},
		{"'foo#", simple(UnknownPrefixLifetime), 4},
		{"'0foo#", TokenKind{Tag: Lifetime, StartsWithNumber: true}, 5},
		{"'🦀a", TokenKind{Tag: Lifetime, StartsWithNumber: true}, 6},
		{"'a🦀", TokenKind{Tag: Lifetime, StartsWithNumber: true}, 6},
		{"'🦀#", simple(UnknownPrefixLifetime), 5},
		{"'0🦀#", TokenKind{Tag: Lifetime, StartsWithNumber: true}, 6},
		{"'ab'suffix", literal(LiteralKind{Tag: Char, Terminated: true}, 4), 4},
		{"'a'suffix", literal(LiteralKind{Tag: Char, Terminated: true}, 3), 9},
		{"'''", literal(LiteralKind{Tag: Char, Terminated: true}, 3), 3},
		{"'/'", literal(LiteralKind{Tag: Char, Terminated: true}, 3), 3},
		{"' /rest", literal(LiteralKind{Tag: Char}, 2), 2},
		{"' \nrest", literal(LiteralKind{Tag: Char}, 2), 2},
		{"' \n'", literal(LiteralKind{Tag: Char, Terminated: true}, 4), 4},
		{"'\x00'", literal(LiteralKind{Tag: Char, Terminated: true}, 3), 3},
		{"'\\", literal(LiteralKind{Tag: Char}, 2), 2},
		{"b'ab'suffix", literal(LiteralKind{Tag: Byte, Terminated: true}, 5), 11},
		{"c'a'", simple(UnknownPrefix), 1},
	} {
		token := NewCursor(tc.input, FrontmatterNo).AdvanceToken()
		want := Token{Kind: tc.kind, Len: tc.length}
		if token != want {
			t.Errorf("first token %q = %s, want %s", tc.input, token.RustDebug(), want.RustDebug())
		}
	}
}

func TestStringMetadataAndRawValidation(t *testing.T) {
	for _, tc := range []struct {
		input          string
		kind           LiteralKind
		suffix, length uint32
	}{
		{`c"a"suffix`, LiteralKind{Tag: CStr, Terminated: true}, 4, 10},
		{`cr#"a"#suffix`, LiteralKind{Tag: RawCStr, HasNHashes: true, NHashes: 1}, 7, 13},
		{`br"a"suffix`, LiteralKind{Tag: RawByteStr, HasNHashes: true}, 5, 11},
		{`r"a"suffix`, LiteralKind{Tag: RawStr, HasNHashes: true}, 4, 10},
		{`r#~suffix`, LiteralKind{Tag: RawStr}, 3, 3},
		{`r#`, LiteralKind{Tag: RawStr}, 2, 2},
		{`br#~suffix`, LiteralKind{Tag: RawByteStr}, 4, 4},
		{`cr#~suffix`, LiteralKind{Tag: RawCStr}, 4, 4},
		{`b"unterminated`, LiteralKind{Tag: ByteStr}, 14, 14},
		{`c"unterminated`, LiteralKind{Tag: CStr}, 14, 14},
		{`"a\"b"_`, LiteralKind{Tag: Str, Terminated: true}, 6, 7},
		{`r###"a"##`, LiteralKind{Tag: RawStr}, 9, 9},
	} {
		token := NewCursor(tc.input, FrontmatterNo).AdvanceToken()
		want := Token{Kind: literal(tc.kind, tc.suffix), Len: tc.length}
		if token != want {
			t.Errorf("first token %q = %s, want %s", tc.input, token.RustDebug(), want.RustDebug())
		}
	}
	for _, tc := range []struct {
		input  string
		prefix uint32
		want   *RawStrError
	}{
		{`r"a"`, 1, nil},
		{`br#"a"#`, 2, nil},
		{`cr#"a"#`, 2, nil},
		{`r#`, 1, &RawStrError{Tag: InvalidStarter}},
		{`r#~`, 1, &RawStrError{Tag: InvalidStarter, BadChar: '~'}},
		{`r##"abc"#`, 1, &RawStrError{Tag: NoTerminator, Expected: 2, Found: 1, PossibleTerminatorOffset: 8, HasPossibleTerminatorOffset: true}},
		{`br##"é"#`, 2, &RawStrError{Tag: NoTerminator, Expected: 2, Found: 1, PossibleTerminatorOffset: 8, HasPossibleTerminatorOffset: true}},
	} {
		if got := ValidateRawStr(tc.input, tc.prefix); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ValidateRawStr(%q,%d) = %#v, want %#v", tc.input, tc.prefix, got, tc.want)
		}
	}
	hashes := strings.Repeat("#", 256)
	input := "r" + hashes + `"x"` + hashes + "suffix"
	token := NewCursor(input, FrontmatterNo).AdvanceToken()
	if token.Len != 516 || token.Kind.Literal.HasNHashes || token.Kind.SuffixStart != 516 {
		t.Errorf("too-many-hashes token = %s", token.RustDebug())
	}
	if got := ValidateRawStr("r"+hashes+`"x`, 1); got.Tag != NoTerminator || got.Expected != 256 {
		t.Errorf("unterminated raw error takes priority over hash count: %#v", got)
	}
}

func TestFrontmatterRecovery(t *testing.T) {
	for _, tc := range []struct {
		input, consumed        string
		badWhitespace, badInfo bool
	}{
		{"---Cargo.toml\nx\n---\na", "---Cargo.toml\nx\n---", false, false},
		{" \t---cargo\nx\n---\na", " \t---cargo\nx\n---", true, false},
		{"---cargo !\nx\n---\na", "---cargo !\nx\n---", false, true},
		{"---", "---", false, true},
		{"---\nx\n--- extra\na", "---\nx\n--- extra", false, false},
		{"---\nx\n----\na", "---\nx\n----", false, false},
		{"----\nx\n---\na", "----\nx\n---", false, false},
		{"---\nx\nuse foo;\na", "---\nx", false, false},
		{"---\nx\n//! doc\na", "---\nx", false, false},
		{"---\nx\n#![allow(a)]\na", "---\nx", false, false},
		{"---\nx\n  ---\na", "---\nx\n  ---", false, false},
		{"---\nx---z\n  ---\na", "---\nx---z\n  ---", false, false},
		{"---\nno closing", "---\nno closing", false, false},
	} {
		token := NewCursor(tc.input, FrontmatterYes).AdvanceToken()
		want := Token{Kind: TokenKind{Tag: Frontmatter, HasInvalidPrecedingWhitespace: tc.badWhitespace, InvalidInfostring: tc.badInfo}, Len: uint32(len(tc.consumed))}
		if token != want {
			t.Errorf("first token %q = %s, want %s", tc.input, token.RustDebug(), want.RustDebug())
		}
	}
	for _, input := range []string{"// comment\n---\nx\n---", "x\n---\nx\n---", "---\nx\n---\n---\ny\n---"} {
		count := 0
		for _, token := range Tokenize(input, FrontmatterYes) {
			if token.Kind.Tag == Frontmatter {
				count++
			}
		}
		want := 0
		if strings.HasPrefix(input, "---") {
			want = 1
		}
		if count != want {
			t.Errorf("frontmatter count %q = %d, want %d", input, count, want)
		}
	}
}

func TestGuardedStrings(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  GuardedStr
		found bool
		rest  string
	}{
		{`#"abc"#`, GuardedStr{NHashes: 1, Terminated: true, TokenLen: 7}, true, ""},
		{`###"abc"####`, GuardedStr{NHashes: 3, Terminated: true, TokenLen: 11}, true, "#"},
		{`##"a"suffix`, GuardedStr{NHashes: 2, Terminated: true, TokenLen: 11}, true, ""},
		{`#"abc`, GuardedStr{NHashes: 1, TokenLen: 5}, true, ""},
		{`"a"`, GuardedStr{Terminated: true, TokenLen: 3}, true, ""},
		{`###abc`, GuardedStr{}, false, "abc"},
	} {
		cursor := NewCursor(tc.input, FrontmatterNo)
		got, found := cursor.GuardedDoubleQuotedString()
		if got != tc.want || found != tc.found || cursor.AsStr() != tc.rest {
			t.Errorf("guarded %q = (%#v,%t,%q), want (%#v,%t,%q)", tc.input, got, found, cursor.AsStr(), tc.want, tc.found, tc.rest)
		}
		if found && cursor.PosWithinToken() != 0 {
			t.Errorf("guarded token did not reset position")
		}
	}
	tokens := Tokenize(`##"a"#`, FrontmatterNo)
	if tokens[0] != (Token{Kind: simple(GuardedStrPrefix), Len: 2}) {
		t.Errorf("guarded prefix = %s", tokens[0].RustDebug())
	}
}

func TestWhitespaceAndPunctuation(t *testing.T) {
	space := "\n\v\f\r\u0085\u2028\u2029\u200e\u200f\t "
	tokens := Tokenize(space+"\u00a0", FrontmatterNo)
	if len(tokens) != 2 || tokens[0] != (Token{Kind: simple(Whitespace), Len: uint32(len(space))}) || tokens[1].Kind.Tag != Unknown {
		t.Errorf("Pattern_White_Space tokens = %#v", tokens)
	}
	punctuation := ";,.(){}[]@#~?:$=!<>-&|+*/^%"
	tags := []TokenTag{Semi, Comma, Dot, OpenParen, CloseParen, OpenBrace, CloseBrace, OpenBracket, CloseBracket, At, Pound, Tilde, Question, Colon, Dollar, Eq, Bang, Lt, Gt, Minus, And, Or, Plus, Star, Slash, Caret, Percent}
	tokens = Tokenize(punctuation, FrontmatterNo)
	if len(tokens) != len(tags) {
		t.Fatalf("punctuation count = %d, want %d", len(tokens), len(tags))
	}
	for i, tag := range tags {
		if tokens[i] != (Token{Kind: simple(tag), Len: 1}) {
			t.Errorf("punctuation %d = %s", i, tokens[i].RustDebug())
		}
	}
	for _, input := range []string{"/*", "/* /* */"} {
		token := NewCursor(input, FrontmatterNo).AdvanceToken()
		if token.Kind.Tag != BlockComment || token.Kind.Terminated || token.Len != uint32(len(input)) {
			t.Errorf("unterminated block %q = %s", input, token.RustDebug())
		}
	}
	for _, tc := range []struct {
		input string
		n     int
		ok    bool
	}{{"#!", 2, true}, {"#!\r", 3, true}, {"#!\r\nx", 2, true}, {"#!//x\r\ny", 5, true}, {"#! ///doc\n[x]", 9, true}, {"#! /**doc*/ [x]", 15, true}} {
		if n, ok := StripShebang(tc.input); n != tc.n || ok != tc.ok {
			t.Errorf("shebang %q = (%d,%t), want (%d,%t)", tc.input, n, ok, tc.n, tc.ok)
		}
	}
}

func TestCursorAndUTF8Contract(t *testing.T) {
	c := NewCursor("é\x00東京", FrontmatterNo)
	if c.First() != 'é' || c.Second() != 0 || c.Third() != '東' || c.IsEOF() {
		t.Fatal("cursor peeking")
	}
	if r, ok := c.Bump(); r != 'é' || !ok || c.Prev() != 'é' || c.PosWithinToken() != 2 {
		t.Fatal("cursor byte position")
	}
	snapshot := *c
	c.BumpBytes(1)
	if c.First() != '東' || c.PosWithinToken() != 3 || snapshot.First() != 0 {
		t.Fatal("cursor snapshot/byte advance")
	}
	c.ResetPosWithinToken()
	c.EatUntil('!')
	if !c.IsEOF() || c.PosWithinToken() != 6 {
		t.Fatal("cursor EatUntil")
	}
	for i := 0; i < 2; i++ {
		if c.AdvanceToken() != (Token{Kind: simple(Eof)}) {
			t.Fatal("EOF token")
		}
	}
	for _, f := range []func(){
		func() { NewCursor("\xff", FrontmatterNo) },
		func() { Tokenize("\xff", FrontmatterNo) },
		func() { StripShebang("\xff") },
		func() { IsIdent("\xff") },
		func() { ValidateRawStr("\xff", 1) },
		func() { ValidateRawStr("", 1) },
		func() { ValidateRawStr("r", 2) },
		func() { NewCursor("é", FrontmatterNo).BumpBytes(1) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected panic for invalid contract")
				}
			}()
			f()
		}()
	}
}
