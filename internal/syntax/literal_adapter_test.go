package syntax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	literalescaper "github.com/lassejlv/rust-port-go/port/dependencies/rustc-literal-escaper/v0_0_8/src"
)

type literalValueCase struct{ name, literal, want string }

func literalAdapterValues() []literalValueCase {
	return []literalValueCase{
		{"ASCII escapes and NUL", `"\0\n\r\t\\\"\'\x00\x7F\u{0}\u{1f980}"`, "\x00\n\r\t\\\"'\x00\x7f\x00🦀"},
		{"Unicode underscores and maximum scalar", `"\u{0_0_0_0_4_1}\u{41__________}\u{10ffff}"`, "AA\U0010ffff"},
		{"literal UTF-8", `"é東京🦀�"`, "é東京🦀�"},
		{"mixed CRLF and LF", "\"é\r\n東京\n🦀\r\nx\"", "é\n東京\n🦀\nx"},
		{"LF continuation", "\"a\\\n \tb\"", "ab"},
		{"CRLF continuation", "\"a\\\r\n \tb\"", "ab"},
		{"skipped bare CR whitespace", "\"a\\\n \r b\"", "ab"},
		{"multiple-line warning", "\"a\\\r\n \r\n\t\r\n b\"", "ab"},
		{"unskipped Unicode whitespace warning", "\"a\\\r\n \t\u2003b\"", "a\u2003b"},
		{"both continuation warnings", "\"a\\\r\n\r\n \u2028b\"", "a\u2028b"},
		{"continuation before delimiter", "\"a\\\r\n  \"", "a"},
	}
}

func TestLiteralAdapterValues(t *testing.T) {
	for _, tc := range literalAdapterValues() {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex(tc.literal + "+1")
			if err != nil {
				t.Fatal(err)
			}
			if len(tokens) != 4 || tokens[0].kind != "string" || tokens[0].text != tc.want || tokens[1].kind != "+" {
				t.Fatalf("tokens=%#v, want string %q followed by +1", tokens, tc.want)
			}
			if want := literalTestPosition(tc.literal+"+1", len(tc.literal)); tokens[1].span != want {
				t.Fatalf("following token span=%#v, want original source %#v", tokens[1].span, want)
			}
		})
	}
}

func TestLiteralAdapterCRLFByteMapping(t *testing.T) {
	for _, tc := range []struct {
		body, want string
		mapping    []int
	}{
		{"é\r\n東\r\n", "é\n東\n", []int{0, 1, 2, 4, 5, 6, 7, 9}},
		{"\r\n\r\n", "\n\n", []int{0, 2, 4}},
		{"x\r\r\ny", "x\r\ny", []int{0, 1, 2, 4, 5}},
		{"é\n東", "é\n東", []int{0, 1, 2, 3, 4, 5, 6}},
	} {
		normalized := normalizeLiteralCRLF(tc.body)
		if normalized.text != tc.want {
			t.Fatalf("normalized=%q, want %q", normalized.text, tc.want)
		}
		var mapping []int
		for offset := 0; offset <= len(normalized.text); offset++ {
			mapping = append(mapping, normalized.sourceOffset(offset))
		}
		if !reflect.DeepEqual(mapping, tc.mapping) {
			t.Fatalf("mapping %q=%v, want %v", tc.body, mapping, tc.mapping)
		}
	}
}

type literalFailureCase struct {
	name, body, message string
	consumed            int
	kind                literalescaper.EscapeError
}

