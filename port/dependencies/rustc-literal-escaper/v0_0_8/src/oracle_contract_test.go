// SPDX-License-Identifier: MIT OR Apache-2.0
package literalescaper

import (
	"path/filepath"
	"runtime"
	"testing"

	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

func literalDebugValues() []validation.Value {
	var values []validation.Value
	for e := ZeroChars; e <= MultipleSkippedLinesWarning; e++ {
		e := e
		values = append(values, &e)
	}
	for m := Char; m <= RawCStr; m++ {
		m := m
		values = append(values, &m)
	}
	for _, c := range []rune{'a', '\n', '\'', '\\', '🦀', '\u0301', '\x7f', '\U0010ffff'} {
		char, ok := NewNonZeroChar(c)
		if !ok {
			panic("invalid native char case")
		}
		unit := MixedChar(char)
		values = append(values, &unit, &char)
	}
	for _, b := range []byte{1, 26, 127, 128, 255} {
		byte, ok := NewNonZeroByte(b)
		if !ok {
			panic("invalid native byte case")
		}
		unit := MixedHighByte(byte)
		values = append(values, &unit, &byte)
	}
	return values
}
func TestOriginalLiteralFullDebug(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	directory := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../../tools/port-literal-contracts"))
	validation.Replay(t, directory, literalDebugValues())
}
