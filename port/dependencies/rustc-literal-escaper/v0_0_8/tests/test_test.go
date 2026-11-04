// Port of rustc-literal-escaper 0.0.8 tests/test.rs.
package literalescaper_test

import (
	"reflect"
	"strings"
	"testing"

	esc "github.com/lassejlv/rust-port-go/port/dependencies/rustc-literal-escaper/v0_0_8/src"
)

type badCase struct {
	input string
	error esc.EscapeError
}

var commonBad = []badCase{
	{"", esc.ZeroChars}, {`\`, esc.LoneSlash},
	{"\n", esc.EscapeOnlyChar}, {"\t", esc.EscapeOnlyChar}, {"'", esc.EscapeOnlyChar}, {"\r", esc.BareCarriageReturn},
	{"spam", esc.MoreThanOneChar}, {`\x0ff`, esc.MoreThanOneChar}, {`\"a`, esc.MoreThanOneChar},
	{`\na`, esc.MoreThanOneChar}, {`\ra`, esc.MoreThanOneChar}, {`\ta`, esc.MoreThanOneChar}, {`\\a`, esc.MoreThanOneChar}, {`\'a`, esc.MoreThanOneChar}, {`\0a`, esc.MoreThanOneChar},
	{`\v`, esc.InvalidEscape}, {`\💩`, esc.InvalidEscape}, {`\●`, esc.InvalidEscape},
	{`\x`, esc.TooShortHexEscape}, {`\x0`, esc.TooShortHexEscape}, {`\xf`, esc.TooShortHexEscape}, {`\xa`, esc.TooShortHexEscape},
	{`\xx`, esc.InvalidCharInHexEscape}, {`\xы`, esc.InvalidCharInHexEscape}, {`\x🦀`, esc.InvalidCharInHexEscape}, {`\xtt`, esc.InvalidCharInHexEscape},
	{`\u`, esc.NoBraceInUnicodeEscape}, {`\u[0123]`, esc.NoBraceInUnicodeEscape}, {`\u{0x}`, esc.InvalidCharInUnicodeEscape},
	{`\u{`, esc.UnclosedUnicodeEscape}, {`\u{0000`, esc.UnclosedUnicodeEscape}, {`\u{}`, esc.EmptyUnicodeEscape},
	{`\u{_0000}`, esc.LeadingUnderscoreUnicodeEscape}, {`\u{0000000}`, esc.OverlongUnicodeEscape},
}

func TestUnescapeCharBad(t *testing.T) {
	cases := append([]badCase(nil), commonBad...)
	cases = append(cases, badCase{"\\\r", esc.InvalidEscape}, badCase{`\u{0}x`, esc.MoreThanOneChar}, badCase{`\u{1F63b}}`, esc.MoreThanOneChar})
	for _, input := range []string{`\xff`, `\xFF`, `\x80`} {
		cases = append(cases, badCase{input, esc.OutOfRangeHexEscape})
	}
	for _, input := range []string{`\u{FFFFFF}`, `\u{ffffff}`, `\u{ffffff}`} {
		cases = append(cases, badCase{input, esc.OutOfRangeUnicodeEscape})
	}
	for _, input := range []string{`\u{DC00}`, `\u{DDDD}`, `\u{DFFF}`, `\u{D800}`, `\u{DAAA}`, `\u{DBFF}`} {
		cases = append(cases, badCase{input, esc.LoneSurrogateUnicodeEscape})
	}
	for _, tc := range cases {
		if _, got := esc.UnescapeChar(tc.input); got != tc.error {
			t.Errorf("UnescapeChar(%q) = %v, want %v", tc.input, got, tc.error)
		}
	}
}

type goodCase struct {
	input string
	value rune
}

var commonGood = []goodCase{
	{"a", 'a'}, {`\"`, '"'}, {`\n`, '\n'}, {`\r`, '\r'}, {`\t`, '\t'}, {`\\`, '\\'}, {`\'`, '\''}, {`\0`, 0},
	{`\x00`, 0}, {`\x5a`, 'Z'}, {`\x5A`, 'Z'}, {`\x7f`, 127},
}

func TestUnescapeCharGood(t *testing.T) {
	cases := append([]goodCase(nil), commonGood...)
	cases = append(cases, goodCase{"ы", 'ы'}, goodCase{"🦀", '🦀'}, goodCase{`\u{0}`, 0}, goodCase{`\u{000000}`, 0}, goodCase{`\u{41}`, 'A'}, goodCase{`\u{0041}`, 'A'}, goodCase{`\u{00_41}`, 'A'}, goodCase{`\u{4__1__}`, 'A'}, goodCase{`\u{1F63b}`, '😻'})
	for _, tc := range cases {
		got, err := esc.UnescapeChar(tc.input)
		if err != esc.NoError || got != tc.value {
			t.Errorf("UnescapeChar(%q) = %U/%v, want %U", tc.input, got, err, tc.value)
		}
	}
}

type runeEvent struct {
	span   esc.Range
	result esc.Result[rune]
}

func TestUnescapeStrWarn(t *testing.T) {
	cases := []struct {
		input string
		want  []runeEvent
	}{
		{"\\\n", nil}, {"\\\n ", nil},
		{"\\\n \u00a0 x", []runeEvent{{esc.Range{Start: 0, End: 5}, esc.Result[rune]{Error: esc.UnskippedWhitespaceWarning}}, {esc.Range{Start: 3, End: 5}, esc.Result[rune]{Value: '\u00a0'}}, {esc.Range{Start: 5, End: 6}, esc.Result[rune]{Value: ' '}}, {esc.Range{Start: 6, End: 7}, esc.Result[rune]{Value: 'x'}}}},
		{"\\\n  \n  x", []runeEvent{{esc.Range{Start: 0, End: 7}, esc.Result[rune]{Error: esc.MultipleSkippedLinesWarning}}, {esc.Range{Start: 7, End: 8}, esc.Result[rune]{Value: 'x'}}}},
	}
	for _, tc := range cases {
		var got []runeEvent
		esc.UnescapeStr(tc.input, func(span esc.Range, result esc.Result[rune]) { got = append(got, runeEvent{span, result}) })
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("UnescapeStr(%q) = %#v, want %#v", tc.input, got, tc.want)
		}
	}
}

