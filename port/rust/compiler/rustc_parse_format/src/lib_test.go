package parseformat

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPinnedStandardCharacterBehavior(t *testing.T) {
	path := filepath.Join(oracleRoot(t), "tools", "port-rust-parse-format", "char-unicode", "oracle.bin.gz")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(io.LimitReader(reader, 9+0x110000+1))
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if len(data) != 9+0x110000 || !bytes.Equal(data[:9], []byte{'P', 'F', 'C', 'U', '2', 0, 17, 0, 0}) {
		t.Fatal("wrong standard-character oracle profile")
	}
	whitespace := 0
	scalars := 0
	for c := rune(0); c <= utf8.MaxRune; c++ {
		if !utf8.ValidRune(c) {
			continue
		}
		scalars++
		flag := data[9+int(c)]
		expected := string(c)
		if flag&1 != 0 {
			expected = fmt.Sprintf(`\u{%x}`, c)
		} else {
			switch c {
			case '\x00':
				expected = `\0`
			case '\t':
				expected = `\t`
			case '\n':
				expected = `\n`
			case '\r':
				expected = `\r`
			case '\\':
				expected = `\\`
			case '\'':
				expected = `\'`
			case '"':
				expected = `\"`
			}
		}
		if got := escapeDebug(c); got != expected {
			t.Fatalf("escape_debug U+%04X=%q, want %q", c, got, expected)
		}
		characterExpected := expected
		if c == '"' {
			characterExpected = string(c)
		}
		if got := debugRune(c); got != "'"+characterExpected+"'" {
			t.Fatalf("character Debug U+%04X=%q", c, got)
		}
		stringExpected := expected
		if c == '\'' {
			stringExpected = string(c)
		}
		if got := debugString(string(c)); got != "\""+stringExpected+"\"" {
			t.Fatalf("leading string Debug U+%04X=%q", c, got)
		}
		followingExpected := stringExpected
		if flag&4 != 0 {
			followingExpected = fmt.Sprintf(`\u{%x}`, c)
		} else if flag&1 != 0 {
			followingExpected = string(c)
		}
		if got := debugString("A" + string(c)); got != "\"A"+followingExpected+"\"" {
			t.Fatalf("non-leading string Debug U+%04X=%q, want %q", c, got, "\"A"+followingExpected+"\"")
		}

		if got := isWhitespace(c); got != (flag&2 != 0) {
			t.Fatalf("whitespace U+%04X=%v, want %v", c, got, flag&2 != 0)
		}
		if flag&2 != 0 {
			whitespace++
		}
	}
	if scalars != 1112064 || whitespace != 25 {
		t.Fatalf("wrong scalar/property counts %d/%d", scalars, whitespace)
	}
}

func TestDerivedCloneEqualityAndIndices(t *testing.T) {
	for _, position := range []Position{{Kind: ArgumentImplicitlyIs, Index: 7}, {Kind: ArgumentIs, Index: 9}, {Kind: ArgumentNamed, Name: "name"}} {
		if !position.Equal(position.Clone()) {
			t.Fatal("Position Clone/PartialEq differs")
		}
		index := position.IndexValue()
		if index.Present != (position.Kind != ArgumentNamed) || (index.Present && index.Value != position.Index) {
			t.Fatal("Position index method differs")
		}
	}
	counts := []Count{{}, {Kind: CountIs, Value: 42}, {Kind: CountIsName, Name: "width", Span: Range{3, 8}}, {Kind: CountIsParam, Index: 4}, {Kind: CountIsStar, Index: 5}}
	for _, count := range counts {
		if !count.Equal(count.Clone()) {
			t.Fatal("Count Clone/PartialEq differs")
		}
	}
	argument := Argument{Position: Position{Kind: ArgumentNamed, Name: "name"}, PositionSpan: Range{1, 5}}
	if !argument.IsIdentifier() {
		t.Fatal("default named argument is not an identifier")
	}
	piece := Piece{Kind: NextArgument, Argument: &argument}
	clone := piece.Clone()
	if !piece.Equal(clone) || piece.Argument == clone.Argument {
		t.Fatal("boxed argument Clone is not deep")
	}
	clone.Argument.Format.Alternate = true
	if piece.Equal(clone) || !piece.Argument.IsIdentifier() || clone.Argument.IsIdentifier() {
		t.Fatal("Clone aliases source or equality ignores format")
	}
	if !(Piece{Kind: Lit, Text: "literal"}).Equal((Piece{Kind: Lit, Text: "literal"}).Clone()) {
		t.Fatal("literal Clone differs")
	}
	// Option payloads are inactive when absent, just as Rust None has no payload.
	format := FormatSpec{Fill: Option[rune]{Value: 'x'}}
	if !format.Equal(FormatSpec{}) {
		t.Fatal("inactive None payload changed default equality")
	}
}

