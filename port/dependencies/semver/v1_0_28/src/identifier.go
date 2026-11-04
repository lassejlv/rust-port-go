// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/identifier.rs. The private allocation, pointer-tag,
// varint-length and Drop machinery is represented by immutable Go strings and
// garbage collection; its byte values and equality remain exact. No Rust ABI,
// allocation timing or inline-storage layout is promised by the Go adapter.
package semver

import "unicode/utf8"

type identifier struct{ text string }

func identifierUnchecked(text string) identifier {
	// The Rust constructor is unsafe and requires ASCII without NUL. Only the
	// parser calls this constructor after checking those preconditions.
	for index := range text {
		if text[index] == 0 || text[index] > 127 {
			panic("semver: Identifier::new_unchecked requires non-NUL ASCII")
		}
	}
	return identifier{text: text}
}
func requireUTF8(text string) {
	if !utf8.ValidString(text) {
		panic("semver: input must be valid UTF-8")
	}
}
