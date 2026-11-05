// SPDX-License-Identifier: MIT OR Apache-2.0
// Symbol's pinned-source Unicode18 dependencies from library/alloc/src/str.rs
// and library/core/src/char/methods.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. These helpers do not replace the
// neutral formatter's separately recorded compiler1.99/Unicode17 profile.
package rustcspan

import (
	"strings"
	"unicode/utf8"

	unicodecore "github.com/lassejlv/rust-port-go/port/rust/library/core/src/unicode"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func symbolIsCased(c rune) bool {
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	if c <= 0xa9 {
		return false
	}
	return unicodecore.LowercaseLookup(c) || unicodecore.UppercaseLookup(c) || unicodecore.LTLookup(c)
}
func symbolIsCaseIgnorable(c rune) bool {
	if c < 0x80 {
		return c == '\'' || c == '.' || c == ':' || c == '^' || c == '`'
	}
	return unicodecore.CaseIgnorableLookup(c)
}

// symbolMapUppercaseSigma scans the original string in both directions,
// skipping only Case_Ignorable characters before testing Cased.
func symbolMapUppercaseSigma(from string, index int) rune {
	before := from[:index]
	precededByCased := false
	for len(before) != 0 {
		c, size := utf8.DecodeLastRuneInString(before)
		before = before[:len(before)-size]
		if !symbolIsCaseIgnorable(c) {
			precededByCased = symbolIsCased(c)
			break
		}
	}
	followedByCased := false
	for _, c := range from[index+len("Σ"):] {
		if !symbolIsCaseIgnorable(c) {
			followedByCased = symbolIsCased(c)
			break
		}
	}
	if precededByCased && !followedByCased {
		return 'ς'
	}
	return 'σ'
}
func symbolLowercase(from string) string {
	var result strings.Builder
	result.Grow(len(from))
	for index, c := range from {
		if c == 'Σ' {
			result.WriteRune(symbolMapUppercaseSigma(from, index))
			continue
		}
		chars := unicodecore.ToLower(c)
		result.WriteRune(chars[0])
		if chars[1] != 0 {
			result.WriteRune(chars[1])
			if chars[2] != 0 {
				result.WriteRune(chars[2])
			}
		}
	}
	return result.String()
}
func symbolUppercase(from string) string {
	var result strings.Builder
	result.Grow(len(from))
	for _, c := range from {
		chars := unicodecore.ToUpper(c)
		result.WriteRune(chars[0])
		if chars[1] != 0 {
			result.WriteRune(chars[1])
			if chars[2] != 0 {
				result.WriteRune(chars[2])
			}
		}
	}
	return result.String()
}

// symbolDebugString18 delegates to the explicit neutral source18 profile.
func symbolDebugString18(value string, f *rustfmt.Formatter) error {
	return rustfmt.FormatDebugStringUnicode18(value, f)
}

// FormatDebugStringPinnedUnicode18 preserves the existing whole-span bridge.
func FormatDebugStringPinnedUnicode18(value string, f *rustfmt.Formatter) error {
	return rustfmt.FormatDebugStringUnicode18(value, f)
}
func FormatDebugCharPinnedUnicode18(c rune, f *rustfmt.Formatter) error {
	return rustfmt.FormatDebugCharUnicode18(c, f)
}
func EscapeDebugPinnedUnicode18(c rune, escapeSingleQuote, escapeDoubleQuote bool) string {
	return rustfmt.EscapeDebugUnicode18(c, escapeSingleQuote, escapeDoubleQuote)
}