func TestPublicStateOverflowProfilesAndNonFusedNext(t *testing.T) {
	parser := NewParser("}literal{}", Option[uint]{}, Option[string]{}, false, Format)
	if _, ok := parser.Next(); ok || len(parser.Errors) != 1 {
		t.Fatal("unmatched close must yield None and append its error")
	}
	if next, ok := parser.Next(); !ok || next.Kind != Lit || next.Text != "literal" {
		t.Fatal("iterator incorrectly fused after error")
	}
	if next, ok := parser.Next(); !ok || next.Argument.Position.Index != 0 {
		t.Fatal("iterator failed to resume argument")
	}
	checked := NewParser("{}", Option[uint]{}, Option[string]{}, false, Format)
	checked.Curarg = ^uint(0)
	if got := oracleCatch(func() { checked.Next() }); got != "add_overflow" {
		t.Fatalf("checked native overflow=%v", got)
	}
	if checked.Curarg != ^uint(0) || !checked.Peek().Present || checked.Peek().Value.Char != '}' {
		t.Fatal("overflow mutated the wrong partial state")
	}
	wrapping := NewParserWithOverflowMode("{}", Option[uint]{}, Option[string]{}, false, Format, WrappingOverflow)
	wrapping.Curarg = ^uint(0)
	next, ok := wrapping.Next()
	if !ok || next.Argument.Position.Index != ^uint(0) || wrapping.Curarg != 0 {
		t.Fatal("wrapping native overflow profile differs")
	}
	if got := oracleCatch(func() { NewParser("", Some(uint(0)), Some(""), false, Format) }); got != "sub_overflow" {
		t.Fatalf("raw-snippet constructor overflow ordering=%v", got)
	}
}

func TestCopyEnumCloneDebugAndDefaultSemantics(t *testing.T) {
	for _, mode := range []ParseMode{Format, InlineAsm, Diagnostic} {
		if mode.Clone() != mode || mode.RustDebug() == "" {
			t.Fatal("ParseMode derived semantics differ")
		}
	}
	for _, alignment := range []Alignment{AlignUnknown, AlignLeft, AlignRight, AlignCenter} {
		if alignment.Clone() != alignment || alignment.RustDebug() == "" {
			t.Fatal("Alignment derived semantics differ")
		}
	}
	for _, sign := range []Sign{Plus, Minus} {
		if sign.Clone() != sign || sign.RustDebug() == "" {
			t.Fatal("Sign derived semantics differ")
		}
	}
	for _, hex := range []DebugHex{Lower, Upper} {
		if hex.Clone() != hex || hex.RustDebug() == "" {
			t.Fatal("DebugHex derived semantics differ")
		}
	}
	format := FormatSpec{}
	if format.Align != AlignUnknown || format.Width.Kind != CountImplied || format.Precision.Kind != CountImplied || !format.Equal(format.Clone()) {
		t.Fatal("FormatSpec Default/Clone semantics differ")
	}
	if (Count{}).Kind != CountImplied || (Alignment(0)) != AlignUnknown {
		t.Fatal("derived default variants differ")
	}
}

func FuzzParser(f *testing.F) {
	for _, input := range []string{"", "{name}", "{{{0:🦀^8.2x?}}}", "}resume{}", "{foo.bar}", "{r#type}", "{65536}", "{x:?X}", "{a\x00}\n \t{name}"} {
		f.Add(input, uint8(0), false)
	}
	f.Fuzz(func(t *testing.T, input string, mode uint8, sourceLiteral bool) {
		if !utf8.ValidString(input) || len(input) > 512 {
			t.Skip()
		}
		parser := NewParser(input, Option[uint]{}, Option[string]{}, false, ParseMode(mode%3))
		parser.IsSourceLiteral = sourceLiteral
		for range utf8.RuneCountInString(input) + 4 {
			before := parser.inputVecIndex
			piece, present := parser.Next()
			if before < uint(len(parser.inputVec)) && parser.inputVecIndex <= before {
				t.Fatal("iterator did not advance over nonempty input")
			}
			if parser.inputVecIndex > uint(len(parser.inputVec)) {
				t.Fatal("iterator advanced beyond its input")
			}
			if present {
				clone := piece.Clone()
				if !piece.Equal(clone) {
					t.Fatal("derived clone equality differs")
				}
				if piece.RustDebug() == "" {
					t.Fatal("empty derived Debug representation")
				}
				if piece.Kind == Lit && !strings.Contains(input, piece.Text) {
					t.Fatal("literal did not borrow a substring of input")
				}
			}
		}
	})
}

func TestInvalidUTF8Precondition(t *testing.T) {
	for _, test := range []struct {
		name string
		call func()
	}{
		{"input", func() { NewParser("\xff", Option[uint]{}, Option[string]{}, false, Format) }},
		{"snippet", func() { NewParser("input", Option[uint]{}, Some("\xc0\xaf"), false, Format) }},
		{"debug string", func() { debugString("\xed\xa0\x80") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				value := recover()
				if value == nil || !strings.Contains(fmt.Sprint(value), "UTF-8") {
					t.Fatalf("invalid UTF-8 did not fail explicitly: %v", value)
				}
			}()
			test.call()
		})
	}
}
