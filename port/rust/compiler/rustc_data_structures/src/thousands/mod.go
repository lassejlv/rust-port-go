// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of rust-lang/rust compiler/rustc_data_structures/src/thousands/mod.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// Source SHA-256: adcd6db15c34f2fbf4aa9804427cd2f86c8e9e63be5a07316b1ab5e6240d21fb
// License texts: ../../../../LICENSE-MIT and LICENSE-APACHE.

// Package thousands formats diagnostic numbers with underscore separators.
package thousands

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// formatWithUnderscores groups the initial ASCII digit run after an optional
// minus sign. Fractional/exponent text and other suffixes remain unchanged.
func formatWithUnderscores(text string) string {
	if !utf8.ValidString(text) {
		panic("thousands: input must be valid UTF-8")
	}
	start := 0
	if strings.HasPrefix(text, "-") {
		start = 1
	}
	end := start
	// Rust char::is_digit(10) recognizes ASCII digits only.
	for end < len(text) && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	if end-start <= 3 {
		return text
	}
	separators := (end - start - 1) / 3
	var formatted strings.Builder
	formatted.Grow(len(text) + separators)
	formatted.WriteString(text[:start])
	for index := start; index < end; index++ {
		if index != start && (end-index)%3 == 0 {
			formatted.WriteByte('_')
		}
		formatted.WriteByte(text[index])
	}
	formatted.WriteString(text[end:])
	return formatted.String()
}

// UsizeWithUnderscores prints a host-sized unsigned integer with separators.
func UsizeWithUnderscores(value uint) string {
	return formatWithUnderscores(strconv.FormatUint(uint64(value), 10))
}

// IsizeWithUnderscores prints a host-sized signed integer with separators.
func IsizeWithUnderscores(value int) string {
	return formatWithUnderscores(strconv.FormatInt(int64(value), 10))
}

// F64p1WithUnderscores prints a float64 with one decimal place and separators.
// Ties round to even. Negative zero is preserved; nonfinite spellings match
// Rust's formatting: "inf", "-inf" and "NaN" (independent of a NaN's sign).
func F64p1WithUnderscores(value float64) string {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	default:
		return formatWithUnderscores(strconv.FormatFloat(value, 'f', 1, 64))
	}
}