var stringGood = []struct{ input, want string }{{"foo", "foo"}, {"", ""}, {" \t\n", " \t\n"}, {"hello \\\n     world", "hello world"}, {"thread's", "thread's"}}

func TestUnescapeStrGood(t *testing.T) {
	for _, tc := range stringGood {
		var got strings.Builder
		esc.UnescapeStr(tc.input, func(_ esc.Range, result esc.Result[rune]) {
			if result.Error != esc.NoError {
				t.Errorf("UnescapeStr(%q): %v", tc.input, result.Error)
			} else {
				got.WriteRune(result.Value)
			}
		})
		if got.String() != tc.want {
			t.Errorf("UnescapeStr(%q) = %q, want %q", tc.input, got.String(), tc.want)
		}
	}
}
func TestUnescapeByteBad(t *testing.T) {
	cases := append([]badCase(nil), commonBad...)
	for _, input := range []string{"ы", "🦀"} {
		cases = append(cases, badCase{input, esc.NonAsciiCharInByte})
	}
	for _, input := range []string{`\u{0}`, `\u{000000}`, `\u{41}`, `\u{0041}`, `\u{00_41}`, `\u{4__1__}`, `\u{1F63b}`, `\u{0}x`, `\u{1F63b}}`, `\u{FFFFFF}`, `\u{ffffff}`, `\u{ffffff}`, `\u{DC00}`, `\u{DDDD}`, `\u{DFFF}`, `\u{D800}`, `\u{DAAA}`, `\u{DBFF}`} {
		cases = append(cases, badCase{input, esc.UnicodeEscapeInByte})
	}
	for _, tc := range cases {
		if _, got := esc.UnescapeByte(tc.input); got != tc.error {
			t.Errorf("UnescapeByte(%q) = %v, want %v", tc.input, got, tc.error)
		}
	}
}
func TestUnescapeByteGood(t *testing.T) {
	cases := append([]goodCase(nil), commonGood...)
	cases = append(cases, goodCase{`\x80`, 128}, goodCase{`\xff`, 255}, goodCase{`\xFF`, 255})
	for _, tc := range cases {
		got, err := esc.UnescapeByte(tc.input)
		if err != esc.NoError || rune(got) != tc.value {
			t.Errorf("UnescapeByte(%q) = %d/%v, want %d", tc.input, got, err, tc.value)
		}
	}
}
func TestUnescapeByteStrGood(t *testing.T) {
	for _, tc := range stringGood {
		var got []byte
		esc.UnescapeByteStr(tc.input, func(_ esc.Range, result esc.Result[byte]) {
			if result.Error != esc.NoError {
				t.Errorf("UnescapeByteStr(%q): %v", tc.input, result.Error)
			} else {
				got = append(got, result.Value)
			}
		})
		if string(got) != tc.want {
			t.Errorf("UnescapeByteStr(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
func TestUnescapeRawStr(t *testing.T) {
	cases := []struct {
		input string
		want  []runeEvent
	}{
		{"\r", []runeEvent{{esc.Range{Start: 0, End: 1}, esc.Result[rune]{Error: esc.BareCarriageReturnInRawString}}}},
		{"\rx", []runeEvent{{esc.Range{Start: 0, End: 1}, esc.Result[rune]{Error: esc.BareCarriageReturnInRawString}}, {esc.Range{Start: 1, End: 2}, esc.Result[rune]{Value: 'x'}}}},
	}
	for _, tc := range cases {
		var got []runeEvent
		esc.CheckRawStr(tc.input, func(span esc.Range, result esc.Result[rune]) { got = append(got, runeEvent{span, result}) })
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("CheckRawStr(%q) = %#v, want %#v", tc.input, got, tc.want)
		}
	}
}
func TestUnescapeRawByteStr(t *testing.T) {
	type byteEvent struct {
		span   esc.Range
		result esc.Result[byte]
	}
	cases := []struct {
		input string
		want  []byteEvent
	}{
		{"\r", []byteEvent{{esc.Range{Start: 0, End: 1}, esc.Result[byte]{Error: esc.BareCarriageReturnInRawString}}}},
		{"🦀", []byteEvent{{esc.Range{Start: 0, End: 4}, esc.Result[byte]{Error: esc.NonAsciiCharInByte}}}},
		{"🦀a", []byteEvent{{esc.Range{Start: 0, End: 4}, esc.Result[byte]{Error: esc.NonAsciiCharInByte}}, {esc.Range{Start: 4, End: 5}, esc.Result[byte]{Value: 'a'}}}},
	}
	for _, tc := range cases {
		var got []byteEvent
		esc.CheckRawByteStr(tc.input, func(span esc.Range, result esc.Result[byte]) { got = append(got, byteEvent{span, result}) })
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("CheckRawByteStr(%q) = %#v, want %#v", tc.input, got, tc.want)
		}
	}
}
