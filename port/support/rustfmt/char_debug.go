// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt

import (
	"strconv"
	"unicode/utf8"
)

func charUnicodeEscape(c rune) bool {
	lo, hi := 0, len(charDebugEscapedRanges)
	for lo < hi {
		mid := (lo + hi) / 2
		if charDebugEscapedRanges[mid][1] < c {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo < len(charDebugEscapedRanges) && charDebugEscapedRanges[lo][0] <= c
}

// FormatDebugChar preserves the recorded Rust 1.99.1 char Debug's distinct
// string/character writes and Unicode 17 escaping, independent of Go Unicode.
func FormatDebugChar(c rune, f *Formatter) error {
	if !utf8.ValidRune(c) {
		panic("rustfmt: char must be a Unicode scalar")
	}
	if err := f.WriteChar('\''); err != nil {
		return err
	}
	escaped := ""
	switch c {
	case 0:
		escaped = `\0`
	case '\t':
		escaped = `\t`
	case '\n':
		escaped = `\n`
	case '\r':
		escaped = `\r`
	case '\\':
		escaped = `\\`
	case '\'':
		escaped = `\'`
	default:
		if charUnicodeEscape(c) {
			escaped = `\u{` + strconv.FormatUint(uint64(c), 16) + `}`
		}
	}
	var err error
	if escaped != "" {
		err = f.WriteString(escaped)
	} else {
		err = f.WriteChar(c)
	}
	if err != nil {
		return err
	}
	return f.WriteChar('\'')
}

// FormatDebugString streams the source str Debug algorithm, including empty
// printable substring writes around escapes. Its options are intentionally not
// padded or truncated: Debug for str writes its quoted representation directly.
func FormatDebugString(value string, f *Formatter) error {
	if !utf8.ValidString(value) {
		panic("rustfmt: str must be valid UTF-8")
	}
	if err := f.WriteChar('"'); err != nil {
		return err
	}
	start, end, rest := 0, 0, 0
	for rest < len(value) {
		fast := rest
		for fast < len(value) {
			b := value[fast]
			if b > 0x7e || b < 0x20 || b == '\\' || b == '"' {
				break
			}
			fast++
		}
		end += fast - rest
		rest = fast
		if rest == len(value) {
			break
		}
		c, size := utf8.DecodeRuneInString(value[rest:])
		escaped := ""
		switch c {
		case 0:
			escaped = `\0`
		case '\t':
			escaped = `\t`
		case '\n':
			escaped = `\n`
		case '\r':
			escaped = `\r`
		case '\\':
			escaped = `\\`
		case '"':
			escaped = `\"`
		default:
			if charUnicodeEscape(c) {
				escaped = `\u{` + strconv.FormatUint(uint64(c), 16) + `}`
			}
		}
		if escaped != "" {
			if err := f.WriteString(value[start:end]); err != nil {
				return err
			}
			if err := f.WriteString(escaped); err != nil {
				return err
			}
			start = end + size
		}
		end += size
		rest += size
	}
	if err := f.WriteString(value[start:end]); err != nil {
		return err
	}
	return f.WriteChar('"')
}
