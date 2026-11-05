// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: rustc_ast/src/util/unicode.rs, blob 6f57d66b2273a2cf4421d42f8057c4290dfc653e.
package astutil

import "strings"

var textFlowControlChars = [9]rune{0x202a, 0x202b, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x202c, 0x2069}

// TextFlowControlCharsBorrowed is the shared immutable source constant. Callers
// must not mutate this borrow. TextFlowControlChars is a separate copy convenience.
func TextFlowControlCharsBorrowed() *[9]rune { return &textFlowControlChars }
func TextFlowControlChars() []rune {
	result := make([]rune, len(textFlowControlChars))
	copy(result, textFlowControlChars[:])
	return result
}

// ContainsTextFlowControlChars borrows a valid UTF-8 Rust &str. It retains the
// original memchr search and three-byte skip: it does not apply a different
// Unicode classification or allocate a converted byte buffer.
func ContainsTextFlowControlChars(text string) bool {
	for {
		index := strings.IndexByte(text, 0xe2)
		if index < 0 {
			return false
		}
		char := text[index : index+3] // Valid UTF-8 guarantees these bytes exist.
		if char[1] == 0x80 && 0xaa <= char[2] && char[2] <= 0xae || char[1] == 0x81 && 0xa6 <= char[2] && char[2] <= 0xa9 {
			return true
		}
		text = text[index+3:]
	}
}
