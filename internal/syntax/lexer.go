package syntax

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type token struct {
	kind, text string
	span       Span
}

type lexer struct {
	src                  string
	offset, line, column int
}

func lex(src string) ([]token, error) {
	if err := validateSourceUTF8(src); err != nil {
		return nil, err
	}
	return lexRawSource(src)
}

func (l *lexer) pos() Span { return Span{Offset: l.offset, Line: l.line, Column: l.column} }
func (l *lexer) advance() {
	if l.src[l.offset] == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	_, n := utf8.DecodeRuneInString(l.src[l.offset:])
	l.offset += n
}
func (l *lexer) stringLiteral() (string, error) {
	start := l.pos()
	l.advance()
	var value strings.Builder
	for l.offset < len(l.src) {
		c := l.src[l.offset]
		if c == '"' {
			l.advance()
			return value.String(), nil
		}
		if c == '\r' {
			position := l.pos()
			l.advance()
			if l.offset == len(l.src) || l.src[l.offset] != '\n' {
				return "", sourceError(position, "bare CR is not allowed in a string; use \\r")
			}
			l.advance()
			value.WriteByte('\n')
			continue
		}
		if c != '\\' {
			r, n := utf8.DecodeRuneInString(l.src[l.offset:])
			if r == utf8.RuneError && n == 1 {
				return "", sourceError(l.pos(), "invalid UTF-8 in string")
			}
			value.WriteRune(r)
			l.advance()
			continue
		}
		escape := l.pos()
		l.advance()
		if l.offset == len(l.src) {
			break
		}
		c = l.src[l.offset]
		l.advance()
		switch c {
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		case '0':
			value.WriteByte(0)
		case '\\', '"', '\'':
			value.WriteByte(c)
		case '\r':
			if l.offset == len(l.src) || l.src[l.offset] != '\n' {
				return "", sourceError(escape, "bare CR is not a valid string escape")
			}
			l.advance()
			fallthrough
		case '\n':
			for l.offset < len(l.src) && strings.ContainsRune(" \t\r\n", rune(l.src[l.offset])) {
				l.advance()
			}
		case 'x':
			if l.offset+2 > len(l.src) {
				return "", sourceError(escape, "expected two hexadecimal digits after \\x")
			}
			n, err := strconv.ParseUint(l.src[l.offset:l.offset+2], 16, 8)
			if err != nil || n > 0x7f {
				return "", sourceError(escape, "\\x escape must be an ASCII byte")
			}
			value.WriteByte(byte(n))
			l.advance()
			l.advance()
		case 'u':
			if l.offset >= len(l.src) || l.src[l.offset] != '{' {
				return "", sourceError(escape, "expected { after \\u")
			}
			l.advance()
			begin := l.offset
			if begin < len(l.src) && !hexDigit(l.src[begin]) {
				return "", sourceError(escape, "invalid Unicode escape: first character must be a hexadecimal digit")
			}
			for l.offset < len(l.src) && l.src[l.offset] != '}' {
				l.advance()
			}
			if l.offset == len(l.src) {
				return "", sourceError(escape, "unterminated Unicode escape")
			}
			digits := strings.ReplaceAll(l.src[begin:l.offset], "_", "")
			n, err := strconv.ParseUint(digits, 16, 32)
			if err != nil || len(digits) > 6 || !utf8.ValidRune(rune(n)) {
				return "", sourceError(escape, "invalid Unicode escape")
			}
			value.WriteRune(rune(n))
			l.advance()
		default:
			return "", sourceError(escape, "unsupported string escape \\%c", c)
		}
	}
	return "", sourceError(start, "unterminated string literal")
}

// Check all source bytes before tokenization, including skipped comments. The
// temporary cursor preserves original byte offsets and counts Unicode columns.
func validateSourceUTF8(src string) error {
	if utf8.ValidString(src) {
		return nil
	}
	l := &lexer{src: src, line: 1, column: 1}
	for l.offset < len(src) {
		r, size := utf8.DecodeRuneInString(src[l.offset:])
		if r == utf8.RuneError && size == 1 {
			return sourceError(l.pos(), "invalid UTF-8 in source")
		}
		l.advance()
	}
	return nil
}

func hexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// Numeric range checking is deferred until type inference, allowing signed
// minima and giving the checker responsibility for all supported integer widths.
func validateInteger(text string) error {
	digits := text
	for _, suffix := range []string{"i8", "i16", "i32", "i64", "isize", "u8", "u16", "u32", "u64", "usize"} {
		if strings.HasSuffix(digits, suffix) {
			digits = strings.TrimSuffix(digits, suffix)
			break
		}
	}
	base := 10
	if len(digits) >= 2 {
		switch digits[:2] {
		case "0x":
			base = 16
			digits = digits[2:]
		case "0o":
			base = 8
			digits = digits[2:]
		case "0b":
			base = 2
			digits = digits[2:]
		}
	}
	count := 0
	for _, c := range digits {
		if c == '_' {
			continue
		}
		n := -1
		switch {
		case c >= '0' && c <= '9':
			n = int(c - '0')
		case c >= 'a' && c <= 'f':
			n = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			n = int(c-'A') + 10
		}
		if n < 0 || n >= base {
			return fmt.Errorf("invalid or unsupported integer literal %q (supported suffixes: i8, i16, i32, i64, isize, u8, u16, u32, u64, usize)", text)
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("integer literal %q has no digits", text)
	}
	return nil
}

func sourceError(span Span, format string, args ...any) error {
	location := fmt.Sprintf("%d:%d", span.Line, span.Column)
	if span.File != "" {
		location = span.File + ":" + location
	}
	return fmt.Errorf("%s: %s", location, fmt.Sprintf(format, args...))
}