func literalAdapterFailures() []literalFailureCase {
	return []literalFailureCase{
		{"unknown escape", `\q`, "unsupported string escape", 2, literalescaper.InvalidEscape},
		{"short hex", `\x`, "expected two hexadecimal digits", 2, literalescaper.TooShortHexEscape},
		{"one hex digit", `\x1`, "expected two hexadecimal digits", 3, literalescaper.TooShortHexEscape},
		{"invalid hex digit", `\xG0`, "ASCII byte", 3, literalescaper.InvalidCharInHexEscape},
		{"non-ASCII hex", `\x80`, "ASCII byte", 4, literalescaper.OutOfRangeHexEscape},
		{"missing Unicode brace", `\u!`, "expected { after", 3, literalescaper.NoBraceInUnicodeEscape},
		{"empty Unicode", `\u{}`, "invalid Unicode escape", 4, literalescaper.EmptyUnicodeEscape},
		{"leading Unicode underscore", `\u{_41}`, "first character must be a hexadecimal digit", 4, literalescaper.LeadingUnderscoreUnicodeEscape},
		{"invalid Unicode digit", `\u{+41}`, "invalid Unicode escape", 4, literalescaper.InvalidCharInUnicodeEscape},
		{"long Unicode", `\u{0000041}`, "invalid Unicode escape", 11, literalescaper.OverlongUnicodeEscape},
		{"surrogate", `\u{d800}`, "invalid Unicode escape", 8, literalescaper.LoneSurrogateUnicodeEscape},
		{"out-of-range scalar", `\u{110000}`, "invalid Unicode escape", 10, literalescaper.OutOfRangeUnicodeEscape},
		{"unclosed Unicode body", `\u{123`, "unterminated Unicode escape", 6, literalescaper.UnclosedUnicodeEscape},
		{"bare CR", "\ry", "bare CR", 1, literalescaper.BareCarriageReturn},
		{"escaped bare CR", "\\\ry", "bare CR", 2, literalescaper.InvalidEscape},
		{"CRLF inside Unicode escape", "\\u{1\r\nx}", "invalid Unicode escape", 6, literalescaper.InvalidCharInUnicodeEscape},
		{"CRLF inside hex escape", "\\x1\r\n", "ASCII byte", 5, literalescaper.InvalidCharInHexEscape},
	}
}

func literalFailureSource(tc literalFailureCase) (source string, offset int) {
	prefix := "// α\r\nfn main() { print!(\"{}\", \"é\r\n東京"
	return prefix + tc.body + "\"); }", len(prefix)
}
func literalTestPosition(source string, offset int) Span {
	prefix := source[:offset]
	lineStart := strings.LastIndexByte(prefix, '\n') + 1
	return Span{Offset: offset, Line: strings.Count(prefix, "\n") + 1, Column: utf8.RuneCountInString(prefix[lineStart:]) + 1}
}

func TestLiteralAdapterFatalRanges(t *testing.T) {
	for _, tc := range literalAdapterFailures() {
		t.Run(tc.name, func(t *testing.T) {
			source, offset := literalFailureSource(tc)
			_, err := lex(source)
			var failure *literalEscapeError
			if !errors.As(err, &failure) {
				t.Fatalf("error=%v, want mapped literal escape", err)
			}
			if failure.kind != tc.kind || failure.start != literalTestPosition(source, offset) || failure.end != literalTestPosition(source, offset+tc.consumed) {
				t.Fatalf("failure=%#v, want %s from %#v to %#v", failure, tc.kind, literalTestPosition(source, offset), literalTestPosition(source, offset+tc.consumed))
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error=%v, want %q", err, tc.message)
			}
		})
	}
	// Warning spans must not contribute a NUL or hide a subsequent fatal error.
	source := "\"é\\\r\n\r\n \u2003\\q\""
	_, err := lex(source)
	var failure *literalEscapeError
	if !errors.As(err, &failure) || failure.kind != literalescaper.InvalidEscape || failure.start != literalTestPosition(source, strings.Index(source, `\q`)) {
		t.Fatalf("failure after continuation warnings=%#v, error=%v", failure, err)
	}
}

