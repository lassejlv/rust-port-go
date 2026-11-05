// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped explicit integer-radix formatting used by translated trait delegates.
// This grants no Rust standard-library source-file completion credit.
package rustfmt

import (
	"strconv"
	"strings"
)

// FormatUint selects an explicit unsigned formatting trait, independently of
// Debug's x?/X? flags. It shares the real mutable Formatter so padding errors
// retain exactly the same fill/alignment state as the primitive implementation.
func FormatUint(value uint64, bits uint, radix uint, uppercase bool, f *Formatter) error {
	if bits == 0 || bits > 64 {
		f.Context.Panic("rustfmt: invalid unsigned integer width")
	}
	if bits < 64 {
		value &= uint64(1)<<bits - 1
	}
	prefix := ""
	switch radix {
	case 2:
		prefix = "0b"
	case 8:
		prefix = "0o"
	case 10:
	case 16:
		prefix = "0x"
	default:
		f.Context.Panic("rustfmt: invalid unsigned formatting radix")
	}
	digits := strconv.FormatUint(value, int(radix))
	if radix == 16 && uppercase {
		digits = strings.ToUpper(digits)
	}
	return f.padIntegral(true, prefix, digits)
}
