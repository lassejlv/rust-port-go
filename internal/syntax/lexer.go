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
	l := &lexer{src: src, line: 1, column: 1}
	var out []token
	for {
		if err := l.space(); err != nil {
			return nil, err
		}
		start := l.pos()
		if l.offset == len(src) {
			return append(out, token{kind: "eof", span: start}), nil
		}
		c := src[l.offset]
		if identStart(c) {
			begin := l.offset
			for l.offset < len(src) && identContinue(src[l.offset]) {
				l.advance()
			}
			out = append(out, token{kind: "ident", text: src[begin:l.offset], span: start})
			continue
		}
		if c >= '0' && c <= '9' {
			begin := l.offset
			for l.offset < len(src) && identContinue(src[l.offset]) {
				l.advance()
			}
			spelling := src[begin:l.offset]
			if err := validateInteger(spelling); err != nil {
				return nil, sourceError(start, "%v", err)
			}
			out = append(out, token{kind: "int", text: spelling, span: start})
			continue
		}
		if c == '"' {
			value, err := l.stringLiteral()
			if err != nil {
				return nil, err
			}
			out = append(out, token{kind: "string", text: value, span: start})
			continue
		}
		matched := false
		for _, op := range []string{"->", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "%="} {
			if strings.HasPrefix(src[l.offset:], op) {
				l.advance()
				l.advance()
				out = append(out, token{kind: op, text: op, span: start})
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if strings.ContainsRune("(){};,:+-*/%=<>!&", rune(c)) {
			l.advance()
			out = append(out, token{kind: string(c), text: string(c), span: start})
			continue
		}
		return nil, sourceError(start, "unsupported character %q", c)
	}
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
func (l *lexer) space() error {
	for l.offset < len(l.src) {
		switch l.src[l.offset] {
		case ' ', '\t', '\r', '\n':
			l.advance()
			continue
		}
		if strings.HasPrefix(l.src[l.offset:], "//") {
			for l.offset < len(l.src) && l.src[l.offset] != '\n' {
				l.advance()
			}
			continue
		}
		if strings.HasPrefix(l.src[l.offset:], "/*") {
			start := l.pos()
			l.advance()
			l.advance()
			depth := 1
			for depth > 0 {
				if l.offset == len(l.src) {
					return sourceError(start, "unterminated block comment")
				}
				if strings.HasPrefix(l.src[l.offset:], "/*") {
					depth++
					l.advance()
					l.advance()
				} else if strings.HasPrefix(l.src[l.offset:], "*/") {
					depth--
					l.advance()
					l.advance()
				} else {
					l.advance()
				}
			}
			continue
		}
		break
	}
	return nil
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

func identStart(c byte) bool    { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func identContinue(c byte) bool { return identStart(c) || c >= '0' && c <= '9' }

// Numeric range checking is deferred until type inference, allowing -2147483648.
func validateInteger(text string) error {
	digits := text
	if strings.HasSuffix(digits, "i32") {
		digits = strings.TrimSuffix(digits, "i32")
	} else if strings.HasSuffix(digits, "i64") {
		digits = strings.TrimSuffix(digits, "i64")
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
			return fmt.Errorf("invalid or unsupported integer literal %q (supported suffixes: i32, i64)", text)
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("integer literal %q has no digits", text)
	}
	return nil
}

func sourceError(span Span, format string, args ...any) error {
	return fmt.Errorf("%d:%d: %s", span.Line, span.Column, fmt.Sprintf(format, args...))
}
