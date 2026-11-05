// SPDX-License-Identifier: MIT OR Apache-2.0
// Explicit Unicode18 primitive profile from library/core/src/char/methods.rs
// and core/src/fmt/mod.rs at 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// These used-contract primitives grant no additional standard-file credit.
// The separately supported compiler1.99/Unicode17 helpers remain unchanged.
package rustfmt

import (
	unicodecore "github.com/lassejlv/rust-port-go/port/rust/library/core/src/unicode"
	"strconv"
	"unicode/utf8"
)

func unicode18IsAssigned(c rune) bool {
	switch {
	case c <= 0x377:
		return true
	case c <= 0x3fffd:
		return !unicodecore.CNPlanes03Lookup(c)
	case c == 0xe0001, c >= 0xe0020 && c <= 0xe007f, c >= 0xe0100 && c <= 0xe01ef,
		c >= 0xf0000 && c <= 0xffffd, c >= 0x100000 && c <= 0x10fffd:
		return true
	default:
		return false
	}
}
func unicode18NeedsEscape(c rune) bool {
	if c >= 0x20 && c <= 0x7e || c == 0xff9e || c == 0xff9f {
		return false
	}
	control := c <= 0x1f || c >= 0x7f && c <= 0x9f
	private := c >= 0xe000 && c <= 0xf8ff || c >= 0xf0000 && c <= 0xffffd || c >= 0x100000 && c <= 0x10fffd
	whitespace := c >= '\t' && c <= '\r' || c == ' ' || c > 0x7f && unicodecore.WhiteSpaceLookup(c)
	return control || private || whitespace || c > 0x2ff && unicodecore.GraphemeExtendLookup(c) ||
		c > 0xac && unicodecore.DefaultIgnorableCodePointLookup(c) || unicodecore.CFLookup(c) || !unicode18IsAssigned(c)
}

// formatDebugString18 streams the pinned str Debug algorithm with pinned
// escape_debug_ext character predicates. Empty printable-segment writes and
// WriteChar versus WriteString calls are part of the observable writer trace.
func formatDebugString18(value string, f *Formatter) error {
	if !utf8.ValidString(value) {
		f.Context.Panic("rustfmt: str must be valid UTF-8")
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
			if unicode18NeedsEscape(c) {
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

// FormatDebugStringUnicode18 selects the pinned source18 standard-library profile.
// It does not change FormatDebugString's separately supported profile17.
func FormatDebugStringUnicode18(value string, f *Formatter) error {
	return formatDebugString18(value, f)
}

// EscapeDebugUnicode18 exposes the original escape_debug_ext scalar
// result for wrappers such as OsStr/Wtf8 that have their own writer protocol.
// Printable scalars are returned verbatim; quote flags are independent.
func EscapeDebugUnicode18(c rune, escapeSingleQuote, escapeDoubleQuote bool) string {
	if !utf8.ValidRune(c) {
		panic("rustfmt: char must be a Unicode scalar")
	}
	switch c {
	case '"':
		if escapeDoubleQuote {
			return `\"`
		}
	case '\'':
		if escapeSingleQuote {
			return `\'`
		}
	case '\\':
		return `\\`
	case '\n':
		return `\n`
	case '\t':
		return `\t`
	case '\r':
		return `\r`
	case 0:
		return `\0`
	}
	if unicode18NeedsEscape(c) {
		return `\u{` + strconv.FormatUint(uint64(c), 16) + `}`
	}
	return string(c)
}

// FormatDebugCharUnicode18 formats one valid scalar with pinned core's
// char Debug quote/escape rules and preserves the fallible writer operations.
func FormatDebugCharUnicode18(c rune, f *Formatter) error {
	if !utf8.ValidRune(c) {
		f.Context.Panic("rustfmt: char must be a Unicode scalar")
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
		if unicode18NeedsEscape(c) {
			escaped = `\u{` + strconv.FormatUint(uint64(c), 16) + `}`
		}
	}
	if escaped != "" {
		if err := f.WriteString(escaped); err != nil {
			return err
		}
	} else if err := f.WriteChar(c); err != nil {
		return err
	}
	return f.WriteChar('\'')
}
