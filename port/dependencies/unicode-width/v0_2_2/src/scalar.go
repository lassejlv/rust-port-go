// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped UnicodeWidthChar::width adapter for unicode-width 0.2.2. This does
// not port the complete lib.rs/tables.rs files or string/CJK width APIs.
package unicodewidth

import "unicode/utf8"

// UnicodeVersion is the immutable version tuple exported by the exact crate.
func UnicodeVersion() [3]uint8 { return [3]uint8{17, 0, 0} }

// CharWidth returns the exact non-CJK UnicodeWidthChar::width result. A false
// second result represents Rust None for a control character. The argument
// must be a Unicode scalar, as required by Rust char; invalid runes panic.
func CharWidth(ch rune) (uint, bool) {
	if !utf8.ValidRune(ch) {
		panic("unicode-width: argument is not a Unicode scalar")
	}
	code := uint32(2) // The overwhelmingly common Some(1) result.
	lo, hi := 0, len(scalarRanges)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if scalarRanges[mid][1] < uint32(ch) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(scalarRanges) && scalarRanges[lo][0] <= uint32(ch) {
		code = scalarRanges[lo][2]
	}
	if code == 0 {
		return 0, false
	}
	return uint(code - 1), true
}