func TestLiteralAdapterRawTokenBounds(t *testing.T) {
	for _, source := range []string{`"\u{41" } "safe"`, `"\x1"; "0"`, `"\q"; "safe"`} {
		_, err := lex(source)
		var failure *literalEscapeError
		if !errors.As(err, &failure) {
			t.Fatalf("source %q: %v", source, err)
		}
		closingQuote := strings.IndexByte(source[1:], '"') + 1
		if failure.end.Offset > closingQuote {
			t.Fatalf("escape consumed closing delimiter/following token: %#v", failure)
		}
	}
	for _, source := range []string{"\"", "\"abc", "\"abc\\", "\"abc\\\""} {
		if _, err := lex(source); err == nil || !strings.HasPrefix(err.Error(), "1:1:") || !strings.Contains(err.Error(), "unterminated string literal") {
			t.Fatalf("unterminated %q: %v", source, err)
		}
	}
	source := "\"é\r\n東京\"suffix"
	_, err := lex(source)
	position := literalTestPosition(source, strings.Index(source, "suffix"))
	if err == nil || !strings.HasPrefix(err.Error(), "2:4:") || !strings.Contains(err.Error(), "string literal suffixes") || position.Line != 2 || position.Column != 4 {
		t.Fatalf("suffix span after CRLF/UTF-8: %v", err)
	}
}

// An explicit oracle request fails instead of silently selecting another tool.
func literalAdapterRustc(t *testing.T) string {
	t.Helper()
	requested, enabled := os.LookupEnv("FERRUGO_RUSTC")
	if !enabled {
		t.Skip("set FERRUGO_RUSTC to compare literal values and spans with Rust")
	}
	if strings.TrimSpace(requested) == "" {
		t.Fatal("FERRUGO_RUSTC is explicitly empty")
	}
	path, err := exec.LookPath(requested)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func literalRustCommand(t *testing.T, executable string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	return output, err
}

func TestLiteralAdapterRustcValuesAndRanges(t *testing.T) {
	rustc := literalAdapterRustc(t)
	for _, tc := range literalAdapterValues() {
		t.Run("value/"+tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source, output := filepath.Join(dir, "main.rs"), filepath.Join(dir, "program")
			if err := os.WriteFile(source, []byte("fn main() { print!(\"{}\", "+tc.literal+"); }"), 0644); err != nil {
				t.Fatal(err)
			}
			if diagnostics, err := literalRustCommand(t, rustc, "--edition=2021", "--crate-name", "literal_adapter", source, "-o", output); err != nil {
				t.Fatalf("rustc: %v\n%s", err, diagnostics)
			}
			actual, err := literalRustCommand(t, output)
			if err != nil {
				t.Fatal(err)
			}
			tokens, err := lex(tc.literal)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, []byte(tokens[0].text)) || !bytes.Equal(actual, []byte(tc.want)) {
				t.Fatalf("Rust=%q, frontend=%q, want=%q", actual, tokens[0].text, tc.want)
			}
		})
	}
	for _, tc := range literalAdapterFailures() {
		t.Run("range/"+tc.name, func(t *testing.T) {
			source, _ := literalFailureSource(tc)
			_, err := lex(source)
			var failure *literalEscapeError
			if !errors.As(err, &failure) {
				t.Fatalf("frontend accepted invalid literal: %v", err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "main.rs")
			if err := os.WriteFile(path, []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
			diagnostics, err := literalRustCommand(t, rustc, "--edition=2021", "--crate-name", "literal_adapter", "--error-format=json", "--emit=metadata", "--out-dir", dir, path)
			if err == nil {
				t.Fatal("rustc accepted invalid literal")
			}
			matched := false
			for _, line := range bytes.Split(diagnostics, []byte{'\n'}) {
				var diagnostic struct {
					Level string `json:"level"`
					Spans []struct {
						Start     int  `json:"byte_start"`
						End       int  `json:"byte_end"`
						LineEnd   int  `json:"line_end"`
						ColumnEnd int  `json:"column_end"`
						Primary   bool `json:"is_primary"`
					} `json:"spans"`
				}
				if json.Unmarshal(line, &diagnostic) != nil || diagnostic.Level != "error" {
					continue
				}
				for _, span := range diagnostic.Spans {
					// Rust sometimes highlights just the invalid scalar; the escaper
					// reports its complete escape. Their original-source ends agree.
					if span.Primary && span.Start >= failure.start.Offset && span.End == failure.end.Offset && span.LineEnd == failure.end.Line && span.ColumnEnd == failure.end.Column {
						matched = true
					}
				}
			}
			if !matched {
				t.Fatalf("Rust diagnostic range differs from mapped escape %#v:\n%s", failure, diagnostics)
			}
		})
	}
}
